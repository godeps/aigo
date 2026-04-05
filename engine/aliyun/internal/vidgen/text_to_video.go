package vidgen

import (
	"context"
	"strings"

	"github.com/godeps/aigo/engine/aliyun/internal/async"
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// IsTextToVideoModel 文生视频（如 *-t2v）。
func IsTextToVideoModel(model string) bool {
	return strings.Contains(model, "-t2v")
}

// RunTextToVideo 文生视频异步任务。
func RunTextToVideo(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := graphx.Prompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{"prompt": prompt}
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
