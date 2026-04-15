// Package volcvoice implements engine.Engine for Volcengine Speech
// (火山语音 / openspeech.bytedance.com) TTS and ASR.
//
// TTS uses POST /api/v1/tts with voice_type and encoding parameters.
// ASR uses POST /api/v1/asr with audio data in base64.
//
// Authentication requires an AppID and Access Token, which can be
// set via Config or environment variables VOLC_SPEECH_APPID and
// VOLC_SPEECH_ACCESS_TOKEN.
package volcvoice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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

const defaultBaseURL = "https://openspeech.bytedance.com"

// Model constants for TTS and ASR.
const (
	// TTS models (voice_type values).
	ModelTTSMega    = "volcano_mega"    // 大模型语音合成
	ModelTTSIcl     = "volcano_icl"     // 复刻音色
	ModelTTSDefault = "volcano_tts"     // 通用语音合成
	ModelASR        = "volcano_asr"     // 语音识别
	ModelASRLarge   = "volcano_asr_pro" // 语音识别 Pro
)

// Default TTS clusters.
const (
	clusterTTS = "volcano_tts"
	clusterASR = "volcengine_streaming_common"
)

var (
	ErrMissingAppID       = errors.New("volcvoice: missing AppID (set Config.AppID or VOLC_SPEECH_APPID)")
	ErrMissingAccessToken = errors.New("volcvoice: missing AccessToken (set Config.AccessToken or VOLC_SPEECH_ACCESS_TOKEN)")
	ErrMissingText        = errors.New("volcvoice: missing text for TTS (set prompt)")
	ErrMissingAudioURL    = errors.New("volcvoice: missing audio_url for ASR")
	ErrMissingVoice       = errors.New("volcvoice: missing voice (set voice option)")
)

// ttsModels lists models that use the TTS endpoint.
var ttsModels = map[string]bool{
	ModelTTSMega:    true,
	ModelTTSIcl:     true,
	ModelTTSDefault: true,
}

// asrModels lists models that use the ASR endpoint.
var asrModels = map[string]bool{
	ModelASR:      true,
	ModelASRLarge: true,
}

// Config configures the Volcengine Speech engine.
type Config struct {
	AppID       string // Volcengine Speech AppID
	AccessToken string // Access token for authentication
	BaseURL     string // e.g. "https://openspeech.bytedance.com"
	Model       string // Model identifier, e.g. "volcano_mega"
	Cluster     string // TTS cluster override (default: auto-detected)

	HTTPClient *http.Client
}

// Engine implements engine.Engine for Volcengine Speech TTS/ASR.
type Engine struct {
	appID       string
	accessToken string
	baseURL     string
	model       string
	cluster     string
	httpClient  *http.Client
}

// New creates a Volcengine Speech engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("VOLC_SPEECH_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	return &Engine{
		appID:       strings.TrimSpace(cfg.AppID),
		accessToken: strings.TrimSpace(cfg.AccessToken),
		baseURL:     base,
		model:       strings.TrimSpace(cfg.Model),
		cluster:     strings.TrimSpace(cfg.Cluster),
		httpClient:  hc,
	}
}

// Execute performs TTS or ASR based on the model.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("volcvoice: validate graph: %w", err)
	}

	appID := e.appID
	if appID == "" {
		appID = os.Getenv("VOLC_SPEECH_APPID")
	}
	if appID == "" {
		return engine.Result{}, ErrMissingAppID
	}

	accessToken := e.accessToken
	if accessToken == "" {
		accessToken = os.Getenv("VOLC_SPEECH_ACCESS_TOKEN")
	}
	if accessToken == "" {
		return engine.Result{}, ErrMissingAccessToken
	}

	if ttsModels[e.model] {
		dataURI, err := runTTS(ctx, e, appID, accessToken, g)
		if err != nil {
			return engine.Result{}, err
		}
		return engine.Result{Value: dataURI, Kind: engine.OutputDataURI}, nil
	}

	if asrModels[e.model] {
		text, err := runASR(ctx, e, appID, accessToken, g)
		if err != nil {
			return engine.Result{}, err
		}
		return engine.Result{Value: text, Kind: engine.OutputPlainText}, nil
	}

	return engine.Result{}, fmt.Errorf("volcvoice: unsupported model %q", e.model)
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	if asrModels[e.model] {
		return engine.Capability{
			MediaTypes:   []string{"audio"},
			Models:       []string{e.model},
			SupportsSync: true,
		}
	}
	voices := []string{
		"BV001_streaming", "BV002_streaming",
		"BV700_streaming", "BV701_streaming",
		"BV406_streaming", "BV407_streaming",
	}
	return engine.Capability{
		MediaTypes:   []string{"audio"},
		Models:       []string{e.model},
		Voices:       voices,
		SupportsSync: true,
	}
}

