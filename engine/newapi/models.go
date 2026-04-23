package newapi

import "github.com/godeps/aigo/engine"

// knownModel describes a model the SDK knows how to handle via a specific route.
type knownModel struct {
	route Route
	kind  MediaKind
	cap   string // capability key: "image", "video", "tts"
}

// knownModels lists models that have been validated against newapi routes.
// This serves as a static catalog for auto-discovery; the gateway may
// support additional models not listed here.
var knownModels = map[string]knownModel{
	// OpenAI compatible images
	"gpt-image-2": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// Kling via newapi gateway
	"kling-v2-master": {route: RouteKlingText2Video, kind: KindVideo, cap: "video"},
	"kling-v1-6-pro":  {route: RouteKlingImage2Video, kind: KindVideo, cap: "video"},

	// Jimeng (Volcengine)
	"jimeng-2.1-pro": {route: RouteJimengVideo, kind: KindVideo, cap: "video"},

	// Sora
	"sora": {route: RouteSoraVideos, kind: KindVideo, cap: "video"},

	// Qwen images via newapi
	"qwen-max-vl": {route: RouteQwenImagesGenerations, kind: KindImage, cap: "image"},

	// Gemini
	"gemini-2.0-flash": {route: RouteGeminiGenerateContent, kind: KindImage, cap: "image"},

	// OpenAI TTS
	"tts-1":    {route: RouteOpenAISpeech, kind: KindSpeech, cap: "tts"},
	"tts-1-hd": {route: RouteOpenAISpeech, kind: KindSpeech, cap: "tts"},

	// OpenAI ASR (Whisper)
	"whisper-1":        {route: RouteOpenAITranscriptions, kind: KindSpeech, cap: "asr"},
	"whisper-large-v3": {route: RouteOpenAITranscriptions, kind: KindSpeech, cap: "asr"},
}

// ConfigSchema returns the configuration fields required by the NewAPI engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "NEWAPI_API_KEY", Description: "NewAPI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "NEWAPI_BASE_URL", Description: "Custom API base URL (optional)"},
	}
}

// ModelsByCapability returns all known newapi models grouped by capability.
func ModelsByCapability() map[string][]string {
	result := map[string][]string{}
	for model, entry := range knownModels {
		result[entry.cap] = append(result[entry.cap], model)
	}
	return result
}

// LookupRoute returns the Route and MediaKind for a known model.
// Returns RouteAuto and KindImage if the model is not in the catalog.
func LookupRoute(model string) (Route, MediaKind) {
	if entry, ok := knownModels[model]; ok {
		return entry.route, entry.kind
	}
	return RouteAuto, KindImage
}
