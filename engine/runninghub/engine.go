// Package runninghub implements engine.Engine for the RunningHub API.
//
// Generation is async: POST {base_url}/{endpoint} submits a task,
// then POST {base_url}/query polls for completion.
// Auth: Authorization: Bearer {key}, env RH_API_KEY.
package runninghub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/httpx"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL      = "https://www.runninghub.cn/openapi/v2"
	defaultPollInterval = 5 * time.Second
)

var (
	ErrMissingAPIKey  = errors.New("runninghub: missing API key (set Config.APIKey or RH_API_KEY)")
	ErrMissingEndpoint = errors.New("runninghub: missing endpoint")
)

// Config configures the RunningHub engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Endpoint          string // model-specific, e.g. "generate/video"
	Model             string // model identifier
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for RunningHub.
type Engine struct {
	apiKey       string
	baseURL      string
	endpoint     string
	model        string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a RunningHub engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("RH_BASE_URL")), "/")
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
		endpoint:     strings.TrimSpace(cfg.Endpoint),
		model:        strings.TrimSpace(cfg.Model),
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute submits a generation task to the RunningHub API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("runninghub: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	if e.endpoint == "" {
		return engine.Result{}, ErrMissingEndpoint
	}

	payload, err := e.buildPayload(g)
	if err != nil {
		return engine.Result{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("runninghub: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/"+e.endpoint, apiKey, body, "runninghub")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("runninghub: decode create: %w", err)
	}
	if created.TaskID == "" {
		return engine.Result{}, fmt.Errorf("runninghub: create returned empty taskId")
	}

	if !e.waitResult {
		return engine.Result{Value: created.TaskID, Kind: engine.OutputPlainText}, nil
	}

	resultURL, err := e.poll(ctx, apiKey, created.TaskID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: resultURL, Kind: engine.OutputURL}, nil
}

// buildPayload constructs the request payload from the workflow graph.
func (e *Engine) buildPayload(g workflow.Graph) (map[string]any, error) {
	payload := map[string]any{}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return nil, fmt.Errorf("runninghub: %w", err)
	}
	if prompt != "" {
		payload["prompt"] = prompt
	}

	if e.model != "" {
		payload["model"] = e.model
	}

	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["imageUrl"] = u
			break
		}
	}

	for _, ref := range g.FindByClassType("LoadVideo") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			payload["videoUrl"] = u
			break
		}
	}

	for _, ref := range g.FindByClassType("NegativePrompt") {
		if t, ok := ref.Node.Inputs["text"].(string); ok && t != "" {
			payload["negative_prompt"] = t
			break
		}
	}

	if size, ok := resolve.StringOption(g, "size"); ok && size != "" {
		payload["size"] = size
	}

	if duration, ok := resolve.IntOption(g, "duration"); ok && duration > 0 {
		payload["duration"] = duration
	}

	resolve.MergeJSONOption(g, payload, "extra", "params")

	return payload, nil
}

// poll waits for a RunningHub task to complete, returning the result URL or text.
func (e *Engine) poll(ctx context.Context, apiKey, taskID string) (string, error) {
	queryBody, _ := json.Marshal(map[string]any{"taskId": taskID})

	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/query", apiKey, queryBody, "runninghub")
		if err != nil {
			return "", false, err
		}

		var task struct {
			Status       string `json:"status"`
			ErrorCode    string `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
			Results      []struct {
				URL  string `json:"url"`
				Text string `json:"text"`
			} `json:"results"`
		}
		if err := json.Unmarshal(respBody, &task); err != nil {
			return "", false, fmt.Errorf("runninghub: decode poll: %w", err)
		}

		switch strings.ToUpper(task.Status) {
		case "SUCCESS":
			if len(task.Results) == 0 {
				return "", true, fmt.Errorf("runninghub: succeeded but no results")
			}
			r := task.Results[0]
			if r.URL != "" {
				return r.URL, true, nil
			}
			return r.Text, true, nil
		case "FAILED", "CANCEL":
			msg := "task " + strings.ToLower(task.Status)
			if task.ErrorMessage != "" {
				msg = task.ErrorMessage
			} else if task.ErrorCode != "" {
				msg = task.ErrorCode
			}
			return "", true, fmt.Errorf("runninghub: %s", msg)
		default:
			// RUNNING, QUEUED, CREATE — still in progress
			return "", false, nil
		}
	})
}

func (e *Engine) resolveAPIKey() (string, error) {
	ak := e.apiKey
	if ak == "" {
		ak = os.Getenv("RH_API_KEY")
	}
	if ak == "" {
		return "", ErrMissingAPIKey
	}
	return ak, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}
	resultURL, err := e.poll(ctx, apiKey, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: resultURL, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image", "video", "audio", "3d", "text"},
		Models:       []string{e.model},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
}

// ConfigSchema returns the configuration fields required by the RunningHub engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "RH_API_KEY", Description: "RunningHub API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "RH_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "endpoint", Label: "Endpoint", Type: "string", Required: true, Description: "Model-specific endpoint path, e.g. generate/video"},
		{Key: "model", Label: "Model", Type: "string", Description: "Model identifier to use for generation"},
	}
}

// ModelsByCapability returns all known RunningHub models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			"nano-banana-v1",
			"flux-dev",
		},
		"video": {
			"kling-v2.5",
			"seedance-v2.0",
		},
	}
}
