// Package comfydeploy implements engine.Engine for the ComfyDeploy API.
//
// Workflow execution is async: POST /run creates a run, then GET /run?run_id={id}
// polls for completion. Auth: Authorization: Bearer {token}, env COMFYDEPLOY_API_TOKEN.
package comfydeploy

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
)

const (
	defaultBaseURL      = "https://www.comfydeploy.com/api"
	defaultPollInterval = 3 * time.Second
)

var ErrMissingDeploymentID = fmt.Errorf("comfydeploy: deployment ID is required")

// Config configures the ComfyDeploy engine.
type Config struct {
	APIToken          string
	BaseURL           string
	DeploymentID      string // required
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	Webhook           string // optional
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for ComfyDeploy.
type Engine struct {
	apiToken     string
	baseURL      string
	deploymentID string
	httpClient   *http.Client
	waitForCompletion bool
	pollInterval time.Duration
	webhook      string
	onProgress   epoll.OnProgress
}

// New creates a ComfyDeploy engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("COMFYDEPLOY_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiToken:          strings.TrimSpace(cfg.APIToken),
		baseURL:           base,
		deploymentID:      strings.TrimSpace(cfg.DeploymentID),
		httpClient:        hc,
		waitForCompletion: cfg.WaitForCompletion,
		pollInterval:      poll,
		webhook:           strings.TrimSpace(cfg.Webhook),
		onProgress:        cfg.OnProgress,
	}
}

// Execute runs a workflow on ComfyDeploy.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("comfydeploy: validate graph: %w", err)
	}

	apiToken, err := e.resolveAPIToken()
	if err != nil {
		return engine.Result{}, err
	}

	if e.deploymentID == "" {
		return engine.Result{}, ErrMissingDeploymentID
	}

	inputs := buildInputs(g)

	payload := map[string]any{
		"deployment_id": e.deploymentID,
		"inputs":        inputs,
	}
	if e.webhook != "" {
		payload["webhook"] = e.webhook
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("comfydeploy: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/run", apiToken, body, "comfydeploy")
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("comfydeploy: decode create: %w", err)
	}
	if created.RunID == "" {
		return engine.Result{}, fmt.Errorf("comfydeploy: create returned empty run_id")
	}

	if !e.waitForCompletion {
		return engine.Result{Value: created.RunID, Kind: engine.OutputPlainText}, nil
	}

	outputURL, err := e.poll(ctx, apiToken, created.RunID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: outputURL, Kind: engine.OutputURL}, nil
}

// buildInputs extracts workflow inputs from the graph for ComfyDeploy.
func buildInputs(g workflow.Graph) map[string]string {
	inputs := make(map[string]string)

	for _, ref := range g.FindByClassType("CLIPTextEncode") {
		if t, ok := ref.Node.Inputs["text"].(string); ok && strings.TrimSpace(t) != "" {
			inputs["prompt"] = t
			break
		}
	}

	for _, ref := range g.FindByClassType("NegativePrompt") {
		if t, ok := ref.Node.Inputs["text"].(string); ok && strings.TrimSpace(t) != "" {
			inputs["negative_prompt"] = t
			break
		}
	}

	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && strings.TrimSpace(u) != "" {
			inputs["image"] = u
			break
		}
	}

	for _, ref := range g.FindByClassType("LoadVideo") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && strings.TrimSpace(u) != "" {
			inputs["video"] = u
			break
		}
	}

	// Flatten any other top-level string inputs from all nodes.
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for k, v := range node.Inputs {
			if _, alreadySet := inputs[k]; alreadySet {
				continue
			}
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				inputs[k] = s
			}
		}
	}

	return inputs
}

// pollResponse mirrors the GET /run response shape.
type pollResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Outputs []struct {
		Data struct {
			Images []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
			} `json:"images"`
			Files []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
			} `json:"files"`
			GIFs []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
			} `json:"gifs"`
		} `json:"data"`
	} `json:"outputs"`
}

func (e *Engine) poll(ctx context.Context, apiToken, runID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/run", nil)
		if err != nil {
			return "", false, fmt.Errorf("comfydeploy: build poll: %w", err)
		}
		q := req.URL.Query()
		q.Set("run_id", runID)
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+apiToken)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("comfydeploy: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("comfydeploy: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "comfydeploy")
		}

		var pr pollResponse
		if err := json.Unmarshal(body, &pr); err != nil {
			return "", false, fmt.Errorf("comfydeploy: decode poll: %w", err)
		}

		switch pr.Status {
		case "success":
			u := firstOutputURL(pr)
			if u == "" {
				return "", true, fmt.Errorf("comfydeploy: succeeded but no output URL")
			}
			return u, true, nil
		case "failed":
			return "", true, fmt.Errorf("comfydeploy: run failed")
		case "timeout":
			return "", true, fmt.Errorf("comfydeploy: run timed out")
		default:
			return "", false, nil
		}
	})
}

// firstOutputURL extracts the first available URL from a poll response.
func firstOutputURL(pr pollResponse) string {
	for _, out := range pr.Outputs {
		for _, img := range out.Data.Images {
			if img.URL != "" {
				return img.URL
			}
		}
		for _, f := range out.Data.Files {
			if f.URL != "" {
				return f.URL
			}
		}
		for _, g := range out.Data.GIFs {
			if g.URL != "" {
				return g.URL
			}
		}
	}
	return ""
}

func (e *Engine) resolveAPIToken() (string, error) {
	return engine.ResolveKey(e.apiToken, "COMFYDEPLOY_API_TOKEN")
}

// Resume implements engine.Resumer — resumes polling a previously submitted run.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiToken, err := e.resolveAPIToken()
	if err != nil {
		return engine.Result{}, err
	}
	url, err := e.poll(ctx, apiToken, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image", "video"},
		SupportsPoll: e.waitForCompletion,
		SupportsSync: !e.waitForCompletion,
	}
}

// ConfigSchema returns the configuration fields required by the ComfyDeploy engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiToken", Label: "API Token", Type: "secret", Required: true, EnvVar: "COMFYDEPLOY_API_TOKEN", Description: "ComfyDeploy API token"},
		{Key: "deploymentId", Label: "Deployment ID", Type: "string", Required: true, Description: "ComfyDeploy deployment ID"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "COMFYDEPLOY_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "webhook", Label: "Webhook URL", Type: "url", Description: "Optional webhook URL for run completion notifications"},
	}
}
