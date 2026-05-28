// Package gemini implements engine.Engine for Google Gemini multi-modal understanding.
//
// Gemini supports text generation with optional image/video inputs for analysis.
// Auth: API key via official SDK, env GEMINI_API_KEY or GOOGLE_API_KEY.
//
// Endpoint: POST /models/{model}:generateContent
// Default model: gemini-2.0-flash
package gemini

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
	"google.golang.org/genai"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Model constants.
const (
	ModelGemini20Flash     = "gemini-2.0-flash"
	ModelGemini15Pro       = "gemini-1.5-pro"
	ModelGemini20FlashLite = "gemini-2.0-flash-lite"
	ModelGemini15Flash     = "gemini-1.5-flash"
)

var (
	ErrMissingAPIKey = errors.New("gemini: missing API key (set Config.APIKey or GEMINI_API_KEY / GOOGLE_API_KEY)")
	ErrMissingPrompt = errors.New("gemini: missing prompt in workflow graph")
)

// Config configures the Gemini engine.
type Config struct {
	APIKey     string
	BaseURL    string // default: https://generativelanguage.googleapis.com
	Model      string // default: gemini-2.0-flash
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Google Gemini multi-modal understanding.
type Engine struct {
	client *genai.Client
	model  string
}

// New creates a Gemini engine instance.
func New(cfg Config) (*Engine, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelGemini20Flash
	}

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("GEMINI_BASE_URL")), "/")
	}

	cc := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if cfg.HTTPClient != nil {
		cc.HTTPClient = cfg.HTTPClient
	}
	if base != "" {
		cc.HTTPOptions = genai.HTTPOptions{BaseURL: base}
	}

	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}

	return &Engine{
		client: client,
		model:  model,
	}, nil
}

// Execute sends a multi-modal request to the Gemini API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("gemini: validate graph: %w", err)
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gemini: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	parts := []*genai.Part{{Text: prompt}}

	for _, ref := range g.FindByClassType("LoadImage") {
		if p := buildSDKPart(ref, "image/jpeg"); p != nil {
			parts = append(parts, p)
		}
	}

	for _, ref := range g.FindByClassType("LoadVideo") {
		if p := buildSDKPart(ref, "video/mp4"); p != nil {
			parts = append(parts, p)
		}
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	resp, err := e.client.Models.GenerateContent(ctx, e.model, contents, nil)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gemini: generate content: %w", err)
	}

	text := resp.Text()
	if text == "" && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		var sb strings.Builder
		for _, p := range resp.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
		text = sb.String()
	}

	return engine.Result{Value: text, Kind: engine.OutputPlainText}, nil
}

// buildSDKPart creates a genai.Part from a LoadImage or LoadVideo node reference.
func buildSDKPart(ref workflow.NodeRef, defaultMime string) *genai.Part {
	if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
		mime := defaultMime
		if m, ok := ref.Node.Inputs["mime_type"].(string); ok && m != "" {
			mime = m
		}
		return &genai.Part{FileData: &genai.FileData{MIMEType: mime, FileURI: u}}
	}
	if d, ok := ref.Node.Inputs["data"].(string); ok && d != "" {
		mime := defaultMime
		if m, ok := ref.Node.Inputs["mime_type"].(string); ok && m != "" {
			mime = m
		}
		data, err := base64.StdEncoding.DecodeString(d)
		if err != nil {
			return nil
		}
		return &genai.Part{InlineData: &genai.Blob{MIMEType: mime, Data: data}}
	}
	return nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"text", "image", "video"},
		Models:       []string{e.model},
		SupportsSync: true,
		SupportsPoll: false,
	}
}

// ConfigSchema returns the configuration fields for the Gemini engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "GEMINI_API_KEY", Description: "Gemini API key (or GOOGLE_API_KEY)"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "GEMINI_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "model", Label: "Model", Type: "string", Description: "Gemini model (gemini-2.0-flash, gemini-1.5-pro, etc.)", Default: ModelGemini20Flash},
	}
}

// ModelsByCapability returns known Gemini models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"text": {
			ModelGemini20Flash,
			ModelGemini15Pro,
			ModelGemini20FlashLite,
			ModelGemini15Flash,
		},
		"image": {
			ModelGemini20Flash,
			ModelGemini15Pro,
			ModelGemini20FlashLite,
			ModelGemini15Flash,
		},
		"video": {
			ModelGemini20Flash,
			ModelGemini15Pro,
			ModelGemini15Flash,
		},
		"vision": {
			ModelGemini20Flash,
			ModelGemini15Pro,
			ModelGemini20FlashLite,
			ModelGemini15Flash,
		},
	}
}
