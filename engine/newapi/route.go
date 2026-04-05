package newapi

// Route 指定调用的网关路径族；为空时由 MediaKind 推导默认 OpenAI 兼容接口。
type Route string

const (
	RouteAuto Route = ""

	// OpenAI 兼容（New API 统一路径）
	RouteOpenAIImagesGenerations Route = "openai_images_generations" // POST /v1/images/generations
	RouteOpenAIImagesEdits       Route = "openai_images_edits"       // POST /v1/images/edits (multipart)
	RouteOpenAIVideoGenerations  Route = "openai_video_generations"  // POST/GET /v1/video/generations
	RouteOpenAISpeech            Route = "openai_audio_speech"       // POST /v1/audio/speech
	RouteOpenAITranscriptions    Route = "openai_audio_transcriptions"
	RouteOpenAITranslations      Route = "openai_audio_translations"

	// 可灵 Kling
	RouteKlingText2Video  Route = "kling_text2video"
	RouteKlingImage2Video Route = "kling_image2video"

	// 即梦（火山系 Action+Version）
	RouteJimengVideo Route = "jimeng_video"

	// Sora（OpenAI 视频：multipart 创建 + 轮询 + /content 取流）
	RouteSoraVideos Route = "sora_v1_videos"

	// 通义千问图像（同路径 /v1/images/generations，请求体为 input.messages）
	RouteQwenImagesGenerations Route = "qwen_images_generations"

	// Gemini 原生 generateContent（图/音等）
	RouteGeminiGenerateContent Route = "gemini_generate_content"
)

func defaultRouteForKind(k MediaKind) Route {
	switch k {
	case KindImage:
		return RouteOpenAIImagesGenerations
	case KindVideo:
		return RouteOpenAIVideoGenerations
	case KindSpeech:
		return RouteOpenAISpeech
	default:
		return RouteOpenAIImagesGenerations
	}
}
