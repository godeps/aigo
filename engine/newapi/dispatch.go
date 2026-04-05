package newapi

import (
	"context"
	"fmt"

	"github.com/godeps/aigo/workflow"
)

type routeExec func(*Engine, context.Context, string, workflow.Graph) (string, error)

var routeTable = map[Route]routeExec{
	RouteOpenAIImagesGenerations: (*Engine).runOpenAIImageGenerations,
	RouteOpenAIImagesEdits:       (*Engine).runOpenAIImageEdits,
	RouteOpenAIVideoGenerations:  (*Engine).runOpenAIVideoGenerations,
	RouteOpenAISpeech:            (*Engine).runOpenAISpeech,
	RouteOpenAITranscriptions: func(e *Engine, ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
		return e.runOpenAIWhisper(ctx, apiKey, g, "/v1/audio/transcriptions")
	},
	RouteOpenAITranslations: func(e *Engine, ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
		return e.runOpenAIWhisper(ctx, apiKey, g, "/v1/audio/translations")
	},
	RouteKlingText2Video: func(e *Engine, ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
		return e.runKlingVideo(ctx, apiKey, g, "/kling/v1/videos/text2video")
	},
	RouteKlingImage2Video: func(e *Engine, ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
		return e.runKlingVideo(ctx, apiKey, g, "/kling/v1/videos/image2video")
	},
	RouteJimengVideo:            (*Engine).runJimengVideo,
	RouteSoraVideos:             (*Engine).runSoraVideo,
	RouteQwenImagesGenerations:  (*Engine).runQwenImageGenerations,
	RouteGeminiGenerateContent:  (*Engine).runGeminiGenerateContent,
}

func (e *Engine) dispatch(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	r := e.effectiveRoute()
	fn, ok := routeTable[r]
	if !ok {
		return "", fmt.Errorf("newapi: unknown Route %q", r)
	}
	return fn(e, ctx, apiKey, g)
}
