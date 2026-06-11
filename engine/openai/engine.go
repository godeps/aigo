package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "dall-e-3"
	defaultSize    = "1024x1024"
)

var (
	ErrMissingPrompt       = errors.New("openai: prompt not found in workflow graph")
	ErrRemoteMediaDisabled = errors.New("openai: remote media fetch is disabled by configuration")
)

// Config configures the OpenAI image engine.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Quality string
	Style   string
	// gpt-image-* 专属可选参数
	Background        string // transparent | opaque | auto
	OutputFormat      string // png | jpeg | webp
	Moderation        string // low | auto
	OutputCompression int    // 0-100, 仅 jpeg/webp 生效
	// AllowRemoteImageFetch 控制 image_url 是否允许 HTTP GET（默认 true）。
	DisableRemoteMediaFetch bool
	HTTPClient              *http.Client
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
	apiKey                string
	baseURL               string
	model                 string
	quality               string
	style                 string
	background            string
	outputFormat          string
	moderation            string
	outputCompression     int
	httpClient            *http.Client
	allowRemoteMediaFetch bool
}

// New creates an OpenAI engine instance.
func New(cfg Config) *Engine {
	httpClient := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	quality := cfg.Quality
	if quality == "" && !isGPTImageModel(model) {
		quality = "standard"
	}

	return &Engine{
		apiKey:                cfg.APIKey,
		baseURL:               baseURL,
		model:                 model,
		quality:               quality,
		style:                 cfg.Style,
		background:            strings.TrimSpace(cfg.Background),
		outputFormat:          strings.ToLower(strings.TrimSpace(cfg.OutputFormat)),
		moderation:            strings.TrimSpace(cfg.Moderation),
		outputCompression:     cfg.OutputCompression,
		httpClient:            httpClient,
		allowRemoteMediaFetch: !cfg.DisableRemoteMediaFetch,
	}
}

// isGPTImageModel reports whether the model belongs to the gpt-image-* family,
// which has a different request/response contract than DALL-E models.
func isGPTImageModel(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "gpt-image-")
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
		prompt, ok, err := resolve.ResolveNodeString(graph, ref.ID, map[string]bool{})
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
			req.Size = resolve.NormalizeOpenAIImageSize(width, height)
			break
		}
	}
	for _, ref := range graph.FindByClassType("ImageOptions") {
		if quality, ok := ref.Node.StringInput("quality"); ok && strings.TrimSpace(quality) != "" {
			req.Quality = strings.TrimSpace(quality)
			break
		}
	}

	if req.Prompt == "" {
		return Request{}, ErrMissingPrompt
	}

	return req, nil
}

// Execute compiles the workflow and calls the OpenAI images API.
// If the graph carries an image source (image_b64/image_path/image_url/LoadImage),
// it dispatches to /v1/images/edits with multipart upload; otherwise it calls
// /v1/images/generations with JSON.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	req, err := e.Compile(graph)
	if err != nil {
		return engine.Result{}, err
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "OPENAI_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	if hasImageSource(graph) {
		return e.executeEdits(ctx, apiKey, req, graph)
	}

	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"size":   req.Size,
		"n":      1,
	}
	if isGPTImageModel(req.Model) {
		// gpt-image-* always returns b64_json; response_format and style
		// parameters are not accepted by the API.
		if req.Quality != "" {
			payload["quality"] = req.Quality
		}
		e.applyGPTImageOptions(payload)
	} else {
		payload["response_format"] = "url"
		if req.Quality != "" {
			payload["quality"] = req.Quality
		}
		if req.Style != "" {
			payload["style"] = req.Style
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: create image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "openai")
	}

	var decoded struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return engine.Result{}, fmt.Errorf("openai: decode response: %w", err)
	}

	if len(decoded.Data) == 0 {
		return engine.Result{}, errors.New("openai: response did not contain generated images")
	}

	if decoded.Data[0].URL != "" {
		return engine.Result{Value: decoded.Data[0].URL, Kind: engine.OutputURL}, nil
	}
	if decoded.Data[0].B64JSON != "" {
		return engine.Result{Value: "data:" + e.b64ImageMIME() + ";base64," + decoded.Data[0].B64JSON, Kind: engine.OutputDataURI}, nil
	}

	return engine.Result{}, errors.New("openai: response did not contain a usable image result")
}

// applyGPTImageOptions writes gpt-image-* specific fields into the JSON payload,
// omitting any field that is empty/zero.
func (e *Engine) applyGPTImageOptions(payload map[string]any) {
	if e.background != "" {
		payload["background"] = e.background
	}
	if e.outputFormat != "" {
		payload["output_format"] = e.outputFormat
	}
	if e.moderation != "" {
		payload["moderation"] = e.moderation
	}
	if e.outputCompression > 0 {
		payload["output_compression"] = e.outputCompression
	}
}

// b64ImageMIME returns the MIME for b64_json data URIs based on OutputFormat.
// Defaults to image/png.
func (e *Engine) b64ImageMIME() string {
	return imageMIMEFromOutputFormat(e.outputFormat)
}

// imageMIMEFromOutputFormat maps OpenAI image `output_format` values to MIME types.
func imageMIMEFromOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "png", "":
		return "image/png"
	}
	return "image/png"
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	sizes := []string{"1024x1024", "1024x1792", "1792x1024"}
	if isGPTImageModel(e.model) {
		sizes = []string{"1024x1024", "1024x1536", "1536x1024"}
	}
	return engine.Capability{
		MediaTypes:   []string{"image"},
		Models:       []string{e.model},
		Sizes:        sizes,
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the OpenAI engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "OPENAI_API_KEY", Description: "OpenAI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "OPENAI_BASE_URL", Description: "Custom API base URL (optional)"},
		{Key: "model", Label: "Model", Type: "string", Description: "Image model, e.g. gpt-image-2, dall-e-3, or dall-e-2.", Default: defaultModel},
		{Key: "quality", Label: "Quality", Type: "string", Description: "Image quality tier, e.g. low, medium, high, auto, standard, or hd."},
		{Key: "style", Label: "Style", Type: "string", Description: "DALL-E style, e.g. vivid or natural."},
		{Key: "background", Label: "Background", Type: "string", Description: "gpt-image-* background mode: transparent, opaque, or auto."},
		{Key: "output_format", Label: "Output Format", Type: "string", Description: "gpt-image-* output format: png, jpeg, or webp."},
		{Key: "moderation", Label: "Moderation", Type: "string", Description: "gpt-image-* moderation mode, e.g. low or auto."},
		{Key: "output_compression", Label: "Output Compression", Type: "number", Description: "gpt-image-* JPEG/WebP compression from 0 to 100."},
	}
}

// ModelsByCapability returns all known OpenAI models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {"gpt-image-2", "dall-e-3", "dall-e-2"},
	}
}
