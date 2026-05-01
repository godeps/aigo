package imggen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// IsMultimodalImageModel 匹配 multimodal-generation/generation 的图生图类模型
// （如 wan2.7-image、z-image-turbo），请求体形态一致。
func IsMultimodalImageModel(model string) bool {
	return (strings.Contains(model, "image") || strings.HasPrefix(model, "z-image")) && !strings.Contains(model, "video")
}

// RunMultimodalImage 同步调用多模态图生成接口（仅返回首张图，向后兼容）。
//
// Prefer RunMultimodalImageMulti when the caller wants the full set of generated
// images (n>1 use case). This wrapper keeps the legacy signature for existing
// integrations that only consume the first URL.
func RunMultimodalImage(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	value, _, err := RunMultimodalImageMulti(ctx, rt, apiKey, model, graph)
	return value, err
}

// RunMultimodalImageMulti 同步调用多模态图生成接口，返回首张图与全部生成结果。
//
// When the request asked for n>1 images, the second return value contains
// every image URL with a stable per-item index in metadata; otherwise it is
// nil (callers should use the first return value alone).
func RunMultimodalImageMulti(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, []engine.ResultItem, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", nil, err
	}

	content := []map[string]any{
		{"text": prompt},
	}
	for _, imageURL := range graphx.ImageURLs(graph) {
		content = append(content, map[string]any{"image": imageURL})
	}

	input := map[string]any{
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	parameters := map[string]any{}
	if size, ok := graphx.StringOption(graph, "size"); ok {
		parameters["size"] = graphx.NormalizeSize(size)
	}
	if size, ok := graphx.WidthHeightSize(graph); ok {
		parameters["size"] = size
	}
	if watermark, ok := graphx.BoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if promptExtend, ok := graphx.BoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}
	if thinkingMode, ok := graphx.BoolOption(graph, "thinking_mode"); ok {
		parameters["thinking_mode"] = thinkingMode
	}
	if n, ok := graphx.IntOption(graph, "n"); ok {
		parameters["n"] = n
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
		return "", nil, fmt.Errorf("aliyun: marshal multimodal image request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL+"/services/aigc/multimodal-generation/generation", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("aliyun: build multimodal image request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("aliyun: call multimodal image api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("aliyun: read multimodal image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
		// Enhance "invalid size" errors with the list of supported sizes.
		if resp.StatusCode == 400 && strings.Contains(string(respBody), "size") {
			return "", nil, fmt.Errorf("%w (supported image sizes: 1024x1024, 1024x1536, 1536x1024, 512x512)", err)
		}
		return "", nil, err
	}

	var decoded struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Type  string `json:"type"`
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", nil, fmt.Errorf("aliyun: decode multimodal image response: %w", err)
	}

	var (
		first   string
		results []engine.ResultItem
	)
	for _, choice := range decoded.Output.Choices {
		for _, item := range choice.Message.Content {
			if item.Image == "" {
				continue
			}
			if first == "" {
				first = item.Image
			}
			results = append(results, engine.ResultItem{
				Value:    item.Image,
				Kind:     engine.OutputURL,
				Metadata: map[string]string{"index": strconv.Itoa(len(results))},
			})
		}
	}
	if first == "" {
		return "", nil, errors.New("aliyun: multimodal image response did not contain an image URL")
	}
	return first, results, nil
}
