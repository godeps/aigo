// Package pika implements engine.Engine for the Pika video generation API.
//
// Video generation is async: POST /v1/generate creates a task,
// GET /v1/generate/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env PIKA_API_KEY.
package pika

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
	defaultBaseURL      = "https://api.pika.art"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	ModelPika22 = "pika-2.2"
	ModelPika21 = "pika-2.1"
)

var (
	ErrMissingPrompt = errors.New("pika: missing prompt in workflow graph")
)

// Config configures the Pika engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Pika.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitVideo    bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a Pika engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("PIKA_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelPika22
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
		waitVideo:    cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// Execute generates a video via the Pika API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("pika: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "PIKA_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("pika: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"model":      e.model,
		"promptText": prompt,
	}

	// Reference image for image-to-video.
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["image"] = map[string]any{"url": u}
			break
		}
	}

	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		payload["duration"] = d
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspectRatio"] = ar
	}
	if res, ok := resolve.StringOption(g, "resolution"); ok && res != "" {
		payload["resolution"] = res
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("pika: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/v1/generate", apiKey, body, "pika")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("pika: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("pika: create returned empty id")
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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/v1/generate/"+taskID, nil)
		if err != nil {
			return "", false, fmt.Errorf("pika: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("pika: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("pika: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "pika")
		}

		var task struct {
			Status string `json:"status"`
			Video  struct {
				URL string `json:"url"`
			} `json:"video"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("pika: decode poll: %w", err)
		}

		switch strings.ToLower(task.Status) {
		case "completed", "finished":
			if task.Video.URL == "" {
				return "", true, fmt.Errorf("pika: completed but no video URL")
			}
			return task.Video.URL, true, nil
		case "failed", "error":
			msg := "failed"
			if task.Error != "" {
				msg = task.Error
			}
			return "", true, fmt.Errorf("pika: task failed: %s", msg)
		default:
			return "", false, nil
		}
	})
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := engine.ResolveKey(e.apiKey, "PIKA_API_KEY")
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

// ConfigSchema returns the configuration fields required by the Pika engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "PIKA_API_KEY", Description: "Pika API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "PIKA_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Pika models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			ModelPika22,
			ModelPika21,
		},
	}
}
