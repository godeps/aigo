// Package elevenlabs implements engine.Engine for the ElevenLabs TTS API.
//
// TTS uses POST /v1/text-to-speech/{voice_id} with JSON body.
// Auth: xi-api-key: {key}, env ELEVENLABS_API_KEY.
// Synchronous — returns audio binary, converted to data URI.
package elevenlabs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const defaultBaseURL = "https://api.elevenlabs.io"

// Model constants.
const (
	ModelMultilingualV2  = "eleven_multilingual_v2"
	ModelTurboV25        = "eleven_turbo_v2_5"
	ModelFlashV25        = "eleven_flash_v2_5"
	ModelMultilingualSTS = "eleven_multilingual_sts_v2"
)

var (
	ErrMissingAPIKey = errors.New("elevenlabs: missing API key (set Config.APIKey or ELEVENLABS_API_KEY)")
	ErrMissingText   = errors.New("elevenlabs: missing text for TTS (set prompt)")
	ErrMissingVoice  = errors.New("elevenlabs: missing voice ID")
)

// Config configures the ElevenLabs engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	VoiceID    string // Default voice ID if not specified in graph.
	HTTPClient *http.Client
}

// Engine implements engine.Engine for ElevenLabs.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	voiceID    string
	httpClient *http.Client
}

// New creates an ElevenLabs engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("ELEVENLABS_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelMultilingualV2
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    base,
		model:      model,
		voiceID:    strings.TrimSpace(cfg.VoiceID),
		httpClient: hc,
	}
}

// Execute performs TTS via the ElevenLabs API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("elevenlabs: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("ELEVENLABS_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	text, err := resolve.ExtractPrompt(g)
	if err != nil {
		// Fallback to text-specific key.
		if t, ok := resolve.StringOption(g, "text"); ok && strings.TrimSpace(t) != "" {
			text = strings.TrimSpace(t)
		}
	}
	if strings.TrimSpace(text) == "" {
		return engine.Result{}, ErrMissingText
	}

	voiceID := e.voiceID
	if v, ok := resolve.StringOption(g, "voice", "voice_id"); ok && strings.TrimSpace(v) != "" {
		voiceID = strings.TrimSpace(v)
	}
	if voiceID == "" {
		return engine.Result{}, ErrMissingVoice
	}

	payload := map[string]any{
		"text":     text,
		"model_id": e.model,
	}

	voiceSettings := map[string]any{
		"stability":        0.5,
		"similarity_boost": 0.75,
	}
	if v, ok := resolve.Float64Option(g, "stability"); ok {
		voiceSettings["stability"] = v
	}
	if v, ok := resolve.Float64Option(g, "similarity_boost"); ok {
		voiceSettings["similarity_boost"] = v
	}
	payload["voice_settings"] = voiceSettings

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("elevenlabs: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/text-to-speech/%s", e.baseURL, url.PathEscape(voiceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, fmt.Errorf("elevenlabs: build request: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return engine.Result{}, fmt.Errorf("elevenlabs: http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("elevenlabs: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "elevenlabs")
	}

	ct := resp.Header.Get("Content-Type")
	mime := "audio/mpeg"
	if strings.HasPrefix(ct, "audio/") {
		mime = ct
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(respBody))
	return engine.Result{Value: dataURI, Kind: engine.OutputDataURI}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"audio"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the ElevenLabs engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "ELEVENLABS_API_KEY", Description: "ElevenLabs API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "ELEVENLABS_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "voiceId", Label: "Default Voice ID", Type: "string", Description: "Default voice ID to use when not specified in the task"},
	}
}

// ModelsByCapability returns all known ElevenLabs models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"tts": {
			ModelMultilingualV2,
			ModelTurboV25,
			ModelFlashV25,
			ModelMultilingualSTS,
		},
	}
}
