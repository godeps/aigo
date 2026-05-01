// Package threedgen 实现阿里云百炼「3D 资产生成」类能力（Tripo 系列）。
package threedgen

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// Tripo API 文档限制：多图最多 4 张、prompt 最长 1024 个字符。
const (
	tripoMaxImages   = 4
	tripoMaxPromptCP = 1024
)

// 模型常量与根包保持同步，避免循环依赖。
const (
	modelTripoP1  = "Tripo/Tripo-P1.0"
	modelTripoH31 = "Tripo/Tripo-H3.1"
)

// tripoEndpoint 是 Tripo 3D 生成 API 路径（相对 BaseURL，BaseURL 已含 /api/v1）。
const tripoEndpoint = "/services/aigc/video-generation/3d-generation"

// IsTripoModel 判断是否为 Tripo 3D 系列模型。
func IsTripoModel(model string) bool {
	return strings.HasPrefix(model, "Tripo/")
}

// RunTripo3D 调用百炼 Tripo 3D 异步接口。
//
// 输入互斥优先级：images（2-4 张）> image（单张）> prompt。
// 缺失三者全部时返回 ErrMissingTripoInput；
// images 超过 4 张返回 ErrTooManyTripoImages；
// prompt 超过 1024 个字符返回 ErrTripoPromptTooLong。
func RunTripo3D(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	input, err := buildTripoInput(graph)
	if err != nil {
		return "", err
	}

	parameters := map[string]any{}
	if v, ok := graphx.StringOption(graph, "texture_quality"); ok && strings.TrimSpace(v) != "" {
		parameters["texture_quality"] = strings.TrimSpace(v)
	}
	// geometry_quality 仅 Tripo-H3.1 支持。
	if model == modelTripoH31 {
		if v, ok := graphx.StringOption(graph, "geometry_quality"); ok && strings.TrimSpace(v) != "" {
			parameters["geometry_quality"] = strings.TrimSpace(v)
		}
	}

	payload := map[string]any{
		"model": model,
		"input": input,
	}
	if len(parameters) > 0 {
		payload["parameters"] = parameters
	}

	return async.Submit(ctx, rt, apiKey, tripoEndpoint, payload, async.URLExtractor{
		URLFields: [][]string{{"results", "pbr_model_url"}},
	})
}

func buildTripoInput(graph workflow.Graph) (map[string]any, error) {
	images := graphx.ImageURLs(graph)
	if len(images) > tripoMaxImages {
		return nil, ierr.ErrTooManyTripoImages
	}
	if len(images) >= 2 {
		urls := make([]any, len(images))
		for i, u := range images {
			urls[i] = u
		}
		return map[string]any{"images": urls}, nil
	}
	if len(images) == 1 {
		return map[string]any{"image": images[0]}, nil
	}

	// 没有图片输入时回退到 prompt。
	prompt, ok := tripoPrompt(graph)
	if !ok {
		return nil, ierr.ErrMissingTripoInput
	}
	if utf8.RuneCountInString(prompt) > tripoMaxPromptCP {
		return nil, ierr.ErrTripoPromptTooLong
	}
	return map[string]any{"prompt": prompt}, nil
}

// tripoPrompt 与 graphx.Prompt 类似，但不会因缺失而返回错误，
// 让上层根据是否有图片输入决定是否报错。
func tripoPrompt(graph workflow.Graph) (string, bool) {
	for _, ref := range graph.FindByClassType("CLIPTextEncode") {
		if v, ok := ref.Node.StringInput("text"); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	for _, key := range []string{"prompt", "text", "value"} {
		if v, ok := graphx.StringOption(graph, key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}
