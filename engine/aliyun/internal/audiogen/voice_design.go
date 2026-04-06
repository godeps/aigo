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
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

type voiceDesignResult struct {
	Type         string `json:"type"`
	Voice        string `json:"voice"`
	TargetModel  string `json:"target_model"`
	PreviewAudio string `json:"preview_audio,omitempty"`
}

// RunVoiceDesign 创建定制音色；返回 JSON 字符串（含 voice、可选预览 data URI）。
func RunVoiceDesign(ctx context.Context, rt *runtime.RT, apiKey, designModel string, graph workflow.Graph) (string, error) {
	voicePrompt, previewText, targetModel, err := graphx.VoiceDesignFields(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{
		"action":       "create",
		"target_model": strings.TrimSpace(targetModel),
		"voice_prompt": voicePrompt,
		"preview_text": previewText,
	}
	if name, ok := graphx.VoiceDesignPreferredName(graph); ok && strings.TrimSpace(name) != "" {
		input["preferred_name"] = strings.TrimSpace(name)
	}
	if lang, ok := graphx.VoiceDesignLanguage(graph); ok && strings.TrimSpace(lang) != "" {
		input["language"] = strings.TrimSpace(lang)
	}

	parameters := map[string]any{}
	if n, ok := graphx.VoiceDesignSampleRate(graph); ok && n > 0 {
		parameters["sample_rate"] = n
	}
	if format, ok := graphx.VoiceDesignResponseFormat(graph); ok && strings.TrimSpace(format) != "" {
		parameters["response_format"] = strings.TrimSpace(format)
	}

	payload := map[string]any{
		"model": designModel,
		"input": input,
	}
	if len(parameters) > 0 {
		payload["parameters"] = parameters
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal voice design request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL+"/services/audio/tts/customization", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build voice design request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: call voice design api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read voice design response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
	}

	var decoded struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Output  struct {
			Voice        string `json:"voice"`
			TargetModel  string `json:"target_model"`
			PreviewAudio struct {
				Data           string `json:"data"`
				ResponseFormat string `json:"response_format"`
			} `json:"preview_audio"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("aliyun: decode voice design response: %w", err)
	}
	if strings.TrimSpace(decoded.Code) != "" {
		return "", fmt.Errorf("aliyun: voice design api error %s: %s", decoded.Code, decoded.Message)
	}

	out := voiceDesignResult{
		Type:        "qwen-voice-design",
		Voice:       strings.TrimSpace(decoded.Output.Voice),
		TargetModel: strings.TrimSpace(decoded.Output.TargetModel),
	}
	if out.TargetModel == "" {
		out.TargetModel = strings.TrimSpace(targetModel)
	}

	if !graphx.VoiceDesignOmitPreview(graph) {
		if data := strings.TrimSpace(decoded.Output.PreviewAudio.Data); data != "" {
			mime := previewAudioMIME(strings.TrimSpace(decoded.Output.PreviewAudio.ResponseFormat))
			out.PreviewAudio = "data:" + mime + ";base64," + data
		}
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal voice design result: %w", err)
	}
	return string(encoded), nil
}

func previewAudioMIME(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/opus"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/wav"
	}
}
