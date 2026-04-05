package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/workflow"
)

func (e *Engine) runQwenImageGenerations(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	payload := map[string]any{
		"model": e.model,
		"input": map[string]any{
			"messages": []any{
				map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{"text": prompt},
					},
				},
			},
		},
		"parameters": map[string]any{},
	}
	params := payload["parameters"].(map[string]any)
	if neg, ok := graph.ExtractNegativePrompt(g); ok {
		params["negative_prompt"] = neg
	}
	if s := graph.ExtractImageSizeOpenAI(g); s != "" {
		params["size"] = s
	}
	if v, ok := graph.StringOption(g, "prompt_extend"); ok {
		if v == "true" || v == "false" {
			params["prompt_extend"] = v == "true"
		}
	}
	if v, ok := graph.StringOption(g, "watermark"); ok {
		if v == "true" || v == "false" {
			params["watermark"] = v == "true"
		}
	}
	_ = graph.MergeJSONObject(g, payload, "extra_body", "qwen_image_extra")
	_ = graph.MergeJSONObject(g, params, "qwen_parameters_extra")

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal qwen image: %w", err)
	}
	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/images/generations"), apiKey, body, "application/json")
	if err != nil {
		return "", err
	}
	return decodeOpenAIImageData(respBody)
}
