package aliyun

import (
	"context"
	"os"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aliyun/internal/async"
)

// extractorForModel returns the URLExtractor for a given model name.
// The paths mirror those used by each handler in imggen/vidgen/audiogen.
func extractorForModel(model string) async.URLExtractor {
	switch model {
	case ModelQwenImage, ModelQwenImage2, ModelQwenImageEditPlus,
		ModelWanImage, ModelZImageTurbo:
		return async.URLExtractor{URLFields: [][]string{{"results", "url"}, {"result_url"}}}
	case ModelQwenASRFlash, ModelQwenASRFlashFiletrans:
		return async.URLExtractor{URLFields: [][]string{{"results", "transcription_url"}, {"results", "text"}}}
	default:
		// All video models (wan, kling) use video_url.
		return async.URLExtractor{URLFields: [][]string{{"video_url"}}}
	}
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	ex := extractorForModel(e.model)
	url, err := async.Wait(ctx, &e.rt, apiKey, remoteID, ex)
	if err != nil {
		return engine.Result{}, err
	}

	kind := engine.OutputURL
	if len(url) > 0 && url[0] == '{' {
		kind = engine.OutputJSON
	}
	return engine.Result{Value: url, Kind: kind}, nil
}
