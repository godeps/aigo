// Package qwenvl implements engine.Engine for Qwen-VL multimodal understanding.
//
// Qwen-VL accepts text, image, and video inputs via the DashScope OpenAI-compatible
// Chat Completions API and returns text responses. Auth: Authorization: Bearer {api_key},
// env DASHSCOPE_API_KEY.
//
// Supported models: qwen3.6-plus, qwen-vl-max, qwen-vl-plus.
package qwenvl

import (
	"bufio"
	"bytes"
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
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL   = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultModel     = "qwen3.6-plus"
	defaultMaxTokens = 4096
	visionTimeout    = 5 * time.Minute
)

// Model constants.
const (
	ModelQwen36Plus = "qwen3.6-plus"
	ModelQwenVLMax  = "qwen-vl-max"
	ModelQwenVLPlus = "qwen-vl-plus"
)

var (
	ErrMissingAPIKey = errors.New("qwenvl: missing API key (set Config.APIKey or DASHSCOPE_API_KEY)")
	ErrMissingPrompt = errors.New("qwenvl: missing prompt in workflow graph")
)

// Config configures the Qwen-VL engine.
type Config struct {
	APIKey     string
	BaseURL    string // default: https://dashscope.aliyuncs.com/compatible-mode/v1
	Model      string // default: qwen3.6-plus
	HTTPClient *http.Client
	MaxTokens  int // default: 4096
}

// Engine implements engine.Engine for Qwen-VL multimodal understanding.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxTokens  int
}

// New creates a Qwen-VL engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, visionTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("DASHSCOPE_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		httpClient: hc,
		maxTokens:  maxTokens,
	}
}

// Execute analyses text, images, and/or videos via the DashScope Chat Completions API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "DASHSCOPE_API_KEY")
	if err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: %w", err)
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	content := e.buildContent(g, prompt)

	payload := map[string]any{
		"model":      e.model,
		"max_tokens": e.maxTokens,
		"stream":     true,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}
	mergeExtraBody(g, payload)

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return engine.Result{}, fmt.Errorf("qwenvl: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, out, "qwenvl")
	}

	var text string
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		text, err = parseSSE(resp.Body)
	} else {
		text, err = parseJSON(resp.Body)
	}
	if err != nil {
		return engine.Result{}, err
	}

	return engine.Result{Value: text, Kind: engine.OutputPlainText}, nil
}

func parseJSON(r io.Reader) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return "", fmt.Errorf("qwenvl: decode response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("qwenvl: response contained no choices")
	}
	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("qwenvl: response had empty content")
	}
	return text, nil
}

// Usage holds token usage information from the API response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func parseSSE(r io.Reader) (string, error) {
	var sb strings.Builder
	var parseErrors int
	var lastErr error
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			parseErrors++
			lastErr = err
			continue
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("qwenvl: read stream: %w", err)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		if lastErr != nil {
			return "", fmt.Errorf("qwenvl: stream produced empty content (%d parse errors, last: %w)", parseErrors, lastErr)
		}
		return "", errors.New("qwenvl: stream produced empty content")
	}
	return text, nil
}

// buildContent constructs the message content field. If LoadImage or LoadVideo
// nodes are present in the graph, it returns a multi-part array with text,
// image_url, and video_url entries. Otherwise it returns the prompt string directly.
func (e *Engine) buildContent(g workflow.Graph, prompt string) any {
	imageRefs := g.FindByClassType("LoadImage")
	videoRefs := g.FindByClassType("LoadVideo")

	if len(imageRefs) == 0 && len(videoRefs) == 0 {
		return prompt
	}

	parts := []map[string]any{}

	for _, ref := range videoRefs {
		u, ok := ref.Node.Inputs["url"].(string)
		if !ok || u == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "video_url",
			"video_url": map[string]any{"url": u},
		})
	}

	for _, ref := range imageRefs {
		u, ok := ref.Node.Inputs["url"].(string)
		if !ok || u == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": u},
		})
	}

	parts = append(parts, map[string]any{
		"type": "text",
		"text": prompt,
	})

	// If no valid media URLs were found, fall back to plain text.
	if len(parts) == 1 {
		return prompt
	}

	return parts
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"text", "image", "video"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields for the Qwen-VL engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "DASHSCOPE_API_KEY", Description: "DashScope API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "DASHSCOPE_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "model", Label: "Model", Type: "string", Description: "Qwen-VL model (qwen3.6-plus, qwen-vl-max, qwen-vl-plus)", Default: defaultModel},
		{Key: "maxTokens", Label: "Max Tokens", Type: "number", Description: "Maximum tokens in response", Default: "4096"},
	}
}

// ModelsByCapability returns known Qwen-VL models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"text":   {ModelQwen36Plus, ModelQwenVLMax, ModelQwenVLPlus},
		"image":  {ModelQwen36Plus, ModelQwenVLMax, ModelQwenVLPlus},
		"video":  {ModelQwen36Plus, ModelQwenVLMax, ModelQwenVLPlus},
		"vision": {ModelQwen36Plus, ModelQwenVLMax, ModelQwenVLPlus},
	}
}

// mergeExtraBody looks for "extra_body" or "chat_extra" in Options/Settings nodes
// and shallow-merges the JSON object into the payload.
func mergeExtraBody(g workflow.Graph, dst map[string]any) {
	for _, key := range []string{"extra_body", "chat_extra"} {
		raw, ok := resolve.StringOption(g, key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		var extra map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err != nil {
			continue
		}
		for k, v := range extra {
			dst[k] = v
		}
	}
}
