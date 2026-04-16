// Package runway implements engine.Engine for the Runway API.
//
// Video generation is async: POST /v1/image_to_video or /v1/text_to_video
// creates a task, then GET /v1/tasks/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env RUNWAY_API_KEY.
// Requires header X-Runway-Version.
package runway

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
	defaultBaseURL      = "https://api.dev.runwayml.com"
	defaultAPIVersion   = "2024-11-06"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	ModelGen4Turbo  = "gen4_turbo"
	ModelGen3ATurbo = "gen3a_turbo"
)

var (
	ErrMissingPrompt = errors.New("runway: missing prompt in workflow graph")
)

// Config configures the Runway engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	APIVersion        string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Runway.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	apiVersion   string
	httpClient   *http.Client
	waitVideo    bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a Runway engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("RUNWAY_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelGen4Turbo
	}

	ver := strings.TrimSpace(cfg.APIVersion)
	if ver == "" {
		ver = defaultAPIVersion
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        model,
		apiVersion:   ver,
		httpClient:   hc,
		waitVideo:    cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// Execute generates a video via the Runway API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("runway: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "RUNWAY_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("runway: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	// Detect image reference for image-to-video.
	imageURL := ""
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			imageURL = u
			break
		}
	}

	payload := map[string]any{
		"model":      e.model,
		"promptText": prompt,
	}

	endpoint := "/v1/text_to_video"
	if imageURL != "" {
		endpoint = "/v1/image_to_video"
		payload["promptImage"] = imageURL
	}

	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		payload["duration"] = d
	}
	if ar, ok := resolve.StringOption(g, "ratio", "aspect_ratio"); ok && ar != "" {
		payload["ratio"] = ar
	}
	if wm, ok := resolve.BoolOption(g, "watermark"); ok {
		payload["watermark"] = wm
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("runway: marshal request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+endpoint, apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("runway: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("runway: create returned empty id")
	}

	if !e.waitVideo {
		return engine.Result{Value: created.ID, Kind: engine.OutputPlainText}, nil
	}

	videoURL, err := e.poll(ctx, apiKey, created.ID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: videoURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/v1/tasks/"+taskID, nil)
		if err != nil {
			return "", false, fmt.Errorf("runway: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Runway-Version", e.apiVersion)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("runway: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("runway: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "runway")
		}

		var task struct {
			Status string   `json:"status"`
			Output []string `json:"output"`
			Error  string   `json:"failure"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("runway: decode poll: %w", err)
		}

		switch strings.ToUpper(task.Status) {
		case "SUCCEEDED":
			if len(task.Output) == 0 {
				return "", true, fmt.Errorf("runway: succeeded but no output")
			}
			return task.Output[0], true, nil
		case "FAILED":
			msg := "failed"
			if task.Error != "" {
				msg = task.Error
			}
			return "", true, fmt.Errorf("runway: task failed: %s", msg)
		case "CANCELLED":
			return "", true, fmt.Errorf("runway: task cancelled")
		default:
			return "", false, nil
		}
	})
}

// doRequest adds X-Runway-Version header; cannot use httpx.DoJSON.
func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("runway: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Runway-Version", e.apiVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("runway: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("runway: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "runway")
	}
	return out, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := engine.ResolveKey(e.apiKey, "RUNWAY_API_KEY")
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
		MediaTypes:   []string{"video"},
		Models:       []string{e.model},
		MaxDuration:  10,
		SupportsPoll: e.waitVideo,
		SupportsSync: !e.waitVideo,
	}
}

// ConfigSchema returns the configuration fields required by the Runway engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "RUNWAY_API_KEY", Description: "Runway API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "RUNWAY_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Runway models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			ModelGen4Turbo,
			ModelGen3ATurbo,
		},
	}
}
