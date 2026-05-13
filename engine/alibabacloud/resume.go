package alibabacloud

import (
	"context"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/alibabacloud/internal/async"
)

// extractorForModel returns the URLExtractor for a given model name.
// The paths mirror those used by each handler in imggen/vidgen/audiogen.
//
// Sync-only handlers (TTS, VoiceDesign) do not submit async tasks and never
// reach Resume; we still register an empty extractor so the table is exhaustive
// and intent is explicit.
func extractorForModel(model string) async.URLExtractor {
	switch model {
	case ModelQwenImage, ModelQwenImage2, ModelQwenImageEditPlus,
		ModelWanImage, ModelZImageTurbo:
		return async.URLExtractor{URLFields: [][]string{{"results", "url"}, {"result_url"}}}
	case ModelQwenASRFlash, ModelQwenASRFlashFiletrans:
		return async.URLExtractor{URLFields: [][]string{{"results", "transcription_url"}, {"results", "text"}}}
	case ModelTripoP1, ModelTripoH31:
		return async.URLExtractor{URLFields: [][]string{{"results", "pbr_model_url"}}}
	case ModelWanTextToVideo, ModelWanImageToVideo, ModelWanReferenceVideo, ModelWanVideoEdit,
		ModelKlingV3Video, ModelKlingV3OmniVideo,
		ModelHappyHorseT2V, ModelHappyHorseI2V, ModelHappyHorseR2V, ModelHappyHorseVideoEdit:
		return async.URLExtractor{URLFields: [][]string{{"video_url"}}}
	case ModelQwenTTSFlash, ModelQwenTTSInstructFlash, ModelQwenVoiceDesign, ModelFunMusic:
		// Synchronous handlers: never reach Resume, but keep the case explicit
		// so adding a new sync model surfaces in code review.
		return async.URLExtractor{}
	default:
		// Conservative fallback: video_url is the most common shape.
		return async.URLExtractor{URLFields: [][]string{{"video_url"}}}
	}
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	apiKey, err := engine.ResolveKey(e.apiKey, "DASHSCOPE_API_KEY")
	if err != nil {
		return engine.Result{}, err
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