// ConfigSchema returns the configuration fields required by the Volcengine Speech engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "Access Token", Type: "secret", Required: true, EnvVar: "VOLC_SPEECH_ACCESS_TOKEN", Description: "Volcengine Speech access token"},
		{Key: "appId", Label: "App ID", Type: "string", Required: true, EnvVar: "VOLC_SPEECH_APPID", Description: "Volcengine Speech application ID"},
	}
}

// ModelsByCapability returns all known Volcengine Speech models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"tts": {
			ModelTTSMega,
			ModelTTSIcl,
			ModelTTSDefault,
		},
		"asr": {
			ModelASR,
			ModelASRLarge,
		},
	}
}

// doRequest sends a JSON POST to the Volcengine Speech API.
// Note: Volcengine uses "Bearer;<token>" (semicolon, not space) for auth,
// so we cannot use httpx.DoJSON directly.
func (e *Engine) doRequest(ctx context.Context, url, accessToken string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("volcvoice: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer;"+accessToken)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("volcvoice: http post: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("volcvoice: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "volcvoice")
	}
	return out, nil
}

// ttsCluster returns the cluster for TTS requests.
func (e *Engine) ttsCluster() string {
	if e.cluster != "" {
		return e.cluster
	}
	switch e.model {
	case ModelTTSMega:
		return "volcano_mega"
	case ModelTTSIcl:
		return "volcano_icl"
	default:
		return clusterTTS
	}
}

// runTTS synthesizes speech from text.
//
// Request:
//
//	POST /api/v1/tts
//	{
//	  "app": {"appid": "<appid>", "token": "access_token", "cluster": "volcano_tts"},
//	  "user": {"uid": "aigo"},
//	  "audio": {"voice_type": "<voice>", "encoding": "mp3", "speed_ratio": 1.0},
//	  "request": {"reqid": "<uuid>", "text": "<text>", "operation": "query"}
//	}
//
// Response: {"code": 3000, "message": "Success", "data": "<base64 audio>"}
func runTTS(ctx context.Context, e *Engine, appID, accessToken string, g workflow.Graph) (string, error) {
	text, ok := resolve.StringOption(g, "prompt", "text")
	if !ok || strings.TrimSpace(text) == "" {
		return "", ErrMissingText
	}
	text = strings.TrimSpace(text)

	voice, ok := resolve.StringOption(g, "voice", "voice_type")
	if !ok || strings.TrimSpace(voice) == "" {
		return "", ErrMissingVoice
	}
	voice = strings.TrimSpace(voice)

	encoding := "mp3"
	if v, ok := resolve.StringOption(g, "encoding", "response_format"); ok && strings.TrimSpace(v) != "" {
		encoding = strings.TrimSpace(v)
	}

	speedRatio := 1.0
	if v, ok := resolve.Float64Option(g, "speed_ratio"); ok && v > 0 {
		speedRatio = v
	}

	payload := map[string]any{
		"app": map[string]any{
			"appid":   appID,
			"token":   accessToken,
			"cluster": e.ttsCluster(),
		},
		"user": map[string]any{
			"uid": "aigo",
		},
		"audio": map[string]any{
			"voice_type":  voice,
			"encoding":    encoding,
			"speed_ratio": speedRatio,
		},
		"request": map[string]any{
			"reqid":     reqID(),
			"text":      text,
			"operation": "query",
		},
	}

	if v, ok := resolve.Float64Option(g, "pitch_ratio"); ok {
		payload["audio"].(map[string]any)["pitch_ratio"] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("volcvoice: marshal tts: %w", err)
	}

	respBody, err := e.doRequest(ctx, e.baseURL+"/api/v1/tts", accessToken, body)
	if err != nil {
		return "", err
	}
	return extractTTSResult(respBody, encoding)
}

