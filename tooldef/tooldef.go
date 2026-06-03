// Package tooldef provides JSON Schema tool definitions for AI agent frameworks.
//
// These definitions are compatible with OpenAI function-calling, Anthropic tool_use,
// Google Gemini function declarations, and any framework that accepts JSON Schema
// (LangChain, Vercel AI SDK, Semantic Kernel, etc.).
//
// Usage:
//
//	tools := tooldef.AllTools()
//	// Register with your agent framework's tool system
package tooldef

import (
	"fmt"
	"strings"
)

// ToolDef describes a callable tool with its JSON Schema parameters.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  Schema `json:"parameters"`
	Category    string `json:"category,omitempty"` // media type category: "image", "video", "audio", "3d", "music", "voice", "sfx"
}

// Schema is a minimal JSON Schema representation.
//
// OneOf encodes mutually-exclusive property groups. Each inner slice is a set
// of property names where exactly one MUST be provided. This is a pragmatic
// subset of JSON Schema's full `oneOf` (which takes sub-schemas) — sufficient
// for our XOR-style tool inputs (e.g. `prompt` vs `image_url` vs `image_urls`)
// without dragging in a full schema validator.
type Schema struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	Default     string            `json:"default,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	OneOf       [][]string        `json:"-"`
}

// ValidateParams checks parameter values against the tool's schema constraints (enums, required fields).
// Returns an error describing the first invalid parameter found, or nil if all values are valid.
func ValidateParams(def ToolDef, params map[string]interface{}) error {
	// Check required fields.
	for _, req := range def.Parameters.Required {
		v, ok := params[req]
		if !ok || v == nil {
			return fmt.Errorf("parameter %q is required", req)
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			return fmt.Errorf("parameter %q is required (got empty string)", req)
		}
	}

	// Check enum constraints.
	for name, prop := range def.Parameters.Properties {
		if len(prop.Enum) == 0 {
			continue
		}
		v, ok := params[name]
		if !ok || v == nil {
			continue // optional param not provided
		}
		s, ok := v.(string)
		if !ok {
			continue // non-string params skip enum check
		}
		valid := false
		for _, e := range prop.Enum {
			if strings.EqualFold(s, e) {
				params[name] = e
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("parameter %q value %q is not valid, must be one of: %s",
				name, s, strings.Join(prop.Enum, ", "))
		}
	}

	// Mutually-exclusive groups: exactly one property in each group must be provided.
	for _, group := range def.Parameters.OneOf {
		provided := make([]string, 0, len(group))
		for _, name := range group {
			if paramProvided(params[name]) {
				provided = append(provided, name)
			}
		}
		switch len(provided) {
		case 0:
			return fmt.Errorf("exactly one of [%s] must be provided", strings.Join(group, ", "))
		case 1:
			// ok
		default:
			return fmt.Errorf("parameters %s are mutually exclusive; provide exactly one of [%s]",
				strings.Join(provided, ", "), strings.Join(group, ", "))
		}
	}
	return nil
}

// paramProvided returns true when v is a meaningful, non-empty value.
// Empty strings, nil, and empty arrays count as "not provided" for OneOf checks.
func paramProvided(v interface{}) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) != ""
	case []interface{}:
		return len(t) > 0
	case []string:
		return len(t) > 0
	default:
		return true
	}
}

// AllTools returns every pre-defined tool aigo can provide.
func AllTools() []ToolDef {
	return []ToolDef{
		GenerateImage(),
		GenerateVideo(),
		Generate3D(),
		TextToSpeech(),
		DesignVoice(),
		EditImage(),
		EditVideo(),
		TranscribeAudio(),
		GenerateMusic(),
		GenerateSfx(),
		MixAudio(),
		UnderstandImage(),
		SearchMaterial(),
	}
}

// ToolsFor returns tools matching the given category (e.g., "image", "video", "audio", "3d", "music").
// Multiple categories can be passed; tools matching any of them are included.
func ToolsFor(categories ...string) []ToolDef {
	if len(categories) == 0 {
		return nil
	}
	want := make(map[string]bool, len(categories))
	for _, c := range categories {
		want[c] = true
	}
	var result []ToolDef
	for _, t := range AllTools() {
		if want[t.Category] {
			result = append(result, t)
		}
	}
	return result
}

// GenerateImage returns the tool definition for image generation.
func GenerateImage() ToolDef {
	return ToolDef{
		Name:        "generate_image",
		Description: "Generate an image from a text prompt. Returns a URL of the generated image. Write detailed prompts: describe subject, style, composition, lighting, and mood. Use negative_prompt to exclude unwanted elements.",
		Category:    "image",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Detailed description of the image. Structure as: subject → style/medium → composition/framing → lighting/mood → color palette. Be specific: 'a ceramic coffee mug on a marble countertop, soft studio lighting, warm tones' is better than 'a mug'.",
				},
				"negative_prompt": {
					Type:        "string",
					Description: "Elements to exclude from the image (e.g. 'blurry, watermark, text, low quality, extra fingers')",
				},
				"size": {
					Type:        "string",
					Description: "Image dimensions. Use 1024x1024 for square, 1536x1024 for landscape, 1024x1536 for portrait",
					Enum:        []string{"1024x1024", "1024x1536", "1536x1024", "512x512"},
					Default:     "1024x1024",
				},
				"aspect_ratio": {
					Type:        "string",
					Description: "Image aspect ratio. 16:9 for landscape/hero banners, 9:16 for mobile/portrait, 1:1 for avatar/square",
					Enum:        []string{"1:1", "3:4", "4:3", "16:9", "9:16"},
				},
				"resolution": {
					Type:        "string",
					Description: "Image resolution quality. Use 1K for drafts, 2K for standard, 4K for print/high-detail",
					Enum:        []string{"1K", "2K", "4K"},
					Default:     "2K",
				},
				"camera_angle": {
					Type:        "string",
					Description: "Camera angle preset for the shot",
					Enum:        []string{"front", "side", "back", "top-down", "low-angle", "high-angle", "45-degree", "close-up"},
				},
				"width": {
					Type:        "integer",
					Description: "Custom image width in pixels (use instead of size for non-standard dimensions)",
				},
				"height": {
					Type:        "integer",
					Description: "Custom image height in pixels (use instead of size for non-standard dimensions)",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// Generate3D returns the tool definition for 3D model generation.
//
// Inputs are mutually exclusive: provide one of `prompt`, `image_url`, or
// `image_urls` (multi-image, 2-4 entries). Tripo-style providers also accept
// `texture_quality` and `geometry_quality` for fidelity tuning.
func Generate3D() ToolDef {
	return ToolDef{
		Name:        "generate_3d",
		Description: "Generate a 3D model from a text prompt, a reference image, or 2-4 multi-view images. Returns a URL to the model file (GLB/FBX/OBJ/USDZ). Provide exactly one of: prompt (text-to-3D), image_url (single image), or image_urls (multi-view reconstruction).",
		Category:    "3d",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe the 3D object: shape, material, surface detail, and scale. 'a low-poly wooden treasure chest with iron hinges and a rusty lock' is better than 'a chest'. Keep under 1024 chars. Mutually exclusive with image_url / image_urls.",
				},
				"image_url": {
					Type:        "string",
					Description: "URL of a single reference image for image-to-3D. Use a clean, well-lit photo with the object centered on a simple background for best results",
				},
				"image_urls": {
					Type:        "array",
					Description: "URLs of 2-4 multi-view images of the same object (e.g. front, side, back, top) for high-precision 3D reconstruction",
					Items: &Schema{
						Type: "string",
					},
				},
				"mode": {
					Type:        "string",
					Description: "Generation quality mode",
					Enum:        []string{"preview", "refine"},
					Default:     "preview",
				},
				"art_style": {
					Type:        "string",
					Description: "Art style for the generated model",
					Enum:        []string{"realistic", "cartoon", "low-poly", "sculpture", "pbr"},
				},
				"negative_prompt": {
					Type:        "string",
					Description: "What to avoid in the generated model",
				},
				"topology": {
					Type:        "string",
					Description: "Mesh topology preference",
					Enum:        []string{"quad", "triangle"},
				},
				"target_polycount": {
					Type:        "integer",
					Description: "Target polygon count for the mesh",
				},
				"texture_quality": {
					Type:        "string",
					Description: "Texture detail level (Tripo). Omit to generate a no-texture mesh.",
					Enum:        []string{"standard", "detailed"},
				},
				"geometry_quality": {
					Type:        "string",
					Description: "Geometry precision level (Tripo H3.1 only). 'ultra' enables max-fidelity mesh up to 2M faces.",
					Enum:        []string{"standard", "ultra"},
				},
			},
			// Tripo and similar 3D engines accept exactly one input modality.
			OneOf: [][]string{{"prompt", "image_url", "image_urls"}},
		},
	}
}

// GenerateVideo returns the tool definition for video generation.
func GenerateVideo() ToolDef {
	return ToolDef{
		Name:        "generate_video",
		Description: "Generate a video from a text prompt. Returns a URL to the generated video. Supports text-to-video, image-to-video (provide reference_image), and video-to-video (provide reference_video). For best results with image-to-video, generate an image first with generate_image, then pass its URL as reference_image.",
		Category:    "video",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe the video scene: subject, action/motion, environment, camera movement, and mood. Be specific about motion: 'camera slowly pans right as leaves fall' is better than 'nature scene'.",
				},
				"duration": {
					Type:        "integer",
					Description: "Video duration in seconds. Must be between 3 and 15 (inclusive). Shorter durations produce higher quality. Default: 5",
				},
				"size": {
					Type:        "string",
					Description: "Video dimensions. 1280x720 for landscape, 720x1280 for portrait/mobile, 960x960 for square",
					Enum:        []string{"1280x720", "960x960", "720x1280", "1920x1080", "1080x1920"},
					Default:     "1280x720",
				},
				"aspect_ratio": {
					Type:        "string",
					Description: "Video aspect ratio. 16:9 for standard video, 9:16 for mobile/shorts, 1:1 for social media",
					Enum:        []string{"16:9", "9:16", "1:1", "4:3", "3:4"},
				},
				"resolution": {
					Type:        "string",
					Description: "Output resolution tier. '720P' produces ~720p equivalent (draft quality), '1080P' produces ~1080p equivalent (final quality). For 9:16 vertical video at full HD, use '1080P'. Actual pixel dimensions depend on the engine.",
					Enum:        []string{"480P", "720P", "1080P"},
					Default:     "720P",
				},
				"reference_image": {
					Type:        "string",
					Description: "URL of a reference image for image-to-video. Tip: generate a high-quality image first with generate_image, then animate it",
				},
				"reference_images": {
					Type:        "array",
					Description: "URLs of multiple reference images for multi-image-to-video (e.g. start and end frames)",
					Items: &Schema{
						Type: "string",
					},
				},
				"reference_video": {
					Type:        "string",
					Description: "URL of a source video for video-to-video transformation (style transfer, motion retargeting)",
				},
				"audio": {
					Type:        "boolean",
					Description: "Enable audio generation when the provider supports it",
				},
				"watermark": {
					Type:        "boolean",
					Description: "Enable watermark when the provider supports it",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// TextToSpeech returns the tool definition for text-to-speech synthesis.
func TextToSpeech() ToolDef {
	return ToolDef{
		Name:        "text_to_speech",
		Description: "Convert text to speech audio. Returns a URL of the audio. Use the instructions parameter to control speed, emotion, and delivery style (e.g. 'speak slowly with warm emotion').",
		Category:    "audio",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"text": {
					Type:        "string",
					Description: "The text to convert to speech. For long text, break into natural paragraphs. Punctuation affects pacing: commas add short pauses, periods add longer pauses.",
				},
				"voice": {
					Type:        "string",
					Description: "Voice identifier. Chinese female: Cherry (warm), Serena (bilingual zh/en), Bella (gentle), Momo (cute). Chinese male: Li (mature), Marcus (deep), Peter (standard). English female: Chelsie, Jennifer, Maia. English male: Ethan, Ryan, Aiden. IMPORTANT: Use Chinese voices for Chinese text.",
					Enum:        []string{"Cherry", "Serena", "Ethan", "Chelsie", "Momo", "Vivian", "Maia", "Bella", "Jennifer", "Katerina", "Mia", "Bellona", "Bunny", "Elias", "Nini", "Seren", "Stella", "Sonrisa", "Sohee", "Jada", "Sunny", "Kiki", "Moon", "Kai", "Nofish", "Ryan", "Aiden", "Mochi", "Vincent", "Neil", "Arthur", "Pip", "Bodega", "Alek", "Dolce", "Lenn", "Emilien", "Andre", "Dylan", "Li", "Marcus", "Roy", "Peter", "Eric", "Rocky"},
				},
				"language": {
					Type:        "string",
					Description: "Language code (e.g., zh, en)",
				},
				"speed": {
					Type:        "number",
					Description: "Speech rate multiplier. 0.5 = half speed (slow, dramatic), 1.0 = normal, 1.5 = fast. For narration/advertising voiceover use 0.7-0.8. Default: 1.0",
				},
				"instructions": {
					Type:        "string",
					Description: "Style instructions for the speech, e.g. 'speak slowly and clearly', 'with warm emotion', 'fast pace'. Controls speed, emotion, and delivery style",
				},
			},
			Required: []string{"text", "voice"},
		},
	}
}

// DesignVoice returns the tool definition for custom voice creation.
func DesignVoice() ToolDef {
	return ToolDef{
		Name:        "design_voice",
		Description: "Create a custom AI voice from a text description. Returns JSON with voice ID and optional preview audio.",
		Category:    "voice",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"voice_prompt": {
					Type:        "string",
					Description: "Describe the voice: gender, age range, tone, and speaking style. 'A warm, confident female voice in her 30s with a slight British accent' is better than 'female voice'.",
				},
				"preview_text": {
					Type:        "string",
					Description: "A short sentence for the voice to speak as a preview. Choose text that showcases the voice's range and character.",
				},
				"target_model": {
					Type:        "string",
					Description: "The TTS model this voice will be used with (default: provider's primary TTS model)",
					Default:     "qwen3-tts-flash",
				},
				"preferred_name": {
					Type:        "string",
					Description: "Preferred identifier name for the created voice",
				},
				"language": {
					Type:        "string",
					Description: "Language of the voice",
					Enum:        []string{"zh", "en", "ja", "ko"},
				},
			},
			Required: []string{"voice_prompt", "preview_text", "target_model"},
		},
	}
}

// EditImage returns the tool definition for image editing.
func EditImage() ToolDef {
	return ToolDef{
		Name:        "edit_image",
		Description: "Edit an existing image based on a text prompt. Returns a URL of the edited image. Use this for iterative refinement after generate_image, or to modify user-provided images.",
		Category:    "image",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe WHAT to change, not the whole image. Be specific: 'change the sky to sunset orange' is better than 'make it look nice'. For additions use 'add ...', for removals use 'remove ...'.",
				},
				"image_url": {
					Type:        "string",
					Description: "URL of the source image to edit. Tip: use a URL returned by generate_image for iterative refinement",
				},
				"size": {
					Type:        "string",
					Description: "Output image dimensions. Match the source aspect ratio to avoid distortion",
					Enum:        []string{"1024x1024", "1024x1536", "1536x1024"},
					Default:     "1024x1024",
				},
			},
			Required: []string{"prompt", "image_url"},
		},
	}
}

// EditVideo returns the tool definition for video editing.
func EditVideo() ToolDef {
	return ToolDef{
		Name:        "edit_video",
		Description: "Edit an existing video based on a text prompt. Returns a URL to the edited video. Optionally provide a reference_image for style guidance.",
		Category:    "video",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe the edit to apply. Focus on what changes: 'apply watercolor painting style' or 'replace background with ocean sunset'. Keep the original motion unless you want to change it.",
				},
				"video_url": {
					Type:        "string",
					Description: "URL of the source video to edit. Can be a URL returned by generate_video",
				},
				"reference_image": {
					Type:        "string",
					Description: "URL of a reference image for style transfer. The video adopts the visual style of this image while keeping its motion",
				},
				"size": {
					Type:        "string",
					Description: "Output video dimensions. Match the source aspect ratio to avoid distortion",
					Enum:        []string{"1280x720", "960x960", "720x1280", "1920x1080", "1080x1920"},
				},
				"duration": {
					Type:        "integer",
					Description: "Output video duration in seconds",
				},
			},
			Required: []string{"prompt", "video_url"},
		},
	}
}

// GenerateMusic returns the tool definition for AI music generation.
func GenerateMusic() ToolDef {
	return ToolDef{
		Name:        "generate_music",
		Description: "Generate music from a text prompt describing style/mood. Returns a URL of the audio. Use lyrics with section markers like [verse], [chorus], [bridge] for songs. Set is_instrumental=true for music without vocals.",
		Category:    "music",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe genre, mood, tempo, and instruments. Be specific: 'upbeat lo-fi hip-hop, jazzy piano chords, vinyl crackle, 85 BPM' is better than 'chill music'. Combine genre + mood + instrumentation for best results.",
				},
				"lyrics": {
					Type:        "string",
					Description: "Song lyrics with structure markers: [verse], [chorus], [bridge], [intro], [outro]. Each section on its own line. Leave empty or set is_instrumental=true for instrumental tracks.",
				},
				"is_instrumental": {
					Type:        "boolean",
					Description: "Set true for music without vocals. When true, lyrics are ignored",
				},
				"output_format": {
					Type:        "string",
					Description: "Output format for the generated audio",
					Enum:        []string{"url", "hex"},
					Default:     "url",
				},
				"sample_rate": {
					Type:        "integer",
					Description: "Audio sample rate in Hz (e.g., 44100)",
				},
				"format": {
					Type:        "string",
					Description: "Audio encoding format",
					Enum:        []string{"mp3", "wav", "flac"},
					Default:     "mp3",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// GenerateSfx returns the tool definition for sound effects generation.
func GenerateSfx() ToolDef {
	return ToolDef{
		Name:        "generate_sfx",
		Description: "Generate a sound effect from a text description. Returns a URL or data URI of the generated audio. Describe the sound precisely: 'heavy rain on a tin roof with distant thunder' is better than 'rain sound'.",
		Category:    "sfx",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Describe the sound effect: source, texture, intensity, and environment. Be specific: 'metallic clang of a sword hitting a shield in a stone hall' is better than 'sword sound'.",
				},
				"duration": {
					Type:        "integer",
					Description: "Duration of the sound effect in seconds",
				},
				"format": {
					Type:        "string",
					Description: "Output audio format",
					Enum:        []string{"mp3", "wav", "flac"},
					Default:     "mp3",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// MixAudio returns the tool definition for mixing multiple audio tracks.
func MixAudio() ToolDef {
	return ToolDef{
		Name:        "mix_audio",
		Description: "Mix multiple audio tracks into a single output. Accepts 2 or more audio URLs with optional per-track volume and delay controls. Use this to combine background music, sound effects, and voice narration into a final mix.",
		Category:    "audio",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"audio_urls": {
					Type:        "array",
					Description: "URLs of audio files to mix (minimum 2). The first track is the primary/base track",
					Items: &Schema{
						Type: "string",
					},
				},
				"volumes": {
					Type:        "array",
					Description: "Per-track volume levels (0.0 to 1.0). Index matches audio_urls. Defaults to 1.0 for all tracks",
					Items: &Schema{
						Type: "number",
					},
				},
				"delays": {
					Type:        "array",
					Description: "Per-track delay in milliseconds before playback starts. Index matches audio_urls. Defaults to 0 for all tracks",
					Items: &Schema{
						Type: "integer",
					},
				},
				"duration": {
					Type:        "integer",
					Description: "Maximum output duration in seconds. If omitted, uses the longest track's duration",
				},
				"format": {
					Type:        "string",
					Description: "Output audio format",
					Enum:        []string{"mp3", "wav", "flac"},
					Default:     "mp3",
				},
			},
			Required: []string{"audio_urls"},
		},
	}
}

// UnderstandImage returns the tool definition for image understanding / vision analysis.
func UnderstandImage() ToolDef {
	return ToolDef{
		Name:        "understand_image",
		Description: "Analyze and describe an image using a vision-capable model. Returns a text description of the image content. Use this to understand images when the primary model does not support vision input.",
		Category:    "vision",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"image_url": {
					Type:        "string",
					Description: "URL of the image to analyze, or a base64 data URI (data:image/...;base64,...)",
				},
				"prompt": {
					Type:        "string",
					Description: "Instructions for the analysis. Default: describe the image in detail",
					Default:     "Describe this image in detail. Include: scene description, visible objects, any text, people and their actions, lighting/mood, and any notable elements.",
				},
			},
			Required: []string{"image_url"},
		},
	}
}

// SearchMaterial returns the tool definition for material/asset search.
func SearchMaterial() ToolDef {
	return ToolDef{
		Name:        "search_material",
		Description: "Search for stock photos, videos, audio, and other creative assets from multiple platforms (Pexels, Unsplash, Pixabay) or your own asset library. Returns a list of matching items with preview URLs and metadata. Use specific keywords and media_type to narrow results.",
		Category:    "search",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"query": {
					Type:        "string",
					Description: "Search keywords or natural language description of the desired material. Be specific: 'aerial view of snow-covered forest at sunset' is better than 'forest'.",
				},
				"media_type": {
					Type:        "string",
					Description: "Type of material to search for",
					Enum:        []string{"image", "video", "audio", "document"},
				},
				"source": {
					Type:        "string",
					Description: "Which platform to search. Use 'all' to search everywhere, or pick a specific source for faster results",
					Enum:        []string{"all", "pexels", "unsplash", "pixabay", "oss", "local"},
					Default:     "all",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results to return (1-100). Default: 20",
				},
				"tags": {
					Type:        "string",
					Description: "Comma-separated tags to filter by (e.g. 'nature,landscape,sunset')",
				},
				"sort": {
					Type:        "string",
					Description: "How to sort results",
					Enum:        []string{"relevance", "newest", "popular"},
					Default:     "relevance",
				},
			},
			Required: []string{"query"},
		},
	}
}

// TranscribeAudio returns the tool definition for audio transcription.
func TranscribeAudio() ToolDef {
	return ToolDef{
		Name:        "transcribe_audio",
		Description: "Transcribe audio to text using speech recognition. Returns the transcription text or JSON.",
		Category:    "audio",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"audio_url": {
					Type:        "string",
					Description: "URL of the audio file to transcribe. Supports common formats: mp3, wav, flac, m4a, webm",
				},
				"language": {
					Type:        "string",
					Description: "Language code of the audio (e.g., en, zh, ja). Providing the correct language improves accuracy",
				},
				"response_format": {
					Type:        "string",
					Description: "Output format. Use 'text' for plain transcript, 'srt'/'vtt' for subtitles with timestamps, 'json' for structured data",
					Enum:        []string{"json", "text", "srt", "vtt"},
				},
			},
			Required: []string{"audio_url"},
		},
	}
}
