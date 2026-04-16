// Package recraft implements engine.Engine for the Recraft AI API.
//
// Image generation uses POST /v1/images/generations (OpenAI-compatible).
// Auth: Authorization: Bearer {key}, env RECRAFT_API_KEY.
// Synchronous — returns image URL directly.
package recraft

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

const defaultBaseURL = "https://external.api.recraft.ai"

// Model constants.
const (
	ModelRecraftV3  = "recraftv3"
	ModelRecraft20B = "recraft20b"
)

// Style constants.
const (
	StyleRealisticImage      = "realistic_image"
	StyleDigitalIllustration = "digital_illustration"
	StyleVectorIllustration  = "vector_illustration"
	StyleIcon                = "icon"
)

var (
	ErrMissingPrompt = errors.New("recraft: missing prompt in workflow graph")
)

// Config configures the Recraft engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Style      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Recraft.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	style      string
	httpClient *http.Client
}

// New creates a Recraft engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("RECRAFT_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelRecraftV3
	}

	style := strings.TrimSpace(cfg.Style)

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		style:      style,
		httpClient: hc,
	}
}

// Execute generates an image via the Recraft API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("recraft: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "RECRAFT_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("recraft: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"prompt": prompt,
		"model":  e.model,
	}

	// Style: prefer graph option, fall back to engine default.
	style := e.style
	if s, ok := resolve.StringOption(g, "style"); ok && s != "" {
		style = s
	}
	if style != "" {
		payload["style"] = style
	}

	if size, ok := resolve.StringOption(g, "size"); ok && size != "" {
		payload["size"] = size
	}
	if n, ok := resolve.IntOption(g, "n"); ok && n > 0 {
		payload["n"] = n
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("recraft: marshal request: %w", err)
	}

	url := e.baseURL + "/v1/images/generations"
	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, url, apiKey, body, "recraft")
	if err != nil {
		return engine.Result{}, err
	}

	return extractResult(respBody)
}

func extractResult(body []byte) (engine.Result, error) {
	var resp struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("recraft: decode response: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return engine.Result{}, fmt.Errorf("recraft: response had no image URL")
	}
	return engine.Result{Value: resp.Data[0].URL, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the Recraft engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "RECRAFT_API_KEY", Description: "Recraft API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "RECRAFT_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Recraft models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelRecraftV3,
			ModelRecraft20B,
		},
	}
}
