package newapi

import (
	"strings"

	"github.com/godeps/aigo/engine"
)

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
	// ═══ Image Generation (/v1/images/generations) ═══════════

	// OpenAI
	"gpt-image-2":          {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"gpt-image-1":          {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"gpt-image-1-mini":     {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"gpt-image-1.5":        {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"chatgpt-image-latest": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"dall-e-3":             {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"dall-e-2":             {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// Qwen / Ali (sync image models, routed via /v1/images/generations on new-api)
	"qwen-image": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"z-image":    {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// Qwen images (Qwen-specific request body via new-api)
	"qwen-max-vl": {route: RouteQwenImagesGenerations, kind: KindImage, cap: "image"},

	// Google Imagen (Gemini / Vertex AI)
	"imagen-4.0-generate-001":       {route: RouteGeminiGenerateContent, kind: KindImage, cap: "image"},
	"imagen-4.0-ultra-generate-001": {route: RouteGeminiGenerateContent, kind: KindImage, cap: "image"},
	"imagen-4.0-fast-generate-001":  {route: RouteGeminiGenerateContent, kind: KindImage, cap: "image"},

	// Gemini native
	"gemini-2.0-flash": {route: RouteGeminiGenerateContent, kind: KindImage, cap: "image"},

	// MiniMax image
	"image-01":      {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"image-01-live": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// Jimeng image
	"jimeng_high_aes_general_v21_L": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// Replicate
	"black-forest-labs/flux-1.1-pro": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// SiliconFlow
	"InstantX/InstantID":               {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"ByteDance/SDXL-Lightning":         {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"black-forest-labs/FLUX.1-schnell": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// ═══ Image Edit (/v1/images/edits) ═══════════════════════

	"qwen-image-edit":                 {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-max":             {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-max-2026-01-16":  {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus":            {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus-2025-12-15": {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus-2025-10-30": {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},

	// ═══ Video Generation ════════════════════════════════════

	// Kling (provider-specific routes via /kling/v1/videos/*)
	"kling-v1":        {route: RouteKlingText2Video, kind: KindVideo, cap: "video"},
	"kling-v1-6":      {route: RouteKlingImage2Video, kind: KindVideo, cap: "video"},
	"kling-v1-6-pro":  {route: RouteKlingImage2Video, kind: KindVideo, cap: "video"},
	"kling-v2-master": {route: RouteKlingText2Video, kind: KindVideo, cap: "video"},

	// Jimeng video (Action+Version route via /jimeng/)
	"jimeng-2.1-pro":            {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_vgfm_t2v_l20":       {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_v30":                {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_v30_pro":            {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_t2v_v30":            {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_i2v_first_v30":      {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_i2v_first_tail_v30": {route: RouteJimengVideo, kind: KindVideo, cap: "video"},
	"jimeng_ti2v_v30_pro":       {route: RouteJimengVideo, kind: KindVideo, cap: "video"},

	// Sora (OpenAI multipart /v1/videos)
	"sora":       {route: RouteSoraVideos, kind: KindVideo, cap: "video"},
	"sora-2":     {route: RouteSoraVideos, kind: KindVideo, cap: "video"},
	"sora-2-pro": {route: RouteSoraVideos, kind: KindVideo, cap: "video"},

	// Doubao Seedance (OpenAI-compatible /v1/video/generations)
	"doubao-seedance-1-0-pro-250528":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"doubao-seedance-1-0-lite-t2v":    {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"doubao-seedance-1-0-lite-i2v":    {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"doubao-seedance-1-5-pro-251215":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"doubao-seedance-2-0-260128":      {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"doubao-seedance-2-0-fast-260128": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},

	// Ali Wan video (via /v1/video/generations on new-api)
	"wan2.5-i2v-preview": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.5-t2v-preview": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.6-i2v":         {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.2-i2v-flash":   {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.2-i2v-plus":    {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.2-t2v-plus":    {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.2-kf2v-flash":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wan2.2-s2v":         {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wanx2.1-i2v-plus":   {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"wanx2.1-i2v-turbo":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},

	// Vidu (via /v1/video/generations on new-api)
	"viduq2":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"viduq1":  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"vidu2.0": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"vidu1.5": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},

	// Hailuo / MiniMax video
	"MiniMax-Hailuo-2.3":      {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"MiniMax-Hailuo-2.3-Fast": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"MiniMax-Hailuo-02":       {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"T2V-01-Director":         {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"T2V-01":                  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"I2V-01-Director":         {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"I2V-01-live":             {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"I2V-01":                  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"S2V-01":                  {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},

	// Google Veo (Vertex AI / Gemini)
	"veo-2.0-generate-001":          {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"veo-3.0-generate-001":          {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"veo-3.0-fast-generate-001":     {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"veo-3.1-generate-preview":      {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	"veo-3.1-fast-generate-preview": {route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},

	// ═══ Vision Understanding (/v1/chat/completions) ════════

	// Qwen vision & omni (DashScope OpenAI-compatible)
	"qwen3.7-plus":      {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"qwen3.6-plus":      {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"qwen3.6-flash":     {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"qwen3.5-omni-plus": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// GLM-4V (ZhiPu)
	"glm-4v":      {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"glm-4v-plus": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"glm-4.6v":    {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// Yi Vision (Lingyiwanwu)
	"yi-vision":  {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"yi-vl-plus": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// Grok Vision (xAI)
	"grok-2-vision-1212": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"grok-2-vision":      {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"grok-vision-beta":   {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// Doubao Vision (ByteDance)
	"Doubao-vision-lite-32k":    {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"Doubao-vision-pro-32k":     {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"Doubao-1.5-pro-vision-32k": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// Step Vision (StepFun)
	"step-1v-8k":     {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"step-1.5v-mini": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// GPT-4V (OpenAI legacy vision)
	"gpt-4-vision-preview":      {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	"gpt-4-1106-vision-preview": {route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// ═══ TTS (/v1/audio/speech) ══════════════════════════════

	"tts-1":    {route: RouteOpenAISpeech, kind: KindSpeech, cap: "tts"},
	"tts-1-hd": {route: RouteOpenAISpeech, kind: KindSpeech, cap: "tts"},

	// ═══ ASR (/v1/audio/transcriptions) ══════════════════════

	"whisper-1":        {route: RouteOpenAITranscriptions, kind: KindSpeech, cap: "asr"},
	"whisper-large-v3": {route: RouteOpenAITranscriptions, kind: KindSpeech, cap: "asr"},

	// ═══ Music ═══════════════════════════════════════════════

	"suno_music":  {route: RouteAuto, kind: "", cap: "music"},
	"suno_lyrics": {route: RouteAuto, kind: "", cap: "music"},
}

// ConfigSchema returns the configuration fields required by the NewAPI engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "NEWAPI_API_KEY", Description: "NewAPI API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "NEWAPI_BASE_URL", Description: "Custom API base URL (optional)"},
		{Key: "model", Label: "Model", Type: "string", Description: "Model name. Known models are routed automatically; custom gateway model names should also set capability."},
		{Key: "capability", Label: "Capability", Type: "string", Description: "Route hint for custom models: image, image_edit, video, tts, asr, video_understanding, or vision."},
		{Key: "quality", Label: "Quality", Type: "string", Description: "Image quality tier for OpenAI-compatible image models, e.g. low, medium, high, auto, standard, or hd."},
		{Key: "style", Label: "Style", Type: "string", Description: "Image style for models that support it, e.g. vivid or natural."},
		{Key: "background", Label: "Background", Type: "string", Description: "gpt-image-* background mode: transparent, opaque, or auto."},
		{Key: "output_format", Label: "Output Format", Type: "string", Description: "gpt-image-* output format: png, jpeg, or webp."},
		{Key: "moderation", Label: "Moderation", Type: "string", Description: "gpt-image-* moderation mode, e.g. low or auto."},
		{Key: "output_compression", Label: "Output Compression", Type: "number", Description: "gpt-image-* JPEG/WebP compression from 0 to 100."},
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

// LookupRoute resolves the Route, MediaKind, and capability for a model using
// a three-tier decision chain:
//
//  1. knownModels exact match (highest priority)
//  2. Model name pattern inference (prefix/substring heuristics)
//  3. Capability fallback from EngineConfig.Capability
//
// Returns (RouteAuto, "", "") when no tier matches — the caller must handle this.
func LookupRoute(model string, capability string) (Route, MediaKind, string) {
	// Tier 1: exact match in knownModels
	if entry, ok := knownModels[model]; ok {
		return entry.route, entry.kind, entry.cap
	}

	// Tier 2: model name pattern inference
	if route, kind, cap := InferFromModelName(model); cap != "" {
		return route, kind, cap
	}

	// Tier 3: capability fallback
	if capability != "" {
		kind, route := capToKindAndRoute(capability)
		return route, kind, capability
	}

	return RouteAuto, "", ""
}

// InferFromModelName attempts to determine the capability, route, and kind
// from a model name using prefix/substring heuristics.
func InferFromModelName(model string) (Route, MediaKind, string) {
	lower := strings.ToLower(model)

	for _, rule := range inferRules {
		if rule.match(lower) {
			return rule.route, rule.kind, rule.cap
		}
	}

	return RouteAuto, "", ""
}

type inferRule struct {
	match func(string) bool
	route Route
	kind  MediaKind
	cap   string
}

var inferRules = []inferRule{
	// Vision understanding patterns — before video to avoid false positives on "vl"
	{match: func(s string) bool {
		return strings.Contains(s, "-vl-") || strings.Contains(s, "-vl") || strings.HasSuffix(s, "-vl")
	}, route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	// Omni models (e.g. qwen3.5-omni-plus) — multimodal understanding via chat completions
	{match: func(s string) bool {
		return strings.Contains(s, "-omni") && !strings.Contains(s, "video-generation")
	}, route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},
	{match: func(s string) bool {
		if strings.Contains(s, "-vision") {
			return true
		}
		// GLM-4V style: "glm-4v", "glm-4.6v", "step-1v", "step-1.5v"
		// Match [- or .] + digit + "v" at end or before dash.
		// Excludes "i2v", "t2v", "s2v" where a letter precedes the digit.
		for i := 2; i < len(s); i++ {
			if s[i] == 'v' && s[i-1] >= '0' && s[i-1] <= '9' &&
				(s[i-2] == '-' || s[i-2] == '.') &&
				(i+1 >= len(s) || s[i+1] == '-') {
				return true
			}
		}
		return false
	}, route: RouteChatCompletions, kind: KindVision, cap: "video_understanding"},

	// Video patterns — ordered by specificity
	{match: func(s string) bool { return strings.Contains(s, "t2v") || strings.Contains(s, "text2video") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.Contains(s, "i2v") || strings.Contains(s, "image2video") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.Contains(s, "s2v") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.Contains(s, "seedance") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.HasPrefix(s, "vidu") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.Contains(s, "hailuo") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.HasPrefix(s, "veo-") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.HasPrefix(s, "sora") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	{match: func(s string) bool { return strings.Contains(s, "video") && !strings.Contains(s, "video2") }, route: RouteOpenAIVideoGenerations, kind: KindVideo, cap: "video"},
	// TTS patterns
	{match: func(s string) bool { return strings.Contains(s, "tts") || strings.Contains(s, "speech") }, route: RouteOpenAISpeech, kind: KindSpeech, cap: "tts"},
	// ASR patterns
	{match: func(s string) bool {
		return strings.Contains(s, "whisper") || strings.Contains(s, "asr") || strings.Contains(s, "transcri")
	}, route: RouteOpenAITranscriptions, kind: KindSpeech, cap: "asr"},
	// Music patterns
	{match: func(s string) bool { return strings.HasPrefix(s, "suno") }, route: RouteAuto, kind: "", cap: "music"},
	// Image patterns — must be AFTER video/tts/asr to avoid false positives
	// t2i = text-to-image (wan2.6-t2i-*, wan2.7-t2i-*)
	{match: func(s string) bool { return strings.Contains(s, "t2i") }, route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	{match: func(s string) bool {
		return strings.Contains(s, "dall-e") ||
			strings.HasPrefix(s, "gpt-image") ||
			strings.HasPrefix(s, "imagen-") ||
			strings.Contains(s, "flux-") ||
			strings.HasPrefix(s, "flux.1-") ||
			strings.Contains(s, "sdxl") ||
			strings.Contains(s, "stable-diffusion") ||
			strings.Contains(s, "qwen-image") ||
			strings.HasPrefix(s, "z-image") ||
			strings.Contains(s, "image")
	}, route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
}

// capToKindAndRoute maps a capability string to the default MediaKind and Route.
func capToKindAndRoute(capability string) (MediaKind, Route) {
	switch capability {
	case "image", "image_edit":
		return KindImage, RouteOpenAIImagesGenerations
	case "video", "video_edit":
		return KindVideo, RouteOpenAIVideoGenerations
	case "tts":
		return KindSpeech, RouteOpenAISpeech
	case "asr":
		return KindSpeech, RouteOpenAITranscriptions
	case "video_understanding", "vision":
		return KindVision, RouteChatCompletions
	default:
		return KindImage, RouteOpenAIImagesGenerations
	}
}
