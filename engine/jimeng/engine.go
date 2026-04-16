// Package jimeng implements engine.Engine for the Jimeng (即梦) API by ByteDance.
//
// Image generation is synchronous: POST /v1/images/generations returns URLs directly.
// Video generation is async: POST /v1/video/generations creates a task,
// then GET /v1/video/generations/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env JIMENG_API_KEY.
package jimeng

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
	defaultBaseURL      = "https://jimeng.jianying.com"
	defaultModel        = "jimeng-2.1"
	defaultPollInterval = 5 * time.Second

	imagesPath = "/v1/images/generations"
	videoPath  = "/v1/video/generations"
)

// Model constants.
const (
	ModelJimeng21    = "jimeng-2.1"
	ModelJimeng20Pro = "jimeng-2.0-pro"
)

var ErrMissingPrompt = fmt.Errorf("jimeng: missing prompt in workflow graph")

// Config configures the Jimeng engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	Endpoint          string // override path, e.g. "/v1/images/generations"
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Jimeng.
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

// New creates a Jimeng engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("JIMENG_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        model,
		endpoint:     strings.TrimSpace(cfg.Endpoint),
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// resolveAPIKey returns the configured API key, falling back to JIMENG_API_KEY.
func (e *Engine) resolveAPIKey() (string, error) {
	return engine.ResolveKey(e.apiKey, "JIMENG_API_KEY")
}

// isVideoModel returns true if the current model targets video generation.
func (e *Engine) isVideoModel() bool {
	// Explicit endpoint override takes priority.
	if e.endpoint != "" {
		return strings.Contains(e.endpoint, "video")
	}
	return false
}

// Execute generates an image or video via the Jimeng API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("jimeng: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("jimeng: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	// Route to video generation if endpoint indicates video.
	if e.isVideoModel() {
		return e.executeVideo(ctx, apiKey, prompt, g)
	}
	return e.executeImage(ctx, apiKey, prompt, g)
}

// executeImage runs synchronous image generation.
func (e *Engine) executeImage(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
	}

	// Response format: "url" (default) or "b64_json".
	respFmt := "url"
	if v, ok := resolve.StringOption(g, "response_format"); ok {
		respFmt = v
	}
	payload["response_format"] = respFmt

	if v, ok := resolve.StringOption(g, "size"); ok {
		payload["size"] = v
	}
	if v, ok := resolve.IntOption(g, "seed"); ok {
		payload["seed"] = v
	}
	if v, ok := resolve.StringOption(g, "negative_prompt"); ok {
		payload["negative_prompt"] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("jimeng: marshal request: %w", err)
	}

	ep := e.endpoint
	if ep == "" {
		ep = imagesPath
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+ep, apiKey, body, "jimeng")
	if err != nil {
		return engine.Result{}, err
	}

	url, err := extractImageResult(respBody, respFmt)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.ClassifyOutput(url)}, nil
}

// extractImageResult parses the OpenAI-compatible /images/generations response.
func extractImageResult(body []byte, format string) (string, error) {
	var resp struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("jimeng: decode image response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("jimeng: image api error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("jimeng: image response had no data")
	}

	img := resp.Data[0]
	if strings.EqualFold(format, "b64_json") && img.B64JSON != "" {
		return "data:image/png;base64," + img.B64JSON, nil
	}
	if img.URL != "" {
		return img.URL, nil
	}
	if img.B64JSON != "" {
		return "data:image/png;base64," + img.B64JSON, nil
	}
	return "", fmt.Errorf("jimeng: image response had no url or b64_json")
}

// executeVideo runs async video generation with task submission and polling.
func (e *Engine) executeVideo(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
	}

	// Check for reference image (image-to-video).
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.StringInput("url"); ok && u != "" {
			payload["image_url"] = u
			break
		}
	}

	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		payload["duration"] = d
	}
	if ar, ok := resolve.StringOption(g, "ratio", "aspect_ratio"); ok && ar != "" {
		payload["ratio"] = ar
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("jimeng: marshal request: %w", err)
	}

	ep := e.endpoint
	if ep == "" {
		ep = videoPath
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+ep, apiKey, body, "jimeng")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("jimeng: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("jimeng: create returned empty id")
	}

	if !e.waitResult {
		return engine.Result{Value: created.ID, Kind: engine.OutputPlainText}, nil
	}

	videoURL, err := e.poll(ctx, apiKey, created.ID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: videoURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	ep := e.endpoint
	if ep == "" {
		ep = videoPath
	}
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+ep+"/"+taskID, nil)
		if err != nil {
			return "", false, fmt.Errorf("jimeng: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("jimeng: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("jimeng: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "jimeng")
		}

		var task struct {
			Status string `json:"status"`
			Output struct {
				VideoURL string `json:"video_url"`
			} `json:"output"`
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("jimeng: decode poll: %w", err)
		}

		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "succeeded":
			if strings.TrimSpace(task.Output.VideoURL) == "" {
				return "", true, fmt.Errorf("jimeng: succeeded but no video_url")
			}
			return task.Output.VideoURL, true, nil
		case "failed":
			msg := "failed"
			if task.Error != nil && task.Error.Message != "" {
				msg = task.Error.Message
			}
			return "", true, fmt.Errorf("jimeng: task failed: %s", msg)
		case "cancelled":
			return "", true, fmt.Errorf("jimeng: task cancelled")
		default:
			return "", false, nil
		}
	})
}

// Resume implements engine.Resumer -- resumes polling a previously submitted video task.
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
	if e.isVideoModel() {
		return engine.Capability{
			MediaTypes:   []string{"video"},
			Models:       []string{e.model},
			MaxDuration:  10,
			SupportsPoll: e.waitResult,
			SupportsSync: !e.waitResult,
		}
	}
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the Jimeng engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "JIMENG_API_KEY", Description: "Jimeng API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "JIMENG_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Jimeng models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelJimeng21,
			ModelJimeng20Pro,
		},
	}
}
