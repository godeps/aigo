// Package suno implements engine.Engine for Suno music generation.
//
// Music generation is async: POST /api/generate creates a task,
// GET /api/feed/{id} polls for completion.
// Auth: Authorization: Bearer {key}, env SUNO_API_KEY.
// Uses the suno-api compatible gateway interface.
package suno

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

// No defaultBaseURL — Suno requires a user-provided gateway URL.
const defaultPollInterval = 5 * time.Second

// Model constants.
const (
	ModelChirpV4  = "chirp-v4"
	ModelChirpV35 = "chirp-v3.5"
)

var (
	ErrMissingBaseURL = errors.New("suno: missing base URL (set Config.BaseURL or SUNO_BASE_URL)")
	ErrMissingPrompt  = errors.New("suno: missing prompt in workflow graph")
)

// Config configures the Suno engine.
type Config struct {
	APIKey            string
	BaseURL           string // Required — Suno API gateway URL.
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
	OnProgress        epoll.OnProgress
}

// Engine implements engine.Engine for Suno.
type Engine struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	waitMusic    bool
	pollInterval time.Duration
	onProgress   epoll.OnProgress
}

// New creates a Suno engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("SUNO_BASE_URL")), "/")
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelChirpV4
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		model:        model,
		httpClient:   hc,
		waitMusic:    cfg.WaitForCompletion,
		pollInterval: poll,
		onProgress:   cfg.OnProgress,
	}
}

// Execute generates music via the Suno API gateway.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("suno: validate graph: %w", err)
	}
	if e.baseURL == "" {
		return engine.Result{}, ErrMissingBaseURL
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "SUNO_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("suno: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	payload := map[string]any{
		"prompt": prompt,
		"model":  e.model,
	}

	if lyrics, ok := resolve.StringOption(g, "lyrics"); ok && lyrics != "" {
		payload["lyrics"] = lyrics
	}
	if instrumental, ok := resolve.BoolOption(g, "is_instrumental"); ok {
		payload["make_instrumental"] = instrumental
	}
	if title, ok := resolve.StringOption(g, "title"); ok && title != "" {
		payload["title"] = title
	}
	if tags, ok := resolve.StringOption(g, "tags"); ok && tags != "" {
		payload["tags"] = tags
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("suno: marshal request: %w", err)
	}

	respBody, err := httpx.DoJSON(ctx, e.httpClient, http.MethodPost,
		e.baseURL+"/api/generate", apiKey, body, "suno")
	if err != nil {
		return engine.Result{}, err
	}

	// Response is an array of generated clips.
	var clips []struct {
		ID       string `json:"id"`
		AudioURL string `json:"audio_url"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &clips); err != nil {
		return engine.Result{}, fmt.Errorf("suno: decode create: %w", err)
	}
	if len(clips) == 0 {
		return engine.Result{}, fmt.Errorf("suno: no clips returned")
	}

	// If first clip already has URL, return it.
	if clips[0].AudioURL != "" {
		return engine.Result{Value: clips[0].AudioURL, Kind: engine.OutputURL}, nil
	}

	if !e.waitMusic {
		return engine.Result{Value: clips[0].ID, Kind: engine.OutputPlainText}, nil
	}

	audioURL, err := e.poll(ctx, apiKey, clips[0].ID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: audioURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, clipID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval, OnProgress: e.onProgress}, func(ctx context.Context) (string, bool, error) {
		url := fmt.Sprintf("%s/api/feed/%s", e.baseURL, clipID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, fmt.Errorf("suno: build poll: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("suno: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("suno: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "suno")
		}

		// Feed returns an array; we want the first clip.
		var clips []struct {
			ID       string `json:"id"`
			AudioURL string `json:"audio_url"`
			Status   string `json:"status"`
		}
		if err := json.Unmarshal(body, &clips); err != nil {
			return "", false, fmt.Errorf("suno: decode poll: %w", err)
		}
		if len(clips) == 0 {
			return "", false, fmt.Errorf("suno: empty feed response")
		}

		clip := clips[0]
		switch strings.ToLower(clip.Status) {
		case "complete":
			if clip.AudioURL == "" {
				return "", true, fmt.Errorf("suno: complete but no audio URL")
			}
			return clip.AudioURL, true, nil
		case "error":
			return "", true, fmt.Errorf("suno: generation failed")
		default:
			return "", false, nil
		}
	})
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	if e.baseURL == "" {
		return engine.Result{}, ErrMissingBaseURL
	}
	apiKey, err := engine.ResolveKey(e.apiKey, "SUNO_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}
	url, err := e.poll(ctx, apiKey, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"audio"},
		Models:       []string{e.model},
		SupportsPoll: e.waitMusic,
		SupportsSync: !e.waitMusic,
	}
}

// ConfigSchema returns the configuration fields required by the Suno engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "SUNO_API_KEY", Description: "Suno API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", Required: true, EnvVar: "SUNO_BASE_URL", Description: "Suno API gateway URL"},
	}
}

// ModelsByCapability returns all known Suno models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"music": {
			ModelChirpV4,
			ModelChirpV35,
		},
	}
}
