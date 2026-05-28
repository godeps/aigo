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
	"InstantX/InstantID":             {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"ByteDance/SDXL-Lightning":       {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},
	"black-forest-labs/FLUX.1-schnell": {route: RouteOpenAIImagesGenerations, kind: KindImage, cap: "image"},

	// ═══ Image Edit (/v1/images/edits) ═══════════════════════

	"qwen-image-edit":                   {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-max":               {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-max-2026-01-16":    {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus":              {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus-2025-12-15":   {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},
	"qwen-image-edit-plus-2025-10-30":   {route: RouteOpenAIImagesEdits, kind: KindImage, cap: "image_edit"},

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
	default:
		return KindImage, RouteOpenAIImagesGenerations
	}
}
