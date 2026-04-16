// Package hedra implements engine.Engine for the Hedra character video API.
//
// Character video generation is async: POST /v1/characters creates a project,
// then GET /v1/projects/{id} polls for completion.
// Auth: X-API-KEY header, env HEDRA_API_KEY.
package hedra

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
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL      = "https://mercury.dev.dream-ai.com/api"
	defaultPollInterval = 5 * time.Second
)

var (
	ErrMissingAPIKey = errors.New("hedra: missing API key (set Config.APIKey or HEDRA_API_KEY)")
)

// Config configures the Hedra engine.
type Config struct {
	APIKey            string
	BaseURL           string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine implements engine.Engine for Hedra character video generation.
type Engine struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	waitVideo    bool
	pollInterval time.Duration
}

// New creates a Hedra engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("HEDRA_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      base,
		httpClient:   hc,
		waitVideo:    cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// resolveAPIKey returns the configured or environment API key.
func (e *Engine) resolveAPIKey() (string, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("HEDRA_API_KEY")
	}
	if apiKey == "" {
		return "", ErrMissingAPIKey
	}
	return apiKey, nil
}

// Execute generates a character video via the Hedra API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("hedra: validate graph: %w", err)
	}

	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return engine.Result{}, err
	}

	// Extract text prompt (used for TTS or video description).
	text, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("hedra: %w", err)
	}

	// Extract voice ID for TTS.
	voiceID, _ := resolve.StringOption(g, "voice_id", "voiceId")

	// Extract audio URL (pre-uploaded audio asset).
	audioURL := ""
	for _, ref := range g.FindByClassType("LoadAudio") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			audioURL = u
			break
		}
	}

	// Extract image URL (portrait/avatar image).
	imageURL := ""
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			imageURL = u
			break
		}
	}

	// Build the generation request payload.
	payload := map[string]any{}

	if text != "" {
		payload["text"] = text
	}
	if voiceID != "" {
		payload["voiceId"] = voiceID
	}
	if audioURL != "" {
		payload["voiceUrl"] = audioURL
		payload["audioSource"] = "audio"
	} else if voiceID != "" {
		payload["audioSource"] = "tts"
	}
	if imageURL != "" {
		payload["avatarImage"] = imageURL
	}

	if ar, ok := resolve.StringOption(g, "aspect_ratio", "ratio"); ok && ar != "" {
		payload["aspectRatio"] = ar
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("hedra: marshal request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+"/v1/characters", apiKey, body)
	if err != nil {
		return engine.Result{}, err
	}

	var created struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return engine.Result{}, fmt.Errorf("hedra: decode create: %w", err)
	}
	if created.JobID == "" {
		return engine.Result{}, fmt.Errorf("hedra: create returned empty jobId")
	}

	if !e.waitVideo {
		return engine.Result{Value: created.JobID, Kind: engine.OutputPlainText}, nil
	}

	videoURL, err := e.poll(ctx, apiKey, created.JobID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: videoURL, Kind: engine.OutputURL}, nil
}

func (e *Engine) poll(ctx context.Context, apiKey, projectID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/v1/projects/"+projectID, nil)
		if err != nil {
			return "", false, fmt.Errorf("hedra: build poll: %w", err)
		}
		req.Header.Set("X-API-KEY", apiKey)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("hedra: poll request: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("hedra: read poll: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "hedra")
		}

		var project struct {
			Status   string `json:"status"`
			VideoURL string `json:"videoUrl"`
			Error    string `json:"errorMessage"`
		}
		if err := json.Unmarshal(body, &project); err != nil {
			return "", false, fmt.Errorf("hedra: decode poll: %w", err)
		}

		switch strings.ToLower(project.Status) {
		case "completed", "complete":
			if project.VideoURL == "" {
				return "", true, fmt.Errorf("hedra: completed but no video URL")
			}
			return project.VideoURL, true, nil
		case "failed", "error":
			msg := "failed"
			if project.Error != "" {
				msg = project.Error
			}
			return "", true, fmt.Errorf("hedra: project failed: %s", msg)
		default:
			return "", false, nil
		}
	})
}

// doRequest sends an authenticated JSON request using X-API-KEY header.
func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("hedra: build request: %w", err)
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hedra: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hedra: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "hedra")
	}
	return out, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted project.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := e.resolveAPIKey()
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
		MediaTypes:   []string{"video"},
		SupportsPoll: e.waitVideo,
		SupportsSync: !e.waitVideo,
	}
}

// ConfigSchema returns the configuration fields required by the Hedra engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "HEDRA_API_KEY", Description: "Hedra API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "HEDRA_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
	}
}

// ModelsByCapability returns all known Hedra models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"video": {"hedra-character-v1"},
	}
}
