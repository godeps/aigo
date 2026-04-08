// Package audiogen 实现阿里云百炼「语音合成 / 声音设计」类能力。
package audiogen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// SupportedVoices lists the built-in voice identifiers for qwen3-tts models.
var SupportedVoices = []string{"Cherry", "Serena", "Ethan", "Chelsie"}

// IsTTSModel 判断是否为 Qwen 语音合成模型（不含声音设计）。
func IsTTSModel(model string) bool {
	m := strings.TrimSpace(model)
	ml := strings.ToLower(m)
	return strings.Contains(ml, "qwen") && strings.Contains(ml, "tts")
}

// RunTTS 非流式语音合成，返回音频 URL 或 data URI。
func RunTTS(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	text, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	voice, ok := graphx.AudioVoice(graph)
	if !ok || strings.TrimSpace(voice) == "" {
		return "", ierr.ErrMissingVoice
	}

	input := map[string]any{
		"text":  text,
		"voice": strings.TrimSpace(voice),
	}
	if lang, ok := graphx.AudioLanguageType(graph); ok && strings.TrimSpace(lang) != "" {
		input["language_type"] = strings.TrimSpace(lang)
	}

	parameters := map[string]any{}
	if instr, ok := graphx.AudioInstructions(graph); ok && strings.TrimSpace(instr) != "" {
		parameters["instructions"] = strings.TrimSpace(instr)
	}
	if opt, ok := graphx.AudioOptimizeInstructions(graph); ok {
		parameters["optimize_instructions"] = opt
	}

	payload := map[string]any{
		"model": model,
		"input": input,
	}
	if len(parameters) > 0 {
		payload["parameters"] = parameters
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal qwen tts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL+"/services/aigc/multimodal-generation/generation", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build qwen tts request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: call qwen tts api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read qwen tts response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
		// Enhance "Invalid voice" errors with the list of supported voices.
		if resp.StatusCode == 400 && strings.Contains(string(respBody), "InvalidParameter") && strings.Contains(string(respBody), "voice") {
			return "", fmt.Errorf("%w (supported voices: %s)", err, strings.Join(SupportedVoices, ", "))
		}
		return "", err
	}

	var decoded struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Output  struct {
			Audio struct {
				URL  string `json:"url"`
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("aliyun: decode qwen tts response: %w", err)
	}
	if strings.TrimSpace(decoded.Code) != "" {
		return "", fmt.Errorf("aliyun: qwen tts api error %s: %s", decoded.Code, decoded.Message)
	}

	if u := strings.TrimSpace(decoded.Output.Audio.URL); u != "" {
		return u, nil
	}
	if d := strings.TrimSpace(decoded.Output.Audio.Data); d != "" {
		return "data:audio/wav;base64," + d, nil
	}

	return "", errors.New("aliyun: qwen tts response did not contain audio url or data")
}
