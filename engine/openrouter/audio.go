package openrouter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

// runTTS generates speech via chat completions with audio modality.
//
// Request format:
//
//	{
//	  "model": "openai/gpt-audio",
//	  "modalities": ["text", "audio"],
//	  "audio": {"voice": "alloy", "format": "wav"},
//	  "messages": [{"role": "user", "content": "Read this text aloud: ..."}]
//	}
//
// Response: choices[0].message.audio.data contains base64-encoded audio.
func runTTS(ctx context.Context, e *Engine, apiKey, model string, graph workflow.Graph) (string, error) {
	text, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	voice, ok := resolve.StringOption(graph, "voice")
	if !ok || strings.TrimSpace(voice) == "" {
		return "", ErrMissingVoice
	}
	voice = strings.TrimSpace(voice)

	format := "wav"
	if f, ok := resolve.StringOption(graph, "response_format"); ok && strings.TrimSpace(f) != "" {
		format = strings.TrimSpace(f)
	}

	payload := map[string]any{
		"model":      model,
		"modalities": []string{"text", "audio"},
		"audio": map[string]any{
			"voice":  voice,
			"format": format,
		},
		"messages": []map[string]any{
			{"role": "user", "content": text},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal tts request: %w", err)
	}

	respBody, err := doRequest(ctx, e.httpClient, http.MethodPost, e.baseURL+"/v1/chat/completions", apiKey, body)
	if err != nil {
		return "", err
	}
	return extractAudioFromChat(respBody, format)
}

// runASR transcribes audio via chat completions with input_audio content.
//
// Request format:
//
//	{
//	  "model": "openai/gpt-audio",
//	  "messages": [{"role": "user", "content": [
//	    {"type": "input_audio", "input_audio": {"data": "<base64>", "format": "wav"}},
//	    {"type": "text", "text": "Transcribe this audio."}
//	  ]}]
//	}
//
// The audio_url from the graph is fetched and base64-encoded before sending.
func runASR(ctx context.Context, e *Engine, apiKey, model string, graph workflow.Graph) (string, error) {
	audioURL, ok := resolve.StringOption(graph, "audio_url")
	if !ok || strings.TrimSpace(audioURL) == "" {
		// Also check prompt field.
		if u, ok := resolve.StringOption(graph, "prompt"); ok {
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

	instruction := "Transcribe this audio."
	if lang, ok := resolve.StringOption(graph, "language"); ok && strings.TrimSpace(lang) != "" {
		instruction = fmt.Sprintf("Transcribe this audio in %s.", strings.TrimSpace(lang))
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data":   audioB64,
							"format": audioFormat,
						},
					},
					{
						"type": "text",
						"text": instruction,
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal asr request: %w", err)
	}

	respBody, err := doRequest(ctx, e.httpClient, http.MethodPost, e.baseURL+"/v1/chat/completions", apiKey, body)
	if err != nil {
		return "", err
	}
	return extractTextFromChat(respBody)
}

// extractAudioFromChat extracts base64 audio from a chat completion response.
func extractAudioFromChat(body []byte, format string) (string, error) {
	var resp struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Audio *struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("openrouter: decode tts response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("openrouter: api error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: tts response had no choices")
	}
	audio := resp.Choices[0].Message.Audio
	if audio == nil || audio.Data == "" {
		return "", fmt.Errorf("openrouter: tts response had no audio data")
	}
	mime := audioMIME(format)
	return "data:" + mime + ";base64," + audio.Data, nil
}

// extractTextFromChat extracts text content from a chat completion response.
func extractTextFromChat(body []byte) (string, error) {
	var resp struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("openrouter: decode asr response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("openrouter: api error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: asr response had no choices")
	}

	raw := resp.Choices[0].Message.Content

	// Try as plain string first.
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if t := strings.TrimSpace(text); t != "" {
			return t, nil
		}
	}

	// Try as array of content blocks.
	var blocks []map[string]any
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if block["type"] == "text" {
				if t, ok := block["text"].(string); ok && strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t), nil
				}
			}
		}
	}

	return "", fmt.Errorf("openrouter: asr response had no text content")
}

// fetchAudioBase64 downloads an audio URL and returns its base64 encoding and format.
func fetchAudioBase64(ctx context.Context, hc *http.Client, audioURL string) (b64, format string, err error) {
	// Handle data URIs directly.
	if strings.HasPrefix(audioURL, "data:") {
		parts := strings.SplitN(audioURL, ",", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("openrouter: invalid data URI")
		}
		// Extract format from MIME: data:audio/wav;base64 → wav
		mime := strings.TrimPrefix(parts[0], "data:")
		mime = strings.TrimSuffix(mime, ";base64")
		format = formatFromMIME(mime)
		return parts[1], format, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("openrouter: build audio fetch request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("openrouter: fetch audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("openrouter: audio fetch returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("openrouter: read audio body: %w", err)
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
	case strings.Contains(mime, "flac"):
		return "flac"
	case strings.Contains(mime, "ogg"):
		return "ogg"
	case strings.Contains(mime, "webm"):
		return "webm"
	default:
		return "wav"
	}
}

func formatFromURL(u string) string {
	u = strings.ToLower(u)
	for _, ext := range []string{".wav", ".mp3", ".flac", ".ogg", ".webm", ".m4a"} {
		if strings.HasSuffix(u, ext) || strings.Contains(u, ext+"?") {
			return strings.TrimPrefix(ext, ".")
		}
	}
	return "wav"
}

func audioMIME(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/wav"
	}
}
