// Package meshy implements engine.Engine for the Meshy 3D model generation API.
//
// Both text-to-3D and image-to-3D are async:
// POST /openapi/v2/text-to-3d or /openapi/v2/image-to-3d creates a task,
// GET /openapi/v2/text-to-3d/{id} or /openapi/v2/image-to-3d/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env MESHY_API_KEY.
package meshy

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
	defaultBaseURL      = "https://api.meshy.ai"
	defaultPollInterval = 5 * time.Second
	defaultEndpoint     = "text-to-3d"
)

var ErrMissingPrompt = fmt.Errorf("meshy: missing prompt in workflow graph")

// Config configures the Meshy engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Endpoint          string // "text-to-3d" or "image-to-3d"
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Meshy.
type Engine struct {
	apiKey       string
	baseURL      string
	endpoint     string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a Meshy engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("MESHY_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		endpoint:     endpoint,
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// resolveAPIKey returns the configured API key, falling back to the environment.
func (e *Engine) resolveAPIKey() (string, error) {
	return engine.ResolveKey(e.apiKey, "MESHY_API_KEY")
}

// Execute generates a 3D model via the Meshy API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("meshy: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("meshy: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	endpoint := e.endpoint

	payload := map[string]any{
		"mode": "preview",
	}

	// Detect image reference for image-to-3d.
	imageURL := ""
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			imageURL = u
			break
		}
	}

	if imageURL != "" || endpoint == "image-to-3d" {
		endpoint = "image-to-3d"
		if imageURL != "" {
			payload["image_url"] = imageURL
		}
	} else {
		payload["prompt"] = prompt
	}

	if negPrompt, ok := resolve.StringOption(g, "negative_prompt"); ok && negPrompt != "" {
		payload["negative_prompt"] = negPrompt
	}
	if artStyle, ok := resolve.StringOption(g, "art_style"); ok && artStyle != "" {
		payload["art_style"] = artStyle
	}
	if mode, ok := resolve.StringOption(g, "mode"); ok && mode != "" {
		payload["mode"] = mode
	}
	if topology, ok := resolve.StringOption(g, "topology"); ok && topology != "" {
		payload["topology"] = topology
	}
	if targetCount, ok := resolve.IntOption(g, "target_polycount"); ok && targetCount > 0 {
		payload["target_polycount"] = targetCount
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("meshy: marshal request: %w", err)
	}

	url := e.baseURL + "/openapi/v2/" + endpoint
	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, url, apiKey, body, "meshy")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("meshy: decode create: %w", err)
	}
	if created.Result == "" {
		return engine.Result{}, fmt.Errorf("meshy: create returned empty task id")
	}

	if !e.waitResult {
		return engine.Result{Value: created.Result, Kind: engine.OutputPlainText}, nil
	}

	modelURL, err := e.poll(ctx, apiKey, endpoint, created.Result)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: modelURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, endpoint, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		url := e.baseURL + "/openapi/v2/" + endpoint + "/" + taskID
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("meshy: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("meshy: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("meshy: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "meshy")
		}

		var task struct {
			Status    string `json:"status"`
			ModelURLs struct {
				GLB  string `json:"glb"`
				FBX  string `json:"fbx"`
				OBJ  string `json:"obj"`
				USDZ string `json:"usdz"`
			} `json:"model_urls"`
			TaskError struct {
				Message string `json:"message"`
			} `json:"task_error"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("meshy: decode poll: %w", err)
		}

		switch strings.ToUpper(task.Status) {
		case "SUCCEEDED":
			// Prefer GLB, then FBX, OBJ, USDZ.
			url := task.ModelURLs.GLB
			if url == "" {
				url = task.ModelURLs.FBX
			}
			if url == "" {
				url = task.ModelURLs.OBJ
			}
			if url == "" {
				url = task.ModelURLs.USDZ
			}
			if url == "" {
				return "", true, fmt.Errorf("meshy: succeeded but no model URL")
			}
			return url, true, nil
		case "FAILED":
			msg := "failed"
			if task.TaskError.Message != "" {
				msg = task.TaskError.Message
			}
			return "", true, fmt.Errorf("meshy: task failed: %s", msg)
		default:
			// PENDING, IN_PROGRESS
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
	url, err := e.poll(ctx, apiKey, e.endpoint, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"3d"},
		Models:       []string{e.endpoint},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
}

// ConfigSchema returns the configuration fields required by the Meshy engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "MESHY_API_KEY", Description: "Meshy API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "MESHY_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "endpoint", Label: "Endpoint", Type: "string", Description: "Generation endpoint: text-to-3d or image-to-3d", Default: defaultEndpoint},
	}
}

// ModelsByCapability returns all known Meshy capabilities grouped by type.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"3d": {
			"text-to-3d",
			"image-to-3d",
		},
	}
}