// runASR transcribes audio to text.
//
// Request:
//
//	POST /api/v1/asr
//	{
//	  "app": {"appid": "<appid>", "token": "access_token", "cluster": "volcengine_streaming_common"},
//	  "user": {"uid": "aigo"},
//	  "audio": {"format": "wav", "rate": 16000, "bits": 16, "channel": 1, "codec": "raw"},
//	  "request": {"reqid": "<uuid>", "sequence": -1, "nbest": 1},
//	  "additions": {"with_speaker_info": false}
//	}
//
// Audio data is sent as base64 in the "audio.data" field.
func runASR(ctx context.Context, e *Engine, appID, accessToken string, g workflow.Graph) (string, error) {
	audioURL, ok := resolve.StringOption(g, "audio_url")
	if !ok || strings.TrimSpace(audioURL) == "" {
		if u, ok := resolve.StringOption(g, "prompt"); ok {
			s := strings.TrimSpace(u)
			if strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "data:") {
				audioURL = s
			}
		}
	} else {
		audioURL = strings.TrimSpace(audioURL)
	}
	if audioURL == "" {
		return "", ErrMissingAudioURL
	}

	audioB64, audioFormat, err := fetchAudioBase64(ctx, e.httpClient, audioURL)
	if err != nil {
		return "", err
	}

	rate := 16000
	if v, ok := resolve.IntOption(g, "sample_rate", "rate"); ok && v > 0 {
		rate = v
	}

	payload := map[string]any{
		"app": map[string]any{
			"appid":   appID,
			"token":   accessToken,
			"cluster": clusterASR,
		},
		"user": map[string]any{
			"uid": "aigo",
		},
		"audio": map[string]any{
			"format":  audioFormat,
			"rate":    rate,
			"bits":    16,
			"channel": 1,
			"codec":   "raw",
			"data":    audioB64,
		},
		"request": map[string]any{
			"reqid":    reqID(),
			"sequence": -1,
			"nbest":    1,
		},
		"additions": map[string]any{
			"with_speaker_info": false,
		},
	}

	if lang, ok := resolve.StringOption(g, "language"); ok && strings.TrimSpace(lang) != "" {
		payload["request"].(map[string]any)["language"] = strings.TrimSpace(lang)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("volcvoice: marshal asr: %w", err)
	}

	respBody, err := e.doRequest(ctx, e.baseURL+"/api/v1/asr", accessToken, body)
	if err != nil {
		return "", err
	}
	return extractASRResult(respBody)
}

// extractTTSResult extracts the base64 audio data from the TTS response.
func extractTTSResult(body []byte, encoding string) (string, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("volcvoice: decode tts response: %w", err)
	}
	if resp.Code != 3000 {
		return "", fmt.Errorf("volcvoice: tts error %d: %s", resp.Code, resp.Message)
	}
	if resp.Data == "" {
		return "", fmt.Errorf("volcvoice: tts response had no audio data")
	}

	mime := audioMIME(encoding)
	return "data:" + mime + ";base64," + resp.Data, nil
}

// extractASRResult extracts the transcribed text from the ASR response.
func extractASRResult(body []byte) (string, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  []struct {
			Text string `json:"text"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("volcvoice: decode asr response: %w", err)
	}
	if resp.Code != 1000 {
		return "", fmt.Errorf("volcvoice: asr error %d: %s", resp.Code, resp.Message)
	}
	for _, r := range resp.Result {
		if t := strings.TrimSpace(r.Text); t != "" {
			return t, nil
		}
	}
	return "", fmt.Errorf("volcvoice: asr response had no text")
}

// fetchAudioBase64 downloads audio from a URL and returns base64-encoded data and format.
func fetchAudioBase64(ctx context.Context, hc *http.Client, audioURL string) (b64, format string, err error) {
	if strings.HasPrefix(audioURL, "data:") {
		parts := strings.SplitN(audioURL, ",", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("volcvoice: invalid data URI")
		}
		mime := strings.TrimPrefix(parts[0], "data:")
		mime = strings.TrimSuffix(mime, ";base64")
		format = formatFromMIME(mime)
		return parts[1], format, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("volcvoice: build audio fetch: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("volcvoice: fetch audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("volcvoice: audio fetch returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("volcvoice: read audio body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	format = formatFromMIME(ct)
	if format == "" {
		format = formatFromURL(audioURL)
	}
	return base64.StdEncoding.EncodeToString(data), format, nil
}

func formatFromMIME(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.Contains(mime, "wav"):
		return "wav"
	case strings.Contains(mime, "mp3"), strings.Contains(mime, "mpeg"):
		return "mp3"
	case strings.Contains(mime, "pcm"):
		return "pcm"
	case strings.Contains(mime, "ogg"):
		return "ogg"
	default:
		return "wav"
	}
}

func formatFromURL(u string) string {
	u = strings.ToLower(u)
	for _, ext := range []string{".wav", ".mp3", ".pcm", ".ogg", ".m4a"} {
		if strings.HasSuffix(u, ext) || strings.Contains(u, ext+"?") {
			return strings.TrimPrefix(ext, ".")
		}
	}
	return "wav"
}

func audioMIME(encoding string) string {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "mp3":
		return "audio/mpeg"
	case "ogg_opus", "ogg":
		return "audio/ogg"
	case "pcm":
		return "audio/pcm"
	case "wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}

// reqID generates a unique request ID using crypto/rand.
func reqID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
