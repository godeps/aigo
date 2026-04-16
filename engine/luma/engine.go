// Package luma implements engine.Engine for the Luma Dream Machine API.
//
// Both video and image generation are async:
// POST /dream-machine/v1/generations creates a task,
// GET /dream-machine/v1/generations/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env LUMA_API_KEY.
package luma

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
	defaultBaseURL      = "https://api.lumalabs.ai"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	// Video models.
	ModelRay2      = "ray-2"
	ModelRayFlash2 = "ray-flash-2"
	// Image models.
	ModelPhoton1      = "photon-1"
	ModelPhotonFlash1 = "photon-flash-1"
)

var videoModels = map[string]bool{
	ModelRay2:      true,
	ModelRayFlash2: true,
}

var imageModels = map[string]bool{
	ModelPhoton1:      true,
	ModelPhotonFlash1: true,
}

var (
	ErrMissingAPIKey = errors.New("luma: missing API key (set Config.APIKey or LUMA_API_KEY)")
	ErrMissingPrompt = errors.New("luma: missing prompt in workflow graph")
)

// Config configures the Luma engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Luma.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a Luma engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("LUMA_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelRay2
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
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute generates a video or image via the Luma API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("luma: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("LUMA_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("luma: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	if imageModels[e.model] {
		return e.runImage(ctx, apiKey, prompt, g)
	}
	return e.runVideo(ctx, apiKey, prompt, g)
}

func (e *Engine) runVideo(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"prompt": prompt,
		"model":  e.model,
	}

	// Check for reference image (image-to-video).
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["keyframes"] = map[string]any{
				"frame0": map[string]any{
					"type": "image",
					"url":  u,
				},
			}
			break
		}
	}

	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}
	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		payload["duration"] = d
	}
	if loop, ok := resolve.BoolOption(g, "loop"); ok {
		payload["loop"] = loop
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("luma: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/dream-machine/v1/generations", apiKey, body, "luma")
	if err != nil {
		return engine.Result{}, err
	}

	return e.handleAsyncResult(ctx, apiKey, respBody)
}

func (e *Engine) runImage(ctx context.Context, apiKey, prompt string, g workflow.Graph) (engine.Result, error) {
	payload := map[string]any{
		"prompt": prompt,
		"model":  e.model,
	}

	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		payload["aspect_ratio"] = ar
	}

	// Check for reference image (image editing / style transfer).
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["image_ref"] = map[string]any{
				"url": u,
			}
			break
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("luma: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/dream-machine/v1/generations/image", apiKey, body, "luma")
	if err != nil {
		return engine.Result{}, err
	}

	return e.handleAsyncResult(ctx, apiKey, respBody)
}

func (e *Engine) handleAsyncResult(ctx context.Context, apiKey string, respBody []byte) (engine.Result, error) {
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("luma: decode create: %w", err)
	}
	if created.ID == "" {
		return engine.Result{}, fmt.Errorf("luma: create returned empty id")
	}

	if !e.waitResult {
		return engine.Result{Value: created.ID, Kind: engine.OutputPlainText}, nil
	}

	resultURL, err := e.poll(ctx, apiKey, created.ID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: resultURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, genID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		url := e.baseURL + "/dream-machine/v1/generations/" + genID
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("luma: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("luma: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("luma: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "luma")
		}

		var gen struct {
			State  string `json:"state"`
			Assets struct {
				Video string `json:"video"`
				Image string `json:"image"`
			} `json:"assets"`
			FailureReason string `json:"failure_reason"`
		}
		if err := json.Unmarshal(body, &gen); err != nil {
			return "", false, fmt.Errorf("luma: decode poll: %w", err)
		}

		switch gen.State {
		case "completed":
			url := gen.Assets.Video
			if url == "" {
				url = gen.Assets.Image
			}
			if url == "" {
				return "", true, fmt.Errorf("luma: completed but no asset URL")
			}
			return url, true, nil
		case "failed":
			msg := "failed"
			if gen.FailureReason != "" {
				msg = gen.FailureReason
			}
			return "", true, fmt.Errorf("luma: generation failed: %s", msg)
		default:
			// queued, dreaming
			return "", false, nil
		}
	})
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("LUMA_API_KEY")
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
	cap := engine.Capability{
		Models:       []string{e.model},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
	if imageModels[e.model] {
		cap.MediaTypes = []string{"image"}
	} else {
		cap.MediaTypes = []string{"video"}
		cap.MaxDuration = 10
	}
	return cap
}

// ConfigSchema returns the configuration fields required by the Luma engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "LUMA_API_KEY", Description: "Luma AI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "LUMA_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Luma models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			ModelRay2,
			ModelRayFlash2,
		},
		"image": {
			ModelPhoton1,
			ModelPhotonFlash1,
		},
	}
}
