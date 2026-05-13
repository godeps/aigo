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

func validateWanVideoEditMedia(media []map[string]any) error {
	var videos, images int
	for _, m := range media {
		switch m["type"] {
		case "video":
			videos++
		case "reference_image":
			images++
		}
	}
	if videos != 1 {
		return ierr.ErrWanVideoEditMissingVideo
	}
	if images > 4 {
		return ierr.ErrWanVideoEditTooManyImages
	}
	return nil
}

// IsVideoEditModel 视频编辑（如 *videoedit*）。
func IsVideoEditModel(model string) bool {
	return strings.Contains(model, "videoedit")
}

// RunVideoEdit 视频编辑异步任务。
func RunVideoEdit(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	media := graphx.VideoEditMedia(graph)
	if err := validateWanVideoEditMedia(media); err != nil {
		return "", err
	}

	input := map[string]any{
		"prompt": prompt,
		"media":  media,
	}
	if negativePrompt, ok := graphx.StringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := BuildParameters(graph, true)
	payload := map[string]any{
		"model":      model,
		"input":      input,
		"parameters": parameters,
	}

	return async.Submit(ctx, rt, apiKey, "/services/aigc/video-generation/video-synthesis", payload, async.URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
}
