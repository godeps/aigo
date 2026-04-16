// Package hailuo implements engine.Engine for Hailuo (MiniMax) video generation.
//
// Video generation is async: POST /v1/video_generation creates a task,
// then GET /v1/query/video_generation?task_id={id} polls for completion.
// Auth: Authorization: Bearer {api_key}, env HAILUO_API_KEY.
//
// Supported models: T2V-01 (text-to-video), I2V-01 (image-to-video),
// S2V-01 (subject-to-video), T2V-01-Director (camera control).
package hailuo

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
	defaultBaseURL      = "https://api.minimaxi.chat"
	defaultPollInterval = 5 * time.Second
)

// Model constants.
const (
	ModelT2V01         = "T2V-01"
	ModelI2V01         = "I2V-01"
	ModelS2V01         = "S2V-01"
	ModelT2V01Director = "T2V-01-Director"
)

var (
	ErrMissingAPIKey = errors.New("hailuo: missing API key (set Config.APIKey or HAILUO_API_KEY)")
	ErrMissingPrompt = errors.New("hailuo: missing prompt in workflow graph")
)

// Config configures the Hailuo video engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string // default: T2V-01
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Hailuo video generation.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a Hailuo video engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("HAILUO_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelT2V01
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

// Execute generates a video via the Hailuo/MiniMax video API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("hailuo: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("hailuo: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
	}

	// Detect image reference for I2V (image-to-video).
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["first_frame_image"] = u
			break
		}
	}

	resolve.MergeJSONOption(g, payload, "extra", "extra_params")

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("hailuo: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/v1/video_generation", apiKey, body, "hailuo")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		TaskID string `json:"task_id"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("hailuo: decode create: %w", err)
	}
	if created.BaseResp.StatusCode != 0 {
		return engine.Result{}, fmt.Errorf("hailuo: API error %d: %s", created.BaseResp.StatusCode, created.BaseResp.StatusMsg)
	}
	if created.TaskID == "" {
		return engine.Result{}, fmt.Errorf("hailuo: create returned empty task_id")
	}

	if !e.waitResult {
		return engine.Result{Value: created.TaskID, Kind: engine.OutputPlainText}, nil
	}

	videoURL, err := e.poll(ctx, apiKey, created.TaskID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: videoURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) resolveAPIKey() (string, error) {
	ak := e.apiKey
	if ak == "" {
		ak = os.Getenv("HAILUO_API_KEY")
	}
	if ak == "" {
		ak = os.Getenv("MINIMAX_API_KEY")
	}
	if ak == "" {
		return "", ErrMissingAPIKey
	}
	return ak, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		url := e.baseURL + "/v1/query/video_generation?task_id=" + taskID
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("hailuo: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("hailuo: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("hailuo: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "hailuo")
		}

		var task struct {
			Status string `json:"status"`
			FileID string `json:"file_id"`
			BaseResp struct {
				StatusCode int    `json:"status_code"`
				StatusMsg  string `json:"status_msg"`
			} `json:"base_resp"`
		}
		if err := json.Unmarshal(body, &task); err != nil {
			return "", false, fmt.Errorf("hailuo: decode poll: %w", err)
		}

		switch task.Status {
		case "Success":
			if task.FileID == "" {
				return "", true, fmt.Errorf("hailuo: succeeded but no file_id")
			}
			// Build download URL from file_id.
			downloadURL := e.baseURL + "/v1/files/retrieve?file_id=" + task.FileID
			return downloadURL, true, nil
		case "Failed":
			msg := "task failed"
			if task.BaseResp.StatusMsg != "" {
				msg = task.BaseResp.StatusMsg
			}
			return "", true, fmt.Errorf("hailuo: %s", msg)
		default:
			// Processing, Queueing, Preparing — still in progress
			return "", false, nil
		}
	})
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
		MediaTypes:   []string{"video"},
		Models:       []string{e.model},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
}

// ConfigSchema returns the configuration fields for the Hailuo engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "HAILUO_API_KEY", Description: "Hailuo/MiniMax API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "HAILUO_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "model", Label: "Model", Type: "string", Description: "Video model (T2V-01, I2V-01, S2V-01, T2V-01-Director)", Default: ModelT2V01},
	}
}

// ModelsByCapability returns known Hailuo models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			ModelT2V01,
			ModelI2V01,
			ModelS2V01,
			ModelT2V01Director,
		},
	}
}
