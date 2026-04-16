// Package ideogram implements engine.Engine for the Ideogram API.
//
// Image generation uses POST /generate with JSON body.
// Auth: Api-Key: {key}, env IDEOGRAM_API_KEY.
// Synchronous — returns image URL directly.
package ideogram

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

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const defaultBaseURL = "https://api.ideogram.ai"

// Model constants.
const (
	ModelV2A      = "V_2A"
	ModelV2ATurbo = "V_2A_TURBO"
	ModelV2       = "V_2"
	ModelV2Turbo  = "V_2_TURBO"
)

var (
	ErrMissingPrompt = errors.New("ideogram: missing prompt in workflow graph")
)

// Config configures the Ideogram engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Ideogram.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates an Ideogram engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("IDEOGRAM_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelV2A
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		httpClient: hc,
	}
}

// Execute generates an image via the Ideogram API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("ideogram: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "IDEOGRAM_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ideogram: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	imageReq := map[string]any{
		"prompt": prompt,
		"model":  e.model,
	}
	if neg, ok := resolve.StringOption(g, "negative_prompt"); ok && neg != "" {
		imageReq["negative_prompt"] = neg
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio"); ok && ar != "" {
		imageReq["aspect_ratio"] = ar
	}
	if style, ok := resolve.StringOption(g, "style_type"); ok && style != "" {
		imageReq["style_type"] = style
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		imageReq["seed"] = seed
	}

	payload := map[string]any{
		"image_request": imageReq,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ideogram: marshal request: %w", err)
	}

	respBody, err := e.doRequest(ctx, e.baseURL+"/generate", apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	return extractResult(respBody)
}

func extractResult(body []byte) (engine.Result, error) {
	var resp struct {
		Data []struct {
			URL    string `json:"url"`
			Prompt string `json:"prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("ideogram: decode response: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return engine.Result{}, fmt.Errorf("ideogram: response had no image URL")
	}
	return engine.Result{Value: resp.Data[0].URL, Kind: engine.OutputURL}, nil
}

// doRequest uses Api-Key header (not Bearer); cannot use httpx.DoJSON.
func (e *Engine) doRequest(ctx context.Context, url, apiKey string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ideogram: build request: %w", err)
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ideogram: http post: %w", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ideogram: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "ideogram")
	}
	return out, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the Ideogram engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "IDEOGRAM_API_KEY", Description: "Ideogram API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "IDEOGRAM_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Ideogram models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelV2A,
			ModelV2ATurbo,
			ModelV2,
			ModelV2Turbo,
		},
	}
}
