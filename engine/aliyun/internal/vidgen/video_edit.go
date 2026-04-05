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
