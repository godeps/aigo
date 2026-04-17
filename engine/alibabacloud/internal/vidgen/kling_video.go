package vidgen

import (
	"context"
	"strings"

	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// IsKlingVideoModel 可灵视频生成模型。
func IsKlingVideoModel(model string) bool {
	return strings.Contains(model, "kling") && strings.Contains(model, "video-generation")
}

// RunKlingVideo 可灵视频生成异步任务。
// 支持 kling/kling-v3-video-generation（t2v, i2v）和
// kling/kling-v3-omni-video-generation（增加 reference, editing）。
func RunKlingVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{"prompt": prompt}
	if negativePrompt, ok := graphx.StringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	if media := buildKlingMedia(graph); len(media) > 0 {
		input["media"] = media
	}

	parameters := buildKlingParameters(graph)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// buildKlingParameters 构建 Kling 特有的 parameters。
// Kling 使用 mode (pro/std) 而非 size/resolution。
func buildKlingParameters(graph workflow.Graph) map[string]any {
	p := map[string]any{}

	if mode, ok := graphx.StringOption(graph, "mode"); ok {
		p["mode"] = mode
	}
	if ar, ok := graphx.StringOption(graph, "aspect_ratio"); ok {
		p["aspect_ratio"] = ar
	}
	if dur, ok := graphx.IntOption(graph, "duration"); ok {
		p["duration"] = dur
	}
	if audio, ok := graphx.BoolOption(graph, "audio"); ok {
		p["audio"] = audio
	}
	if wm, ok := graphx.BoolOption(graph, "watermark"); ok {
		p["watermark"] = wm
	}

	return p
}

// buildKlingMedia 从 graph 构建 Kling media 数组。
// Kling 使用 typed media objects: {type: "first_frame"|"last_frame"|"refer"|"feature"|"base", url: "..."}.
func buildKlingMedia(graph workflow.Graph) []map[string]any {
	media := make([]map[string]any, 0)

	images := graphx.ImageURLs(graph)
	videos := graphx.VideoURLs(graph)

	for i, url := range images {
		mediaType := "first_frame"
		if i == 1 {
			mediaType = "last_frame"
		}
		if i > 1 {
			mediaType = "refer"
		}
		media = append(media, map[string]any{"type": mediaType, "url": url})
	}

	for _, url := range videos {
		media = append(media, map[string]any{"type": "feature", "url": url})
	}

	return media
}
