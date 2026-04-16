// Package stability implements engine.Engine for the Stability AI API.
//
// Image generation uses POST /v2beta/stable-image/generate/{model}
// with multipart/form-data encoding.
// Auth: Authorization: Bearer {key}, env STABILITY_API_KEY.
package stability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const defaultBaseURL = "https://api.stability.ai"

// Model constants.
const (
	ModelSD35Large       = "sd3.5-large"
	ModelSD35LargeTurbo  = "sd3.5-large-turbo"
	ModelSD35Medium      = "sd3.5-medium"
	ModelSD3Turbo        = "sd3-turbo"
	ModelImageCore       = "stable-image-core"
	ModelImageUltra      = "stable-image-ultra"
)

var (
	ErrMissingAPIKey = errors.New("stability: missing API key (set Config.APIKey or STABILITY_API_KEY)")
	ErrMissingPrompt = errors.New("stability: missing prompt in workflow graph")
)

// Config configures the Stability AI engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Stability AI.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates a Stability AI engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("STABILITY_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelSD35Large
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		httpClient: hc,
	}
}

// Execute generates an image via the Stability AI API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("stability: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("STABILITY_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("stability: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	negPrompt, _ := resolve.StringOption(g, "negative_prompt")
	aspectRatio := "1:1"
	if v, ok := resolve.StringOption(g, "aspect_ratio"); ok && v != "" {
		aspectRatio = v
	}

	// Build multipart form.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("prompt", prompt)
	if negPrompt != "" {
		_ = w.WriteField("negative_prompt", negPrompt)
	}
	_ = w.WriteField("aspect_ratio", aspectRatio)
	_ = w.WriteField("model", e.model)
	_ = w.WriteField("output_format", "png")
	if err := w.Close(); err != nil {
		return engine.Result{}, fmt.Errorf("stability: close form: %w", err)
	}

	url := e.baseURL + "/v2beta/stable-image/generate/sd3"
	if e.model == ModelImageCore {
		url = e.baseURL + "/v2beta/stable-image/generate/core"
	} else if e.model == ModelImageUltra {
		url = e.baseURL + "/v2beta/stable-image/generate/ultra"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return engine.Result{}, fmt.Errorf("stability: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return engine.Result{}, fmt.Errorf("stability: http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("stability: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "stability")
	}

	return extractResult(respBody)
}

// extractResult parses the JSON response with base64-encoded image.
func extractResult(body []byte) (engine.Result, error) {
	// Response: {"image": "<base64>", "finish_reason": "SUCCESS", "seed": 123}
	// Try to find the image field.
	type stableResp struct {
		Image        string `json:"image"`
		FinishReason string `json:"finish_reason"`
	}
	var resp stableResp

	if err := json.Unmarshal(body, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("stability: decode response: %w", err)
	}

	if resp.Image == "" {
		return engine.Result{}, fmt.Errorf("stability: response had no image data")
	}

	// URL response takes priority over base64.
	if strings.HasPrefix(resp.Image, "http") {
		return engine.Result{Value: resp.Image, Kind: engine.OutputURL}, nil
	}

	dataURI := "data:image/png;base64," + resp.Image
	return engine.Result{Value: dataURI, Kind: engine.OutputDataURI}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the Stability AI engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "STABILITY_API_KEY", Description: "Stability AI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "STABILITY_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Stability AI models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelSD35Large,
			ModelSD35LargeTurbo,
			ModelSD35Medium,
			ModelSD3Turbo,
			ModelImageCore,
			ModelImageUltra,
		},
	}
}

