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

// IsReferenceToVideoModel 参考图/视频生视频（如 *-r2v）。
func IsReferenceToVideoModel(model string) bool {
	return strings.Contains(model, "-r2v")
}

// RunReferenceToVideo 参考媒体生视频异步任务。
func RunReferenceToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	referenceURLs := graphx.MediaURLs(graph)
	if len(referenceURLs) == 0 {
		return "", ierr.ErrMissingReference
	}

	input := map[string]any{
		"prompt":         prompt,
		"reference_urls": referenceURLs,
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
