package vidgen

import (
	"context"
	"strings"

	"github.com/godeps/aigo/engine/aliyun/internal/async"
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// IsReferenceToVideoModel 参考图/视频生视频（如 *-r2v 或 *-i2v）。
func IsReferenceToVideoModel(model string) bool {
	return strings.Contains(model, "-r2v") || strings.Contains(model, "-i2v")
}

// RunReferenceToVideo 参考媒体生视频异步任务。
// 使用 input.media（typed object array）格式，与 video-synthesis API 统一。
// media 类型：图片 → first_frame / last_frame，视频 → first_clip。
func RunReferenceToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	media := buildReferenceMedia(graph)
	if len(media) == 0 {
		return "", ierr.ErrMissingReference
	}

	input := map[string]any{
		"prompt": prompt,
		"media":  media,
	}
	if negativePrompt, ok := graphx.StringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := BuildParameters(graph, false)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// buildReferenceMedia 为 i2v/r2v 构建 media 数组。
// Wan API 有效类型：first_frame, last_frame, driving_audio, first_clip。
func buildReferenceMedia(graph workflow.Graph) []map[string]any {
	media := make([]map[string]any, 0)
	images := graphx.ImageURLs(graph)
	videos := graphx.VideoURLs(graph)

	for i, url := range images {
		mediaType := "first_frame"
		if i == 1 {
			mediaType = "last_frame"
		}
		media = append(media, map[string]any{"type": mediaType, "url": url})
	}
	for _, url := range videos {
		media = append(media, map[string]any{"type": "first_clip", "url": url})
	}

	return media
}
