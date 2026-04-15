package ark

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
)

const (
	defaultBaseURL      = "https://ark.cn-beijing.volces.com"
	defaultPollInterval = 5 * time.Second
	tasksPath           = "/api/v3/contents/generations/tasks"
)

var (
	ErrMissingAPIKey  = errors.New("ark: missing API key (set Config.APIKey or ARK_API_KEY)")
	ErrMissingBaseURL = errors.New("ark: BaseURL is empty")
	ErrMissingModel   = errors.New("ark: Model is empty")
	ErrMissingContent = errors.New("ark: no content provided (need at least a text prompt or image)")
)

// Config configures the Ark video generation engine.
type Config struct {
	APIKey  string
	BaseURL string // e.g. "https://ark.cn-beijing.volces.com"
	Model   string // Model ID or Endpoint ID, e.g. "doubao-seedance-2-0-260128"

	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Volcengine Ark content generation.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitVideo    bool
	pollInterval time.Duration
}

// New creates an Ark engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("ARK_BASE_URL")), "/")
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
		waitVideo:    cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// imageModels lists models that use the /images/generations endpoint.
var imageModels = map[string]bool{
	ModelSeedream3_0: true,
	ModelSeedream2_1: true,
}

// Execute creates a media generation task.
// Image models use the synchronous /images/generations endpoint.
// Video models use the async /contents/generations/tasks endpoint with polling.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("ark: validate graph: %w", err)
	}
	if e.model == "" {
		return engine.Result{}, ErrMissingModel
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("ARK_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	// Route image models to synchronous endpoint.
	if imageModels[e.model] {
		url, err := runImageGeneration(ctx, e, apiKey, g)
		if err != nil {
			return engine.Result{}, err
		}
		return engine.Result{Value: url, Kind: engine.ClassifyOutput(url)}, nil
	}

	// Video models: async content generation.
	payload, err := e.buildPayload(g)
	if err != nil {
		return engine.Result{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ark: marshal create: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+tasksPath, apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("ark: decode create: %w", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		return engine.Result{}, fmt.Errorf("ark: create missing id: %s", strings.TrimSpace(string(respBody)))
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

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	if imageModels[e.model] {
		return engine.Capability{
			MediaTypes:   []string{"image"},
			Models:       []string{e.model},
			SupportsSync: true,
		}
	}
	return engine.Capability{
		MediaTypes:   []string{"video"},
		Models:       []string{e.model},
		MaxDuration:  15,
		SupportsPoll: e.waitVideo,
		SupportsSync: !e.waitVideo,
	}
}

// ConfigSchema returns the configuration fields required by the Ark engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "ARK_API_KEY", Description: "Volcengine Ark API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "ARK_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Ark (Volcengine) models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {
			"doubao-seedance-2-0-260128",
			"doubao-seedance-1-0-lite-250428",
		},
		"image": {
			ModelSeedream3_0,
			ModelSeedream2_1,
		},
	}
}

func (e *Engine) buildPayload(g workflow.Graph) (map[string]any, error) {
	payload := map[string]any{
		"model": e.model,
	}

	var content []map[string]any

	// text prompt
	if prompt := extractPrompt(g); prompt != "" {
		content = append(content, map[string]any{
			"type": "text",
			"text": prompt,
		})
	}

	// images
	content = appendImages(g, content)

	// videos
	content = appendVideos(g, content)

	// audios
	content = appendAudios(g, content)

	if len(content) == 0 {
		return nil, ErrMissingContent
	}
	payload["content"] = content

	// optional parameters
	if v, ok := stringOption(g, "ratio"); ok {
		payload["ratio"] = v
	}
	if v, ok := stringOption(g, "resolution"); ok {
		payload["resolution"] = v
	}
	if d := extractDuration(g); d != 0 {
		payload["duration"] = d
	}
	if v, ok := intOption(g, "seed"); ok {
		payload["seed"] = v
	}
	if v, ok := boolOption(g, "generate_audio"); ok {
		payload["generate_audio"] = v
	}
	if v, ok := boolOption(g, "watermark"); ok {
		payload["watermark"] = v
	}
	if v, ok := boolOption(g, "return_last_frame"); ok {
		payload["return_last_frame"] = v
	}
	if v, ok := boolOption(g, "draft"); ok {
		payload["draft"] = v
	}
	if v, ok := stringOption(g, "service_tier"); ok {
		payload["service_tier"] = v
	}
	if v, ok := stringOption(g, "callback_url"); ok {
		payload["callback_url"] = v
	}
	if v, ok := intOption(g, "execution_expires_after"); ok {
		payload["execution_expires_after"] = v
	}

	// extra body merge
	mergeJSONOption(g, payload, "extra_body", "ark_extra")

	return payload, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+tasksPath+"/"+taskID, nil)
		if err != nil {
			return "", false, fmt.Errorf("ark: build get: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("ark: get task: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("ark: read get: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "ark")
		}
		return parseTaskResponse(body)
	})
}

func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	return httpx.DoJSON(ctx, e.httpClient, method, url, apiKey, body, "ark")
}

type taskResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Content *struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseTaskResponse(body []byte) (videoURL string, done bool, err error) {
	var task taskResponse
	if err := json.Unmarshal(body, &task); err != nil {
		return "", false, fmt.Errorf("ark: decode task: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "succeeded":
		if task.Content != nil && strings.TrimSpace(task.Content.VideoURL) != "" {
			return strings.TrimSpace(task.Content.VideoURL), true, nil
		}
		return "", true, fmt.Errorf("ark: task succeeded but no video_url")
	case "failed":
		msg := "failed"
		if task.Error != nil && task.Error.Message != "" {
			msg = task.Error.Message
		}
		return "", true, fmt.Errorf("ark: task failed: %s", msg)
	case "expired":
		return "", true, fmt.Errorf("ark: task expired")
	case "cancelled":
		return "", true, fmt.Errorf("ark: task cancelled")
	default:
		// queued, running
		return "", false, nil
	}
}
