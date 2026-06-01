package vidgen

import (
	"context"
	"strings"

	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
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

	media, err := buildReferenceMedia(ctx, rt, apiKey, model, graph)
	if err != nil {
		return "", err
	}
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
//
// i2v 模型 media 类型：first_frame, last_frame, driving_audio, first_clip。
// r2v 模型 media 类型：reference_image, reference_video, first_frame。
func buildReferenceMedia(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) ([]map[string]any, error) {
	isR2V := strings.Contains(model, "-r2v")
	media := make([]map[string]any, 0)
	images := graphx.ImageURLs(graph)
	videos := graphx.VideoURLs(graph)

	if isR2V {
		if ffURL, ok := graphx.FirstFrameURL(graph); ok {
			url, err := EnsureRemoteURL(ctx, rt, apiKey, model, ffURL)
			if err != nil {
				return nil, err
			}
			media = append(media, map[string]any{"type": "first_frame", "url": url})
		}
	}

	for i, rawURL := range images {
		url, err := EnsureRemoteURL(ctx, rt, apiKey, model, rawURL)
		if err != nil {
			return nil, err
		}
		var mediaType string
		if isR2V {
			mediaType = "reference_image"
		} else {
			mediaType = "first_frame"
			if i == 1 {
				mediaType = "last_frame"
			}
		}
		entry := map[string]any{"type": mediaType, "url": url}
		if isR2V {
			if voice, ok := graphx.ReferenceVoiceForURL(graph, rawURL); ok {
				entry["reference_voice"] = voice
			}
		}
		media = append(media, entry)
	}
	for _, rawURL := range videos {
		url, err := EnsureRemoteURL(ctx, rt, apiKey, model, rawURL)
		if err != nil {
			return nil, err
		}
		videoType := "first_clip"
		if isR2V {
			videoType = "reference_video"
		}
		entry := map[string]any{"type": videoType, "url": url}
		if isR2V {
			if voice, ok := graphx.ReferenceVoiceForURL(graph, rawURL); ok {
				entry["reference_voice"] = voice
			}
		}
		media = append(media, entry)
	}

	if !isR2V {
		for _, rawURL := range graphx.AudioURLs(graph) {
			url, err := EnsureRemoteURL(ctx, rt, apiKey, model, rawURL)
			if err != nil {
				return nil, err
			}
			media = append(media, map[string]any{"type": "driving_audio", "url": url})
		}
	}

	return media, nil
}
