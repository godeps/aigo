// Package fal implements engine.Engine for the Fal.ai inference platform.
//
// Execution is async via the queue API:
// POST https://queue.fal.run/{model} submits a request,
// GET https://queue.fal.run/{model}/requests/{id}/status polls for completion,
// GET https://queue.fal.run/{model}/requests/{id} retrieves the result.
// Auth: Authorization: Key {key}, env FAL_KEY.
package fal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultQueueURL     = "https://queue.fal.run"
	defaultPollInterval = 3 * time.Second
)

// Model constants for popular Fal.ai models.
const (
	ModelFluxDev     = "fal-ai/flux/dev"
	ModelFluxSchnell = "fal-ai/flux/schnell"
	ModelFluxPro     = "fal-ai/flux-pro"
	ModelSDXL        = "fal-ai/fast-sdxl"
	ModelKling       = "fal-ai/kling-video/v2/master/text-to-video"
	ModelMinimax     = "fal-ai/minimax/video-01"
)

var (
	ErrMissingAPIKey = errors.New("fal: missing API key (set Config.APIKey or FAL_KEY)")
	ErrMissingModel  = errors.New("fal: missing model")
	ErrMissingPrompt = errors.New("fal: missing prompt in workflow graph")
)

// Config configures the Fal.ai engine.
type Config struct {
	APIKey            string
	QueueURL          string // e.g. "https://queue.fal.run"
	Model             string // e.g. "fal-ai/flux/dev"
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Fal.ai.
type Engine struct {
	apiKey       string
	queueURL     string
	model        string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a Fal.ai engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	queueURL := strings.TrimRight(strings.TrimSpace(cfg.QueueURL), "/")
	if queueURL == "" {
		queueURL = strings.TrimRight(strings.TrimSpace(os.Getenv("FAL_QUEUE_URL")), "/")
	}
	if queueURL == "" {
		queueURL = defaultQueueURL
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		queueURL:     queueURL,
		model:        strings.TrimSpace(cfg.Model),
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute submits a generation request to Fal.ai.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("fal: validate graph: %w", err)
	}
	if e.model == "" {
		return engine.Result{}, ErrMissingModel
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("FAL_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("fal: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	input := map[string]any{
		"prompt": prompt,
	}
	if neg, ok := resolve.StringOption(g, "negative_prompt"); ok && neg != "" {
		input["negative_prompt"] = neg
	}
	if w, ok := resolve.IntOption(g, "width"); ok && w > 0 {
		input["image_size"] = map[string]any{"width": w}
	}
	if h, ok := resolve.IntOption(g, "height"); ok && h > 0 {
		if m, ok := input["image_size"].(map[string]any); ok {
			m["height"] = h
		} else {
			input["image_size"] = map[string]any{"height": h}
		}
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		input["seed"] = seed
	}
	if steps, ok := resolve.IntOption(g, "num_inference_steps"); ok && steps > 0 {
		input["num_inference_steps"] = steps
	}

	// Reference image.
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			input["image_url"] = u
			break
		}
	}

	body, err := json.Marshal(input)
	if err != nil {
		return engine.Result{}, fmt.Errorf("fal: marshal request: %w", err)
	}

	submitURL := fmt.Sprintf("%s/%s", e.queueURL, e.model)
	respBody, err := e.doRequest(ctx, http.MethodPost, submitURL, apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var queued struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(respBody, &queued); err != nil {
		return engine.Result{}, fmt.Errorf("fal: decode submit: %w", err)
	}
	if queued.RequestID == "" {
		return engine.Result{}, fmt.Errorf("fal: submit returned empty request_id")
	}

	if !e.waitResult {
		return engine.Result{Value: queued.RequestID, Kind: engine.OutputPlainText}, nil
	}

	return e.poll(ctx, apiKey, queued.RequestID)
}

func (e *Engine) poll(ctx context.Context, apiKey, reqID string) (engine.Result, error) {
	// First poll status until complete.
	_, err := epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		statusURL := fmt.Sprintf("%s/%s/requests/%s/status", e.queueURL, e.model, reqID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			return "", false, fmt.Errorf("fal: build status: %w", err)
		}
		req.Header.Set("Authorization", "Key "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("fal: status request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("fal: read status: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "fal")
		}

		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			return "", false, fmt.Errorf("fal: decode status: %w", err)
		}

		switch strings.ToUpper(status.Status) {
		case "COMPLETED":
			return "", true, nil
		case "FAILED":
			return "", true, fmt.Errorf("fal: request failed")
		default:
			return "", false, nil
		}
	})
	if err != nil {
		return engine.Result{}, err
	}

	// Fetch the result.
	resultURL := fmt.Sprintf("%s/%s/requests/%s", e.queueURL, e.model, reqID)
	respBody, err := e.doRequest(ctx, http.MethodGet, resultURL, apiKey, nil)
	if err != nil {
		return engine.Result{}, err
	}

	return extractResult(respBody)
}

func extractResult(body []byte) (engine.Result, error) {
	var resp struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Video struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("fal: decode result: %w", err)
	}
	if len(resp.Images) > 0 && resp.Images[0].URL != "" {
		return engine.Result{Value: resp.Images[0].URL, Kind: engine.OutputURL}, nil
	}
	if resp.Video.URL != "" {
		return engine.Result{Value: resp.Video.URL, Kind: engine.OutputURL}, nil
	}
	// Fallback: return raw JSON.
	return engine.Result{Value: string(body), Kind: engine.OutputJSON}, nil
}

// doRequest uses "Key" auth prefix (not Bearer); cannot use httpx.DoJSON.
func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("fal: build request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fal: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fal: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "fal")
	}
	return out, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("FAL_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}
	return e.poll(ctx, apiKey, remoteID)
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image", "video"},
		Models:       []string{e.model},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
}

// ConfigSchema returns the configuration fields required by the Fal.ai engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "FAL_KEY", Description: "Fal.ai API key"},
		{Key: "queueUrl", Label: "Queue URL", Type: "url", EnvVar: "FAL_QUEUE_URL", Description: "Custom queue URL (optional)", Default: defaultQueueURL},
	}
}

// ModelsByCapability returns popular Fal.ai models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelFluxDev,
			ModelFluxSchnell,
			ModelFluxPro,
			ModelSDXL,
		},
		"video": {
			ModelKling,
			ModelMinimax,
		},
	}
}
