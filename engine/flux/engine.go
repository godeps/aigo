// Package flux implements engine.Engine for Black Forest Labs FLUX API.
//
// Image generation is async: POST /v1/{model} creates a task,
// then GET /v1/get_result?id={id} polls for completion.
// Auth: X-Key: {key}, env BFL_API_KEY.
package flux

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
	defaultBaseURL      = "https://api.bfl.ml"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	ModelProUltra = "flux-pro-1.1-ultra"
	ModelPro11    = "flux-pro-1.1"
	ModelPro      = "flux-pro"
	ModelDev      = "flux-dev"
)

var (
	ErrMissingAPIKey = errors.New("flux: missing API key (set Config.APIKey or BFL_API_KEY)")
	ErrMissingPrompt = errors.New("flux: missing prompt in workflow graph")
)

// Config configures the FLUX engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for FLUX.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitImage    bool
	pollInterval time.Duration
}

// New creates a FLUX engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("BFL_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelPro11
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        model,
		httpClient:   hc,
		waitImage:    cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute generates an image via the FLUX API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("flux: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("BFL_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("flux: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"prompt": prompt,
	}
	if w, ok := resolve.IntOption(g, "width"); ok && w > 0 {
		payload["width"] = w
	}
	if h, ok := resolve.IntOption(g, "height"); ok && h > 0 {
		payload["height"] = h
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		payload["seed"] = seed
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("flux: marshal request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+"/v1/"+e.model, apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("flux: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("flux: create returned empty id")
	}

	if !e.waitImage {
		return engine.Result{Value: created.ID, Kind: engine.OutputPlainText}, nil
	}

	imageURL, err := e.poll(ctx, apiKey, created.ID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: imageURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		url := fmt.Sprintf("%s/v1/get_result?id=%s", e.baseURL, taskID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("flux: build poll: %w", err)
		}
		req.Header.Set("X-Key", apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("flux: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("flux: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "flux")
		}

		var result struct {
			Status string `json:"status"`
			Result struct {
				Sample string `json:"sample"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", false, fmt.Errorf("flux: decode poll: %w", err)
		}

		switch result.Status {
		case "Ready":
			if result.Result.Sample == "" {
				return "", true, fmt.Errorf("flux: ready but no sample URL")
			}
			return result.Result.Sample, true, nil
		case "Error":
			return "", true, fmt.Errorf("flux: task failed")
		default:
			// Pending, Queued, etc.
			return "", false, nil
		}
	})
}

// doRequest uses X-Key header (not Bearer); cannot use httpx.DoJSON.
func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("flux: build request: %w", err)
	}
	req.Header.Set("X-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flux: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("flux: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "flux")
	}
	return out, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("BFL_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}
	url, err := e.poll(ctx, apiKey, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsPoll: e.waitImage,
		SupportsSync: !e.waitImage,
	}
}

// ConfigSchema returns the configuration fields required by the FLUX engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "BFL_API_KEY", Description: "Black Forest Labs API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "BFL_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known FLUX models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelProUltra,
			ModelPro11,
			ModelPro,
			ModelDev,
		},
	}
}
