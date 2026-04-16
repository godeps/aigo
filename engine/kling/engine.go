// Package kling implements engine.Engine for the Kling (快手可灵) API.
//
// Video generation is async: POST /v1/videos/text2video or /v1/videos/image2video
// creates a task, then GET /v1/videos/{task_id} polls for completion.
// Image generation: POST /v1/images/generations, GET /v1/images/{task_id}.
// Auth: Authorization: Bearer {key}, env KLING_API_KEY.
package kling

import (
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
	defaultBaseURL      = "https://api.klingai.com"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	ModelKlingV2       = "kling-v2"
	ModelKlingV2Master = "kling-v2-master"
	ModelKlingV1       = "kling-v1"
)

// Endpoint selects the generation mode.
const (
	EndpointText2Video  = "text2video"
	EndpointImage2Video = "image2video"
	EndpointImage       = "image"
)

var videoModels = map[string]bool{
	ModelKlingV2:       true,
	ModelKlingV2Master: true,
	ModelKlingV1:       true,
}

var ErrMissingPrompt = fmt.Errorf("kling: missing prompt in workflow graph")

// Config configures the Kling engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	Endpoint          string // "text2video", "image2video", or "image"
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Kling.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	endpoint     string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a Kling engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("KLING_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelKlingV2
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = EndpointText2Video
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        model,
		endpoint:     endpoint,
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// resolveAPIKey returns the configured API key, falling back to the
// KLING_API_KEY environment variable.
func (e *Engine) resolveAPIKey() (string, error) {
	return engine.ResolveKey(e.apiKey, "KLING_API_KEY")
}

// Execute generates a video or image via the Kling API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("kling: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("kling: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	if e.endpoint == EndpointImage {
		return e.runImage(ctx, apiKey, prompt, g)
	}
	return e.runVideo(ctx, apiKey, prompt, g)
}

func (e *Engine) runVideo(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"prompt":     prompt,
		"model_name": e.model,
	}

	// Detect image reference for image-to-video.
	imageURL := ""
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			imageURL = u
			break
		}
	}

	endpoint := "/v1/videos/text2video"
	if imageURL != "" || e.endpoint == EndpointImage2Video {
		endpoint = "/v1/videos/image2video"
		if imageURL != "" {
			payload["image"] = imageURL
		}
	}

	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		payload["duration"] = d
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}
	if neg, ok := resolve.StringOption(g, "negative_prompt"); ok && neg != "" {
		payload["negative_prompt"] = neg
	}
	if cfg, ok := resolve.Float64Option(g, "cfg_scale"); ok && cfg > 0 {
		payload["cfg_scale"] = cfg
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("kling: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+endpoint, apiKey, body, "kling")
	if err != nil {
		return engine.Result{}, err
	}

	return e.handleAsyncResult(ctx, apiKey, respBody, "video")
}

func (e *Engine) runImage(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"prompt":     prompt,
		"model_name": e.model,
	}

	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}
	if neg, ok := resolve.StringOption(g, "negative_prompt"); ok && neg != "" {
		payload["negative_prompt"] = neg
	}
	if n, ok := resolve.IntOption(g, "n"); ok && n > 0 {
		payload["n"] = n
	}

	// Reference image for image variation / editing.
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["image"] = u
			break
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("kling: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/v1/images/generations", apiKey, body, "kling")
	if err != nil {
		return engine.Result{}, err
	}

	return e.handleAsyncResult(ctx, apiKey, respBody, "image")
}

func (e *Engine) handleAsyncResult(ctx context.Context, apiKey string, respBody []byte, mediaType string) (engine.Result, error) {
	var created struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("kling: decode create: %w", err)
	}
	if created.Data.TaskID == "" {
		return engine.Result{}, fmt.Errorf("kling: create returned empty task_id")
	}

	if !e.waitResult {
		return engine.Result{Value: created.Data.TaskID, Kind: engine.OutputPlainText}, nil
	}

	resultURL, err := e.poll(ctx, apiKey, created.Data.TaskID, mediaType)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: resultURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID, mediaType string) (string, error) {
	pollPath := "/v1/videos/" + taskID
	if mediaType == "image" {
		pollPath = "/v1/images/" + taskID
	}

	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+pollPath, nil)
		if err != nil {
			return "", false, fmt.Errorf("kling: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("kling: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("kling: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "kling")
		}

		var task struct {
			Data struct {
				TaskStatus string `json:"task_status"`
				TaskResult struct {
					Videos []struct {
						URL string `json:"url"`
					} `json:"videos"`
					Images []struct {
						URL string `json:"url"`
					} `json:"images"`
				} `json:"task_result"`
			} `json:"data"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("kling: decode poll: %w", err)
		}

		switch strings.ToLower(task.Data.TaskStatus) {
		case "completed", "succeed":
			// Try videos first, then images.
			if len(task.Data.TaskResult.Videos) > 0 && task.Data.TaskResult.Videos[0].URL != "" {
				return task.Data.TaskResult.Videos[0].URL, true, nil
			}
			if len(task.Data.TaskResult.Images) > 0 && task.Data.TaskResult.Images[0].URL != "" {
				return task.Data.TaskResult.Images[0].URL, true, nil
			}
			return "", true, fmt.Errorf("kling: completed but no output URL")
		case "failed":
			msg := "failed"
			if task.Message != "" {
				msg = task.Message
			}
			return "", true, fmt.Errorf("kling: task failed: %s", msg)
		default:
			// processing, queued, submitted
			return "", false, nil
		}
	})
}

// Resume implements engine.Resumer -- resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}
	// Default to video polling; image tasks also return a URL in the videos array
	// or can be detected by the response structure.
	url, err := e.poll(ctx, apiKey, remoteID, "video")
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	cap := engine.Capability{
		Models:       []string{e.model},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
	if e.endpoint == EndpointImage {
		cap.MediaTypes = []string{"image"}
	} else {
		cap.MediaTypes = []string{"video"}
		cap.MaxDuration = 10
	}
	return cap
}

// ConfigSchema returns the configuration fields required by the Kling engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "KLING_API_KEY", Description: "Kling API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "KLING_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Kling models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			ModelKlingV2,
			ModelKlingV2Master,
			ModelKlingV1,
		},
		"image": {
			ModelKlingV2,
			ModelKlingV1,
		},
	}
}
