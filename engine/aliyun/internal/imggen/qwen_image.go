// Package imggen 实现阿里云百炼「图片生成」类能力（文生图等）。
package imggen

import (
	"context"
	"strings"

	"github.com/godeps/aigo/engine/aliyun/internal/async"
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// IsQwenImageModel 判断是否为 qwen-image 系列文生图模型。
func IsQwenImageModel(model string) bool {
	return strings.HasPrefix(model, "qwen-image")
}

// RunQwenImage 调用 text2image 异步接口。
func RunQwenImage(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{
		"prompt": prompt,
	}
	if negativePrompt, ok := graphx.StringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := map[string]any{
		"size": graphx.Size(graph, "1024*1024"),
	}
	if n, ok := graphx.IntOption(graph, "n"); ok {
		parameters["n"] = n
	}
	if watermark, ok := graphx.BoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if promptExtend, ok := graphx.BoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}
	if seed, ok := graphx.IntOption(graph, "seed"); ok {
		parameters["seed"] = seed
	}

	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/text2image/image-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"results", "url"}, {"result_url"}},
	})
}
