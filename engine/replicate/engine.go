// Package replicate implements engine.Engine for the Replicate API.
//
// Replicate is a multi-model platform. Predictions are async:
// POST /v1/predictions creates a prediction,
// GET /v1/predictions/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env REPLICATE_API_TOKEN.
package replicate

import (
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
	defaultBaseURL      = "https://api.replicate.com"
	defaultPollInterval = 5 * time.Second
)

var (
	ErrMissingAPIKey = errors.New("replicate: missing API key (set Config.APIKey or REPLICATE_API_TOKEN)")
	ErrMissingModel  = errors.New("replicate: missing model version")
	ErrMissingPrompt = errors.New("replicate: missing prompt in workflow graph")
)

// Config configures the Replicate engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string // Full model version, e.g. "stability-ai/sdxl:abc123..."
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Replicate.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a Replicate engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("REPLICATE_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        strings.TrimSpace(cfg.Model),
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute runs a prediction on the Replicate API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("replicate: validate graph: %w", err)
	}
	if e.model == "" {
		return engine.Result{}, ErrMissingModel
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("REPLICATE_API_TOKEN")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("replicate: %w", err)
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
		input["width"] = w
	}
	if h, ok := resolve.IntOption(g, "height"); ok && h > 0 {
		input["height"] = h
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		input["seed"] = seed
	}

	// Reference image.
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			input["image"] = u
			break
		}
	}

	payload := map[string]any{
		"version": e.model,
		"input":   input,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("replicate: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/v1/predictions", apiKey, body, "replicate")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Output any    `json:"output"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("replicate: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("replicate: create returned empty id")
	}

	// Some predictions complete synchronously.
	if created.Status == "succeeded" {
		return extractOutput(created.Output)
	}

	if !e.waitResult {
		return engine.Result{Value: created.ID, Kind: engine.OutputPlainText}, nil
	}

	return e.poll(ctx, apiKey, created.ID)
}

func (e *Engine) poll(ctx context.Context, apiKey, predID string) (engine.Result, error) {
	val, err := epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		url := e.baseURL + "/v1/predictions/" + predID
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("replicate: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("replicate: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("replicate: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "replicate")
		}

		var pred struct {
			Status string `json:"status"`
			Output any    `json:"output"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(body, &pred); err != nil {
			return "", false, fmt.Errorf("replicate: decode poll: %w", err)
		}

		switch pred.Status {
		case "succeeded":
			result, err := extractOutput(pred.Output)
			if err != nil {
				return "", true, err
			}
			return result.Value, true, nil
		case "failed":
			msg := "failed"
			if pred.Error != "" {
				msg = pred.Error
			}
			return "", true, fmt.Errorf("replicate: prediction failed: %s", msg)
		case "canceled":
			return "", true, fmt.Errorf("replicate: prediction canceled")
		default:
			return "", false, nil
		}
	})
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: val, Kind: engine.ClassifyOutput(val)}, nil
}

// extractOutput gets the first URL or string from the Replicate output.
func extractOutput(output any) (engine.Result, error) {
	switch v := output.(type) {
	case string:
		return engine.Result{Value: v, Kind: engine.ClassifyOutput(v)}, nil
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return engine.Result{Value: s, Kind: engine.ClassifyOutput(s)}, nil
			}
		}
	}
	// Fallback: marshal as JSON.
	b, err := json.Marshal(output)
	if err != nil {
		return engine.Result{}, fmt.Errorf("replicate: marshal output: %w", err)
	}
	return engine.Result{Value: string(b), Kind: engine.OutputJSON}, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("REPLICATE_API_TOKEN")
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

// ConfigSchema returns the configuration fields required by the Replicate engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Token", Type: "secret", Required: true, EnvVar: "REPLICATE_API_TOKEN", Description: "Replicate API token"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "REPLICATE_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns a placeholder — Replicate supports many models
// but the user specifies the exact version at configuration time.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {"stability-ai/sdxl", "black-forest-labs/flux-schnell"},
		"video": {"minimax/video-01"},
	}
}
