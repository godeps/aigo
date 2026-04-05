package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/godeps/aigo/workflow"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "dall-e-3"
	defaultSize    = "1024x1024"
)

var ErrMissingPrompt = errors.New("openai: prompt not found in workflow graph")

// Config configures the OpenAI image engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Quality    string
	Style      string
	HTTPClient *http.Client
}

// Request is the flattened image generation payload derived from a graph.
type Request struct {
	Model   string
	Prompt  string
	Size    string
	Quality string
	Style   string
}

// Engine compiles a workflow graph into an OpenAI image request.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	quality    string
	style      string
	httpClient *http.Client
}

// New creates an OpenAI engine instance.
func New(cfg Config) *Engine {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	quality := cfg.Quality
	if quality == "" {
		quality = "standard"
	}

	return &Engine{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		model:      model,
		quality:    quality,
		style:      cfg.Style,
		httpClient: httpClient,
	}
}

// Compile extracts prompt and size from a graph into an OpenAI request.
func (e *Engine) Compile(graph workflow.Graph) (Request, error) {
	if err := graph.Validate(); err != nil {
		return Request{}, fmt.Errorf("openai: validate graph: %w", err)
	}

	req := Request{
		Model:   e.model,
		Quality: e.quality,
		Style:   e.style,
		Size:    defaultSize,
	}

	for _, ref := range graph.FindByClassType("CLIPTextEncode") {
		prompt, ok, err := resolveNodeString(graph, ref.ID, map[string]bool{})
		if err != nil {
			return Request{}, fmt.Errorf("openai: resolve prompt from node %q: %w", ref.ID, err)
		}
		if ok && strings.TrimSpace(prompt) != "" {
			req.Prompt = prompt
			break
		}
	}

	for _, ref := range graph.FindByClassType("EmptyLatentImage") {
		width, okW := ref.Node.IntInput("width")
		height, okH := ref.Node.IntInput("height")
		if okW && okH {
			req.Size = normalizeSize(width, height)
			break
		}
	}

	if req.Prompt == "" {
		return Request{}, ErrMissingPrompt
	}

	return req, nil
}

// Execute compiles the workflow and calls the OpenAI images API.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (string, error) {
	req, err := e.Compile(graph)
	if err != nil {
		return "", err
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return "", errors.New("openai: missing API key")
	}

	payload := map[string]any{
		"model":           req.Model,
		"prompt":          req.Prompt,
		"size":            req.Size,
		"quality":         req.Quality,
		"n":               1,
		"response_format": "url",
	}
	if req.Style != "" {
		payload["style"] = req.Style
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai: create image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai: create image request failed with status %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("openai: decode response: %w", err)
	}

	if len(decoded.Data) == 0 {
		return "", errors.New("openai: response did not contain generated images")
	}

	if decoded.Data[0].URL != "" {
		return decoded.Data[0].URL, nil
	}
	if decoded.Data[0].B64JSON != "" {
		return "data:image/png;base64," + decoded.Data[0].B64JSON, nil
	}

	return "", errors.New("openai: response did not contain a usable image result")
}

func resolveNodeString(graph workflow.Graph, nodeID string, seen map[string]bool) (string, bool, error) {
	if seen[nodeID] {
		return "", false, fmt.Errorf("cycle detected at node %q", nodeID)
	}
	seen[nodeID] = true

	node, ok := graph[nodeID]
	if !ok {
		return "", false, fmt.Errorf("node %q not found", nodeID)
	}

	if value, ok := node.StringInput("text"); ok && strings.TrimSpace(value) != "" {
		return value, true, nil
	}

	for _, key := range []string{"text", "prompt", "string", "value"} {
		value, exists := node.Input(key)
		if !exists {
			continue
		}
		resolved, ok, err := resolveValueString(graph, value, seen)
		if err != nil {
			return "", false, err
		}
		if ok && strings.TrimSpace(resolved) != "" {
			return resolved, true, nil
		}
	}

	return "", false, nil
}

func resolveValueString(graph workflow.Graph, value any, seen map[string]bool) (string, bool, error) {
	switch v := value.(type) {
	case string:
		return v, true, nil
	case []any:
		return resolveLinkString(graph, v, seen)
	default:
		return "", false, nil
	}
}

func resolveLinkString(graph workflow.Graph, ref []any, seen map[string]bool) (string, bool, error) {
	if len(ref) == 0 {
		return "", false, nil
	}

	nodeID, ok := ref[0].(string)
	if !ok {
		return "", false, nil
	}

	nextSeen := make(map[string]bool, len(seen))
	for k, v := range seen {
		nextSeen[k] = v
	}

	return resolveNodeString(graph, nodeID, nextSeen)
}

func normalizeSize(width, height int) string {
	switch {
	case width == 1024 && height == 1024:
		return "1024x1024"
	case width == 1024 && height == 1536:
		return "1024x1536"
	case width == 1536 && height == 1024:
		return "1536x1024"
	case width > height:
		return "1536x1024"
	case height > width:
		return "1024x1536"
	default:
		return defaultSize
	}
}
