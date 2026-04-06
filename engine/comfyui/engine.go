package comfyui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
)

const defaultPollInterval = 2 * time.Second

// Config configures the ComfyUI passthrough engine.
type Config struct {
	BaseURL           string
	ClientID          string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine submits a graph to a ComfyUI server.
type Engine struct {
	baseURL           string
	clientID          string
	httpClient        *http.Client
	waitForCompletion bool
	pollInterval      time.Duration
}

// New creates a ComfyUI engine instance.
func New(cfg Config) *Engine {
	httpClient := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	return &Engine{
		baseURL:           strings.TrimRight(cfg.BaseURL, "/"),
		clientID:          cfg.ClientID,
		httpClient:        httpClient,
		waitForCompletion: cfg.WaitForCompletion,
		pollInterval:      pollInterval,
	}
}

// Execute enqueues the graph on a ComfyUI server and returns either the prompt id or the first output URL.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	if e.baseURL == "" {
		return engine.Result{}, errors.New("comfyui: base URL is required")
	}
	if err := graph.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: validate graph: %w", err)
	}

	payload := struct {
		ClientID string         `json:"client_id,omitempty"`
		Prompt   workflow.Graph `json:"prompt"`
	}{
		ClientID: e.clientID,
		Prompt:   graph,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: marshal prompt: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: build prompt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: enqueue prompt: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: read prompt response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "comfyui")
	}

	var queued struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.Unmarshal(respBody, &queued); err != nil {
		return engine.Result{}, fmt.Errorf("comfyui: decode prompt response: %w", err)
	}
	if queued.PromptID == "" {
		return engine.Result{}, errors.New("comfyui: prompt response did not include prompt_id")
	}

	if !e.waitForCompletion {
		return engine.Result{Value: queued.PromptID, Kind: engine.OutputPlainText}, nil
	}

	result, err := e.waitForResult(ctx, queued.PromptID)
	if err != nil {
		return engine.Result{}, err
	}
	if result == "" {
		return engine.Result{Value: queued.PromptID, Kind: engine.OutputPlainText}, nil
	}
	return engine.Result{Value: result, Kind: engine.OutputURL}, nil
}

func (e *Engine) waitForResult(ctx context.Context, promptID string) (string, error) {
	return poll.Poll(ctx, poll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		result, done, err := e.fetchResult(ctx, promptID)
		if err != nil {
			return "", false, err
		}
		if done {
			return result, true, nil
		}
		return "", false, nil
	})
}

func (e *Engine) fetchResult(ctx context.Context, promptID string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/history/"+url.PathEscape(promptID), nil)
	if err != nil {
		return "", false, fmt.Errorf("comfyui: build history request: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("comfyui: fetch history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("comfyui: read history response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, aigoerr.FromHTTPResponse(resp, body, "comfyui")
	}

	item, ok, err := decodeHistoryItem(body, promptID)
	if err != nil {
		return "", false, fmt.Errorf("comfyui: decode history response: %w", err)
	}
	if !ok {
		return "", false, nil
	}

	return firstOutputURL(e.baseURL, item), true, nil
}

type historyItem struct {
	Outputs map[string]historyOutputs `json:"outputs"`
}

type historyOutputs struct {
	Images []historyAsset `json:"images,omitempty"`
	GIFs   []historyAsset `json:"gifs,omitempty"`
	Videos []historyAsset `json:"videos,omitempty"`
}

type historyAsset struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

func decodeHistoryItem(body []byte, promptID string) (historyItem, bool, error) {
	var direct historyItem
	if err := json.Unmarshal(body, &direct); err == nil && direct.Outputs != nil {
		return direct, true, nil
	}

	var wrapped map[string]historyItem
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return historyItem{}, false, err
	}
	if item, ok := wrapped[promptID]; ok {
		return item, true, nil
	}

	return historyItem{}, false, nil
}

func firstOutputURL(baseURL string, item historyItem) string {
	for _, outputs := range item.Outputs {
		for _, assets := range [][]historyAsset{outputs.Images, outputs.GIFs, outputs.Videos} {
			for _, asset := range assets {
				if asset.Filename == "" {
					continue
				}
				return buildViewURL(baseURL, asset)
			}
		}
	}
	return ""
}

func buildViewURL(baseURL string, asset historyAsset) string {
	values := url.Values{}
	values.Set("filename", asset.Filename)
	if asset.Subfolder != "" {
		values.Set("subfolder", asset.Subfolder)
	}
	if asset.Type != "" {
		values.Set("type", asset.Type)
	}
	return strings.TrimRight(baseURL, "/") + "/view?" + values.Encode()
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image", "video"},
		SupportsSync: !e.waitForCompletion,
		SupportsPoll: e.waitForCompletion,
	}
}
