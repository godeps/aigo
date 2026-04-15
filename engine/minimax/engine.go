// Package minimax implements engine.Engine for MiniMax music generation.
//
// The music generation API is synchronous — no polling is needed.
// Endpoint: POST {baseURL}/v1/music_generation
// Auth: Authorization: Bearer {api_key}
//
// Supported models: music-2.6, music-cover, music-2.6-free, music-cover-free.
package minimax

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

const defaultBaseURL = "https://api.minimaxi.com"

// Model constants for MiniMax music generation.
const (
	ModelMusic26       = "music-2.6"
	ModelMusicCover    = "music-cover"
	ModelMusic26Free   = "music-2.6-free"
	ModelMusicCoverFree = "music-cover-free"
)

// Sentinel errors.
var (
	ErrMissingAPIKey    = errors.New("minimax: missing API key (set Config.APIKey or MINIMAX_API_KEY)")
	ErrMissingPrompt    = errors.New("minimax: missing prompt in workflow graph")
	ErrUnsupportedModel = errors.New("minimax: unsupported model")
)

// allModels lists all supported music generation models.
var allModels = map[string]bool{
	ModelMusic26:        true,
	ModelMusicCover:     true,
	ModelMusic26Free:    true,
	ModelMusicCoverFree: true,
}

// Config configures the MiniMax engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// Engine implements engine.Engine for MiniMax music generation.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates a MiniMax engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("MINIMAX_BASE_URL")), "/")
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Engine{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    baseURL,
		model:      strings.TrimSpace(cfg.Model),
		httpClient: hc,
	}
}

// Execute runs music generation against the MiniMax API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("minimax: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("MINIMAX_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	if !allModels[e.model] {
		return engine.Result{}, fmt.Errorf("%w: %s", ErrUnsupportedModel, e.model)
	}

	value, err := runMusic(ctx, e, apiKey, g)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: value, Kind: engine.ClassifyOutput(value)}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"audio"},
		Models:       []string{e.model},
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the MiniMax engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "MINIMAX_API_KEY", Description: "MiniMax API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "MINIMAX_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known MiniMax models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"music": {
			ModelMusic26,
			ModelMusicCover,
			ModelMusic26Free,
			ModelMusicCoverFree,
		},
	}
}

// runMusic calls the MiniMax music generation API.
//
// Request:
//
//	POST /v1/music_generation
//	{
//	  "model": "music-2.6",
//	  "prompt": "独立民谣,忧郁,内省",
//	  "lyrics": "[verse]\n...",
//	  "stream": false,
//	  "output_format": "url",
//	  "is_instrumental": false,
//	  "audio_setting": {"sample_rate": 44100, "bitrate": 256000, "format": "mp3"}
//	}
//
// Response:
//
//	{
//	  "data": {"status": 2, "audio": "<url_or_hex>"},
//	  "base_resp": {"status_code": 0, "status_msg": "success"},
//	  "extra_info": {"music_duration": 25364}
//	}
func runMusic(ctx context.Context, e *Engine, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return "", fmt.Errorf("minimax: %w", err)
	}
	if prompt == "" {
		return "", ErrMissingPrompt
	}

	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
		"stream": false,
	}

	if lyrics, ok := resolve.StringOption(g, "lyrics"); ok {
		payload["lyrics"] = lyrics
	}

	if instrumental, ok := resolve.BoolOption(g, "is_instrumental"); ok {
		payload["is_instrumental"] = instrumental
	}

	if optimizeLyrics, ok := resolve.BoolOption(g, "lyrics_optimizer"); ok {
		payload["lyrics_optimizer"] = optimizeLyrics
	}

	outputFormat := "url"
	if v, ok := resolve.StringOption(g, "output_format"); ok && v != "" {
		outputFormat = v
	}
	payload["output_format"] = outputFormat

	// Build audio_setting if any audio options are provided.
	audioSetting := map[string]any{}
	if v, ok := resolve.IntOption(g, "sample_rate"); ok && v > 0 {
		audioSetting["sample_rate"] = v
	}
	if v, ok := resolve.IntOption(g, "bitrate"); ok && v > 0 {
		audioSetting["bitrate"] = v
	}
	if v, ok := resolve.StringOption(g, "format"); ok && v != "" {
		audioSetting["format"] = v
	}
	if len(audioSetting) > 0 {
		payload["audio_setting"] = audioSetting
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("minimax: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost, e.baseURL+"/v1/music_generation", apiKey, body, "minimax")
	if err != nil {
		return "", err
	}

	return extractMusicResult(respBody)
}

// musicResponse represents the MiniMax music generation API response.
type musicResponse struct {
	Data struct {
		Status int    `json:"status"`
		Audio  string `json:"audio"`
	} `json:"data"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	ExtraInfo struct {
		MusicDuration int `json:"music_duration"`
	} `json:"extra_info"`
}

// extractMusicResult parses the API response and returns the audio output.
func extractMusicResult(body []byte) (string, error) {
	var resp musicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("minimax: decode response: %w", err)
	}
	if resp.BaseResp.StatusCode != 0 {
		return "", fmt.Errorf("minimax: API error %d: %s", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
	}
	if resp.Data.Audio == "" {
		return "", fmt.Errorf("minimax: response had no audio data")
	}
	return resp.Data.Audio, nil
}
