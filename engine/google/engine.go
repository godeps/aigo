// Package google implements engine.Engine for Google Imagen and Veo APIs.
//
// Image generation is synchronous via the Gemini API:
// POST /v1beta/models/{model}:predict with API key as query param.
// Auth: ?key={api_key}, env GOOGLE_API_KEY.
package google

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

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Model constants.
const (
	ModelImagen3Generate002 = "imagen-3.0-generate-002"
	ModelImagen3Generate001 = "imagen-3.0-generate-001"
)

var ErrMissingPrompt = errors.New("google: missing prompt in workflow graph")

// Config configures the Google Imagen engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Google Imagen.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates a Google Imagen engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("GOOGLE_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelImagen3Generate002
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		httpClient: hc,
	}
}

// predictRequest is the request body for the Imagen predict endpoint.
type predictRequest struct {
	Instances  []instance  `json:"instances"`
	Parameters *parameters `json:"parameters,omitempty"`
}

type instance struct {
	Prompt string `json:"prompt"`
}

type parameters struct {
	SampleCount int    `json:"sampleCount,omitempty"`
	AspectRatio string `json:"aspectRatio,omitempty"`
	Seed        int    `json:"seed,omitempty"`
	HasSeed     bool   `json:"-"`
}

func (p parameters) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if p.SampleCount > 0 {
		m["sampleCount"] = p.SampleCount
	}
	if p.AspectRatio != "" {
		m["aspectRatio"] = p.AspectRatio
	}
	if p.HasSeed {
		m["seed"] = p.Seed
	}
	return json.Marshal(m)
}

// predictResponse is the response from the Imagen predict endpoint.
type predictResponse struct {
	Predictions []prediction `json:"predictions"`
}

type prediction struct {
	BytesBase64Encoded string `json:"bytesBase64Encoded"`
	MimeType           string `json:"mimeType"`
}

// Execute generates an image via the Google Imagen API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("google: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "GOOGLE_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("google: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	params := &parameters{
		SampleCount: 1,
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio", "aspectRatio"); ok && ar != "" {
		params.AspectRatio = ar
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		params.Seed = seed
		params.HasSeed = true
	}
	if count, ok := resolve.IntOption(g, "sample_count", "sampleCount"); ok && count > 0 {
		params.SampleCount = count
	}

	reqBody := predictRequest{
		Instances:  []instance{{Prompt: prompt}},
		Parameters: params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return engine.Result{}, fmt.Errorf("google: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:predict?key=%s", e.baseURL, e.model, apiKey)
	respBody, err := e.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return engine.Result{}, err
	}

	var resp predictResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("google: decode response: %w", err)
	}
	if len(resp.Predictions) == 0 {
		return engine.Result{}, fmt.Errorf("google: no predictions in response")
	}

	mime := resp.Predictions[0].MimeType
	if mime == "" {
		mime = "image/png"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, resp.Predictions[0].BytesBase64Encoded)
	return engine.Result{Value: dataURI, Kind: engine.OutputDataURI}, nil
}

// doRequest sends a JSON request with API key as query param (not Bearer).
func (e *Engine) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("google: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "google")
	}
	return out, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		SupportsSync: true,
		SupportsPoll: false,
	}
}

// ConfigSchema returns the configuration fields required by the Google Imagen engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "GOOGLE_API_KEY", Description: "Google API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "GOOGLE_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Google Imagen models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {
			ModelImagen3Generate002,
			ModelImagen3Generate001,
		},
	}
}
