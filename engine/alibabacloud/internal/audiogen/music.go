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
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// RunMusic 同步音乐生成，返回音频 URL 或 data URI。
func RunMusic(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	input := map[string]any{}

	if lyrics, ok := graphx.StringOption(graph, "lyrics"); ok && strings.TrimSpace(lyrics) != "" {
		input["lyrics"] = strings.TrimSpace(lyrics)
	}
	if prompt, err := graphx.Prompt(graph); err == nil && strings.TrimSpace(prompt) != "" {
		input["prompt"] = strings.TrimSpace(prompt)
	}
	if _, hasLyrics := input["lyrics"]; !hasLyrics {
		if _, hasPrompt := input["prompt"]; !hasPrompt {
			return "", ierr.ErrMissingPrompt
		}
	}

	if gender, ok := graphx.StringOption(graph, "gender"); ok && strings.TrimSpace(gender) != "" {
		input["gender"] = strings.TrimSpace(gender)
	}
	if format, ok := graphx.StringOption(graph, "format"); ok && strings.TrimSpace(format) != "" {
		input["format"] = strings.TrimSpace(format)
	}
	if watermark, ok := graphx.BoolOption(graph, "enable_aigc_watermark"); ok {
		input["enable_aigc_watermark"] = watermark
	}

	payload := map[string]any{
		"model": model,
		"input": input,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal music request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL+"/services/audio/music/generation", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build music request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: call music api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read music response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
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
		return "", fmt.Errorf("aliyun: decode music response: %w", err)
	}
	if strings.TrimSpace(decoded.Code) != "" {
		return "", fmt.Errorf("aliyun: music api error %s: %s", decoded.Code, decoded.Message)
	}

	if u := strings.TrimSpace(decoded.Output.Audio.URL); u != "" {
		return u, nil
	}
	if d := strings.TrimSpace(decoded.Output.Audio.Data); d != "" {
		return "data:audio/mpeg;base64," + d, nil
	}

	return "", errors.New("aliyun: music response did not contain audio url or data")
}
