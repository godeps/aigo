package vidgen

import (
	"context"

	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// RunHappyHorseTextToVideo 欢乐马文生视频异步任务。
func RunHappyHorseTextToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{"prompt": prompt}
	parameters := buildHappyHorseParams(graph, true, false)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// RunHappyHorseImageToVideo 欢乐马图生视频异步任务（首帧驱动）。
func RunHappyHorseImageToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	images := graphx.ImageURLs(graph)
	if len(images) == 0 {
		return "", ierr.ErrMissingReference
	}

	media := []map[string]any{{"type": "first_frame", "url": images[0]}}
	input := map[string]any{"media": media}
	if prompt, err := graphx.Prompt(graph); err == nil {
		input["prompt"] = prompt
	}

	parameters := buildHappyHorseParams(graph, false, false)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// RunHappyHorseReferenceToVideo 欢乐马参考图生视频异步任务（1-9 张参考图）。
func RunHappyHorseReferenceToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	images := graphx.ImageURLs(graph)
	if len(images) == 0 {
		return "", ierr.ErrMissingReference
	}
	if len(images) > 9 {
		return "", ierr.ErrTooManyHappyHorseImages
	}

	media := make([]map[string]any, 0, len(images))
	for _, url := range images {
		media = append(media, map[string]any{"type": "reference_image", "url": url})
	}

	input := map[string]any{
		"prompt": prompt,
		"media":  media,
	}
	parameters := buildHappyHorseParams(graph, true, false)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// RunHappyHorseVideoEdit 欢乐马视频编辑异步任务。
func RunHappyHorseVideoEdit(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	media := graphx.VideoEditMedia(graph)
	if len(media) == 0 {
		return "", ierr.ErrMissingReference
	}

	input := map[string]any{
		"prompt": prompt,
		"media":  media,
	}
	parameters := buildHappyHorseParams(graph, false, true)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}

// buildHappyHorseParams 构建 HappyHorse 特有的 parameters。
// HappyHorse 始终使用 resolution（非 size），可选 ratio 和 audio_setting。
func buildHappyHorseParams(graph workflow.Graph, includeRatio, includeAudioSetting bool) map[string]any {
	p := map[string]any{}

	if resolution, ok := graphx.Resolution(graph); ok {
		p["resolution"] = resolution
	}
	if includeRatio {
		if ratio, ok := graphx.StringOption(graph, "ratio"); ok {
			p["ratio"] = ratio
		}
	}
	if duration, ok := graphx.IntOption(graph, "duration"); ok {
		p["duration"] = duration
	}
	if watermark, ok := graphx.BoolOption(graph, "watermark"); ok {
		p["watermark"] = watermark
	}
	if includeAudioSetting {
		if audioSetting, ok := graphx.StringOption(graph, "audio_setting"); ok {
			p["audio_setting"] = audioSetting
		}
	}
	if seed, ok := graphx.IntOption(graph, "seed"); ok {
		p["seed"] = seed
	}

	return p
}
