// Package gemini implements engine.Engine for Google Gemini multi-modal understanding.
//
// Gemini supports text generation with optional image/video inputs for analysis.
// Auth: API key as query param ?key={api_key}, env GEMINI_API_KEY or GOOGLE_API_KEY.
//
// Endpoint: POST /models/{model}:generateContent
// Default model: gemini-2.0-flash
package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

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
	BaseURL    string // default: https://generativelanguage.googleapis.com/v1beta
	Model      string // default: gemini-2.0-flash
	HTTPClient *http.Client
}

// Engine implements engine.Engine for Google Gemini multi-modal understanding.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates a Gemini engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("GEMINI_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelGemini20Flash
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		httpClient: hc,
	}
}

// --- request types ---

type generateRequest struct {
	Contents []content `json:"contents"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string    `json:"text,omitempty"`
	InlineData *blobData `json:"inline_data,omitempty"`
	FileData   *fileData `json:"file_data,omitempty"`
}

type blobData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type fileData struct {
	MimeType string `json:"mime_type"`
	FileURI  string `json:"file_uri"`
}

// --- response types ---

type generateResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content candidateContent `json:"content"`
}

type candidateContent struct {
	Parts []responsePart `json:"parts"`
}

type responsePart struct {
	Text string `json:"text"`
}

// Execute sends a multi-modal request to the Gemini API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("gemini: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "GEMINI_API_KEY", "GOOGLE_API_KEY")
	if err != nil {
		return engine.Result{}, fmt.Errorf("gemini: %w", err)
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gemini: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	parts := []part{{Text: prompt}}

	// Collect image inputs from LoadImage nodes.
	for _, ref := range g.FindByClassType("LoadImage") {
		p, ok := buildMediaPart(ref, "image/jpeg")
		if ok {
			parts = append(parts, p)
		}
	}

	// Collect video inputs from LoadVideo nodes.
	for _, ref := range g.FindByClassType("LoadVideo") {
		p, ok := buildMediaPart(ref, "video/mp4")
		if ok {
			parts = append(parts, p)
		}
	}

	reqBody := generateRequest{
		Contents: []content{{Parts: parts}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return engine.Result{}, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", e.baseURL, e.model, apiKey)
	respBody, err := e.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return engine.Result{}, err
	}

	var resp generateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return engine.Result{}, fmt.Errorf("gemini: no candidates in response")
	}

	text := extractText(resp.Candidates[0])
	return engine.Result{Value: text, Kind: engine.OutputPlainText}, nil
}

// buildMediaPart creates a part from a LoadImage or LoadVideo node reference.
// It checks for a "url" input (file_data) or "data"+"mime_type" inputs (inline_data).
func buildMediaPart(ref workflow.NodeRef, defaultMime string) (part, bool) {
	if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
		mime := defaultMime
		if m, ok := ref.Node.Inputs["mime_type"].(string); ok && m != "" {
			mime = m
		}
		return part{FileData: &fileData{MimeType: mime, FileURI: u}}, true
	}
	if d, ok := ref.Node.Inputs["data"].(string); ok && d != "" {
		mime := defaultMime
		if m, ok := ref.Node.Inputs["mime_type"].(string); ok && m != "" {
			mime = m
		}
		return part{InlineData: &blobData{MimeType: mime, Data: d}}, true
	}
	return part{}, false
}

// extractText concatenates all text parts from a candidate.
func extractText(c candidate) string {
	var sb strings.Builder
	for _, p := range c.Content.Parts {
		sb.WriteString(p.Text)
	}
	return sb.String()
}

// doRequest sends a JSON request with API key as query param (not Bearer).
func (e *Engine) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "gemini")
	}
	return out, nil
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
	}
}
