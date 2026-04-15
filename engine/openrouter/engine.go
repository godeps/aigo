// Package openrouter implements an aigo engine for the OpenRouter API.
//
// OpenRouter routes all multimodal interactions through the chat completions
// endpoint (/v1/chat/completions). Image generation, TTS, and ASR are handled
// via modality flags and multimodal content blocks — there are no dedicated
// /v1/images/generations or /v1/audio/* endpoints.
package openrouter

import (
	"context"
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

const defaultBaseURL = "https://openrouter.ai/api"

// Model constants for known OpenRouter media models.
const (
	ModelGPT5Image        = "openai/gpt-5-image"
	ModelGPT5ImageMini    = "openai/gpt-5-image-mini"
	ModelGeminiFlashImage = "google/gemini-2.5-flash-image"
	ModelGemini3ProImage  = "google/gemini-3-pro-image-preview"
	ModelGPTAudio         = "openai/gpt-audio"
	ModelGPTAudioMini     = "openai/gpt-audio-mini"
)

// Sentinel errors.
var (
	ErrMissingAPIKey   = errors.New("openrouter: missing API key")
	ErrMissingPrompt   = errors.New("openrouter: prompt not found in workflow graph")
	ErrMissingAudioURL = errors.New("openrouter: audio URL not found in workflow graph")
	ErrMissingVoice    = errors.New("openrouter: TTS voice not found in workflow graph")
	ErrUnsupportedModel = errors.New("openrouter: unsupported model")
)

// Config configures the OpenRouter engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for OpenRouter.
type Engine struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// New creates an OpenRouter engine.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Engine{
		baseURL:    baseURL,
		model:      strings.TrimSpace(cfg.Model),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		httpClient: hc,
	}
}

type capability string

const (
	capImage capability = "image"
	capTTS   capability = "tts"
	capASR   capability = "asr"
)

type modelEntry struct {
	handler func(ctx context.Context, e *Engine, apiKey, model string, graph workflow.Graph) (string, error)
	kind    engine.OutputKind
	cap     capability
}

var modelTable = map[string]modelEntry{
	ModelGPT5Image:        {runImageGeneration, engine.OutputDataURI, capImage},
	ModelGPT5ImageMini:    {runImageGeneration, engine.OutputDataURI, capImage},
	ModelGeminiFlashImage: {runImageGeneration, engine.OutputDataURI, capImage},
	ModelGemini3ProImage:  {runImageGeneration, engine.OutputDataURI, capImage},
	ModelGPTAudio:         {runTTS, engine.OutputDataURI, capTTS},
	ModelGPTAudioMini:     {runTTS, engine.OutputDataURI, capTTS},
}

// asrTable maps the same audio models but for ASR usage.
// ASR uses the same models as TTS but with a different handler.
var asrTable = map[string]modelEntry{
	ModelGPTAudio:     {runASR, engine.OutputPlainText, capASR},
	ModelGPTAudioMini: {runASR, engine.OutputPlainText, capASR},
}

// Execute runs a workflow graph against the OpenRouter API.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	if err := graph.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("openrouter: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	// Detect ASR usage: if graph contains audio_url, use ASR handler for audio models.
	entry, ok := modelTable[e.model]
	if ok && entry.cap == capTTS {
		if _, hasAudio := resolve.StringOption(graph, "audio_url"); hasAudio {
			if asrEntry, asrOK := asrTable[e.model]; asrOK {
				entry = asrEntry
				ok = true
			}
		}
	}
	if !ok {
		entry, ok = asrTable[e.model]
	}
	if !ok {
		return engine.Result{}, fmt.Errorf("%w: %s", ErrUnsupportedModel, e.model)
	}

	value, err := entry.handler(ctx, e, apiKey, e.model, graph)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: value, Kind: engine.ClassifyOutput(value)}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	cap := engine.Capability{
		Models:       []string{e.model},
		SupportsSync: true,
	}
	if entry, ok := modelTable[e.model]; ok {
		switch entry.cap {
		case capImage:
			cap.MediaTypes = []string{"image"}
		case capTTS:
			cap.MediaTypes = []string{"audio"}
			cap.Voices = []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer"}
		}
	} else if _, ok := asrTable[e.model]; ok {
		cap.MediaTypes = []string{"audio"}
	}
	return cap
}

// ConfigSchema returns the configuration fields required by the OpenRouter engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "OPENROUTER_API_KEY", Description: "OpenRouter API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "OPENROUTER_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known OpenRouter media models grouped by capability.
func ModelsByCapability() map[string][]string {
	result := map[string][]string{}
	seen := map[string]bool{}
	for model, entry := range modelTable {
		result[string(entry.cap)] = append(result[string(entry.cap)], model)
		seen[model] = true
	}
	for model, entry := range asrTable {
		if !seen[model] {
			result[string(entry.cap)] = append(result[string(entry.cap)], model)
		} else {
			// Model exists in both tables — add under ASR cap too.
			result[string(entry.cap)] = append(result[string(entry.cap)], model)
		}
	}
	return result
}

// --- helpers ---

func extractPrompt(graph workflow.Graph) (string, error) {
	p, err := resolve.ExtractPrompt(graph)
	if err != nil {
		return "", fmt.Errorf("openrouter: %w", err)
	}
	if p == "" {
		return "", ErrMissingPrompt
	}
	return p, nil
}

func doRequest(ctx context.Context, hc *http.Client, method, url, apiKey string, body []byte) ([]byte, error) {
	return httpx.DoJSON(ctx, hc, method, url, apiKey, body, "openrouter")
}
