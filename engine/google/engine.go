// Package google implements engine.Engine for Google Imagen and Veo APIs.
//
// Image generation via the official Google GenAI SDK.
// Auth: API key via SDK, env GOOGLE_API_KEY.
package google

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
	client *genai.Client
	model  string
}

// New creates a Google Imagen engine instance.
func New(cfg Config) (*Engine, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("google: missing API key (set Config.APIKey or GOOGLE_API_KEY)")
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelImagen3Generate002
	}

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("GOOGLE_BASE_URL")), "/")
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
		return nil, fmt.Errorf("google: create client: %w", err)
	}

	return &Engine{
		client: client,
		model:  model,
	}, nil
}

// Execute generates an image via the Google Imagen API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("google: validate graph: %w", err)
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("google: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	imgCfg := &genai.GenerateImagesConfig{
		NumberOfImages: 1,
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio", "aspectRatio"); ok && ar != "" {
		imgCfg.AspectRatio = ar
	}
	if seed, ok := resolve.IntOption(g, "seed"); ok {
		s := int32(seed)
		imgCfg.Seed = &s
	}
	if count, ok := resolve.IntOption(g, "sample_count", "sampleCount"); ok && count > 0 {
		imgCfg.NumberOfImages = int32(count)
	}

	resp, err := e.client.Models.GenerateImages(ctx, e.model, prompt, imgCfg)
	if err != nil {
		return engine.Result{}, fmt.Errorf("google: generate images: %w", err)
	}

	if resp == nil || len(resp.GeneratedImages) == 0 {
		return engine.Result{}, fmt.Errorf("google: no images in response")
	}

	img := resp.GeneratedImages[0].Image
	mime := img.MIMEType
	if mime == "" {
		mime = "image/png"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(img.ImageBytes))
	return engine.Result{Value: dataURI, Kind: engine.OutputDataURI}, nil
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
