// Package gpt4o implements engine.Engine for OpenAI GPT-4o vision understanding.
//
// GPT-4o accepts text and image inputs via the Chat Completions API and returns
// text responses. Auth: Authorization: Bearer {api_key}, env OPENAI_API_KEY.
//
// Supported models: gpt-4o, gpt-4o-mini, gpt-4-turbo.
package gpt4o

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL   = "https://api.openai.com/v1"
	defaultModel     = "gpt-4o"
	defaultMaxTokens = 4096
)

// Model constants.
const (
	ModelGPT4o     = "gpt-4o"
	ModelGPT4oMini = "gpt-4o-mini"
	ModelGPT4Turbo = "gpt-4-turbo"
)

var (
	ErrMissingAPIKey = errors.New("gpt4o: missing API key (set Config.APIKey or OPENAI_API_KEY)")
	ErrMissingPrompt = errors.New("gpt4o: missing prompt in workflow graph")
)

// Config configures the GPT-4o vision engine.
type Config struct {
	APIKey     string
	BaseURL    string // default: https://api.openai.com/v1
	Model      string // default: gpt-4o
	HTTPClient *http.Client
	MaxTokens  int // default: 4096
}

// Engine implements engine.Engine for GPT-4o vision understanding.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxTokens  int
}

// New creates a GPT-4o vision engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
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

// Execute analyses text and/or images via the OpenAI Chat Completions API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("gpt4o: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "OPENAI_API_KEY")
	if err != nil {
		return engine.Result{}, fmt.Errorf("gpt4o: %w", err)
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gpt4o: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	// Build message content: either a plain string or multi-part with images.
	content := e.buildContent(g, prompt)

	payload := map[string]any{
		"model":      e.model,
		"max_tokens": e.maxTokens,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gpt4o: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/chat/completions", apiKey, body, "gpt4o")
	if err != nil {
		return engine.Result{}, err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("gpt4o: decode response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return engine.Result{}, errors.New("gpt4o: response contained no choices")
	}

	text := resp.Choices[0].Message.Content
	return engine.Result{Value: text, Kind: engine.OutputPlainText}, nil
}

// buildContent constructs the message content field. If LoadImage nodes are
// present in the graph, it returns a multi-part array with text and image_url
// entries. Otherwise it returns the prompt string directly.
func (e *Engine) buildContent(g workflow.Graph, prompt string) any {
	imageRefs := g.FindByClassType("LoadImage")
	if len(imageRefs) == 0 {
		return prompt
	}

	parts := []map[string]any{
		{"type": "text", "text": prompt},
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

	// If no valid image URLs were found, fall back to plain text.
	if len(parts) == 1 {
		return prompt
	}

	return parts
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"text", "image"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields for the GPT-4o engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "OPENAI_API_KEY", Description: "OpenAI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "OPENAI_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "model", Label: "Model", Type: "string", Description: "Vision model (gpt-4o, gpt-4o-mini, gpt-4-turbo)", Default: defaultModel},
		{Key: "maxTokens", Label: "Max Tokens", Type: "number", Description: "Maximum tokens in response", Default: "4096"},
	}
}

// ModelsByCapability returns known GPT-4o models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"text":  {ModelGPT4o, ModelGPT4oMini, ModelGPT4Turbo},
		"image": {ModelGPT4o, ModelGPT4oMini, ModelGPT4Turbo},
	}
}
