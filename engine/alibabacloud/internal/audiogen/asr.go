package audiogen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// Qwen3-ASR endpoints per official DashScope documentation.
const (
	// asrSyncEndpoint is the OpenAI-compatible chat completions endpoint
	// used by qwen3-asr-flash for synchronous recognition.
	asrSyncEndpoint = "/compatible-mode/v1/chat/completions"

	// asrAsyncEndpoint is the DashScope ASR transcription endpoint
	// used by qwen3-asr-flash-filetrans for async file transcription.
	asrAsyncEndpoint = "/services/audio/asr/transcription"
)

// audioURL extracts the audio URL from the workflow graph.
func audioURL(graph workflow.Graph) (string, error) {
	if u, ok := graphx.StringOption(graph, "audio_url"); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u), nil
	}
	// Also check prompt field — animus buildTranscribeTask puts the URL there.
	if u, ok := graphx.StringOption(graph, "prompt"); ok && strings.TrimSpace(u) != "" {
		s := strings.TrimSpace(u)
		// Only use prompt if it looks like a URL, path, or data URI.
		if strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "data:") {
			return s, nil
		}
	}
	return "", ierr.ErrMissingAudioURL
}

// RunQwenASR 调用 Qwen3-ASR-Flash 同步语音识别（OpenAI 兼容格式）。
//
// API: POST {baseURL}/compatible-mode/v1/chat/completions
//
// Request format (OpenAI-compatible):
//
//	{
//	  "model": "qwen3-asr-flash",
//	  "messages": [{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"<url-or-base64>"}}]}],
//	  "asr_options": {"language": "zh", "enable_itn": false}
//	}
func RunQwenASR(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	url, err := audioURL(graph)
	if err != nil {
		return "", err
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
							"data": url,
						},
					},
				},
			},
		},
		"stream": false,
	}

	// Optional asr_options.
	asrOpts := map[string]any{}
	if lang, ok := graphx.StringOption(graph, "language"); ok && strings.TrimSpace(lang) != "" {
		asrOpts["language"] = strings.TrimSpace(lang)
	}
	if len(asrOpts) > 0 {
		payload["asr_options"] = asrOpts
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal qwen-asr request: %w", err)
	}

	// The sync ASR endpoint uses /compatible-mode/v1/... which is at the host
	// root, not under /api/v1. Strip the API version suffix to avoid a doubled path.
	syncBase := strings.TrimSuffix(rt.BaseURL, "/api/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, syncBase+asrSyncEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build qwen-asr request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: call qwen-asr api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read qwen-asr response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return "", &aigoerr.Error{
				Code:       aigoerr.CodeInvalidInput,
				StatusCode: 404,
				Message:    fmt.Sprintf("aliyun: qwen-asr model %q not found (404)", model),
				Retryable:  false,
			}
		}
		return "", aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
	}

	return extractChatCompletion(respBody)
}

// RunQwenASRFiletrans 调用 Qwen3-ASR-Flash-Filetrans 异步录音文件识别。
//
// API: POST {baseURL}/services/audio/asr/transcription
// Header: X-DashScope-Async: enable
//
// Request format (DashScope):
//
//	{
//	  "model": "qwen3-asr-flash-filetrans",
//	  "input": {"file_url": "<url>"},
//	  "parameters": {"channel_id": [0], "enable_itn": false}
//	}
func RunQwenASRFiletrans(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	url, err := audioURL(graph)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(url, "data:") {
		return "", fmt.Errorf("filetrans requires HTTP(S) file_url: %w", ierr.ErrDataURINotSupported)
	}

	input := map[string]any{
		"file_url": url,
	}

	parameters := map[string]any{
		"channel_id": []int{0},
	}
	if lang, ok := graphx.StringOption(graph, "language"); ok && strings.TrimSpace(lang) != "" {
		parameters["language"] = strings.TrimSpace(lang)
	}

	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	result, err := async.Submit(ctx, rt, apiKey, asrAsyncEndpoint, payload, async.URLExtractor{
		URLFields: [][]string{
			{"result", "transcription_url"},
		},
	})
	if err != nil {
		return "", err
	}

	if !rt.WaitForCompletion {
		return result, nil
	}

	return fetchTranscriptionText(ctx, rt.HTTPClient, result)
}

// fetchTranscriptionText downloads the transcription JSON from the given URL
// and extracts the text from transcripts[0].text.
func fetchTranscriptionText(ctx context.Context, client *http.Client, transcriptionURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transcriptionURL, nil)
	if err != nil {
		return "", fmt.Errorf("aliyun: build transcription fetch request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: fetch transcription json: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read transcription json: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("aliyun: transcription json HTTP %d", resp.StatusCode)
	}

	var result struct {
		Transcripts []struct {
			Text string `json:"text"`
		} `json:"transcripts"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("aliyun: decode transcription json: %w", err)
	}
	if len(result.Transcripts) > 0 {
		if text := strings.TrimSpace(result.Transcripts[0].Text); text != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("aliyun: transcription json contained no text")
}

// extractChatCompletion parses an OpenAI-compatible chat completions response.
//
// Response format:
//
//	{"choices":[{"message":{"content":"transcribed text"}}]}
func extractChatCompletion(body []byte) (string, error) {
	var decoded struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("aliyun: decode qwen-asr response: %w", err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("aliyun: qwen-asr api error %s: %s", decoded.Error.Code, decoded.Error.Message)
	}

	if len(decoded.Choices) > 0 {
		if text := strings.TrimSpace(decoded.Choices[0].Message.Content); text != "" {
			return text, nil
		}
	}

	return "", fmt.Errorf("aliyun: qwen-asr response did not contain transcription text")
}
