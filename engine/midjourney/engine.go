// Package midjourney implements engine.Engine for MidJourney image generation
// via third-party proxy APIs (e.g. GoAPI at api.goapi.ai).
//
// Image generation is async: POST /mj/v2/imagine creates a task,
// then POST /mj/v2/fetch polls for completion.
// Auth: X-API-Key: {key}, env MIDJOURNEY_API_KEY.
package midjourney

import (
	"bytes"
	"context"
	"encoding/json"
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
	defaultBaseURL      = "https://api.goapi.ai"
	defaultProcessMode  = "fast"
	defaultPollInterval = 5 * time.Second
)

var ErrMissingPrompt = fmt.Errorf("midjourney: missing prompt in workflow graph")

// Config configures the MidJourney proxy engine.
type Config struct {
	APIKey            string
	BaseURL           string
	ProcessMode       string // "fast" or "relax"
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for MidJourney via a proxy API.
type Engine struct {
	apiKey       string
	baseURL      string
	processMode  string
	httpClient   *http.Client
	waitImage    bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a MidJourney engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("MIDJOURNEY_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	mode := strings.TrimSpace(cfg.ProcessMode)
	if mode == "" {
		mode = defaultProcessMode
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		processMode:  mode,
		httpClient:   hc,
		waitImage:    cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// resolveAPIKey returns the configured API key, falling back to the environment.
func (e *Engine) resolveAPIKey() (string, error) {
	return engine.ResolveKey(e.apiKey, "GOAPI_KEY")
}

// Execute generates an image via the MidJourney proxy API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("midjourney: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("midjourney: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"prompt":       prompt,
		"process_mode": e.processMode,
	}

	if ar, ok := resolve.StringOption(g, "aspect_ratio", "ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("midjourney: marshal request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+"/mj/v2/imagine", apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("midjourney: decode create: %w", err)
	}
	if created.TaskID == "" {
		return engine.Result{}, fmt.Errorf("midjourney: create returned empty task_id")
	}

	if !e.waitImage {
		return engine.Result{Value: created.TaskID, Kind: engine.OutputPlainText}, nil
	}

	imageURL, err := e.poll(ctx, apiKey, created.TaskID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: imageURL, Kind: engine.OutputURL}, nil
}

// poll checks the task status until it finishes or fails.
func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		fetchPayload, err := json.Marshal(map[string]string{"task_id": taskID})
		if err != nil {
			return "", false, fmt.Errorf("midjourney: marshal fetch: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/mj/v2/fetch", bytes.NewReader(fetchPayload))
		if err != nil {
			return "", false, fmt.Errorf("midjourney: build poll: %w", err)
		}
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("midjourney: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("midjourney: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "midjourney")
		}

		var task struct {
			Status     string `json:"status"`
			TaskResult struct {
				ImageURL string `json:"image_url"`
			} `json:"task_result"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("midjourney: decode poll: %w", err)
		}

		switch strings.ToLower(task.Status) {
		case "finished":
			if task.TaskResult.ImageURL == "" {
				return "", true, fmt.Errorf("midjourney: finished but no image_url")
			}
			return task.TaskResult.ImageURL, true, nil
		case "failed":
			return "", true, fmt.Errorf("midjourney: task failed")
		default: // "processing", "pending", etc.
			return "", false, nil
		}
	})
}

// doRequest sends an authenticated JSON request using the X-API-Key header.
func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("midjourney: build request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("midjourney: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("midjourney: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "midjourney")
	}
	return out, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
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
		SupportsPoll: e.waitImage,
		SupportsSync: !e.waitImage,
	}
}

// ConfigSchema returns the configuration fields required by the MidJourney engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "MIDJOURNEY_API_KEY", Description: "MidJourney proxy API key (e.g. GoAPI)"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "MIDJOURNEY_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "processMode", Label: "Process Mode", Type: "string", Description: "Generation speed mode: fast or relax", Default: defaultProcessMode},
	}
}

// ModelsByCapability returns all known MidJourney models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {"midjourney"},
	}
}
