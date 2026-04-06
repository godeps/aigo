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

// ToolDef describes a callable tool with its JSON Schema parameters.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  Schema `json:"parameters"`
}

// Schema is a minimal JSON Schema representation.
type Schema struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
}

// AllTools returns every pre-defined tool aigo can provide.
func AllTools() []ToolDef {
	return []ToolDef{
		GenerateImage(),
		GenerateVideo(),
		TextToSpeech(),
		DesignVoice(),
		EditImage(),
		TranscribeAudio(),
	}
}

// GenerateImage returns the tool definition for image generation.
func GenerateImage() ToolDef {
	return ToolDef{
		Name:        "generate_image",
		Description: "Generate an image from a text prompt using AI. Returns a URL or data URI of the generated image.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Text description of the image to generate",
				},
				"negative_prompt": {
					Type:        "string",
					Description: "What to avoid in the generated image",
				},
				"size": {
					Type:        "string",
					Description: "Image dimensions (e.g., 1024x1024, 1024x1536)",
					Enum:        []string{"1024x1024", "1024x1536", "1536x1024", "512x512"},
				},
				"width": {
					Type:        "integer",
					Description: "Image width in pixels",
				},
				"height": {
					Type:        "integer",
					Description: "Image height in pixels",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

// GenerateVideo returns the tool definition for video generation.
func GenerateVideo() ToolDef {
	return ToolDef{
		Name:        "generate_video",
		Description: "Generate a video from a text prompt, optionally with reference images or videos. Supports text-to-video, image-to-video, and video-to-video modes. Returns a URL to the generated video.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Text description of the video to generate",
				},
				"duration": {
					Type:        "integer",
					Description: "Video duration in seconds",
				},
				"size": {
					Type:        "string",
					Description: "Video dimensions (e.g., 1280x720, 1920x1080)",
				},
				"aspect_ratio": {
					Type:        "string",
					Description: "Video aspect ratio (e.g., 16:9, 9:16, 1:1, 4:3, 3:4)",
				},
				"resolution": {
					Type:        "string",
					Description: "Video resolution quality",
					Enum:        []string{"480P", "720P", "1080P"},
				},
				"reference_image": {
					Type:        "string",
					Description: "URL of a reference image for image-to-video generation",
				},
				"reference_video": {
					Type:        "string",
					Description: "URL of a reference video for video-to-video generation",
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
		Description: "Convert text to speech audio. Returns a URL or data URI of the audio.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"text": {
					Type:        "string",
					Description: "The text to convert to speech",
				},
				"voice": {
					Type:        "string",
					Description: "Voice identifier to use for synthesis",
				},
				"language": {
					Type:        "string",
					Description: "Language code (e.g., zh, en)",
				},
				"instructions": {
					Type:        "string",
					Description: "Style instructions for the speech (e.g., speak slowly, with emotion)",
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
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"voice_prompt": {
					Type:        "string",
					Description: "Natural language description of the desired voice characteristics",
				},
				"preview_text": {
					Type:        "string",
					Description: "Sample text for the voice to speak as a preview",
				},
				"target_model": {
					Type:        "string",
					Description: "The TTS model this voice will be used with",
				},
				"preferred_name": {
					Type:        "string",
					Description: "Preferred identifier name for the created voice",
				},
				"language": {
					Type:        "string",
					Description: "Language code for the voice",
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
		Description: "Edit an existing image based on a text prompt. Returns a URL or data URI of the edited image.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Text description of the desired edit",
				},
				"image_url": {
					Type:        "string",
					Description: "URL of the source image to edit",
				},
				"size": {
					Type:        "string",
					Description: "Output image dimensions",
					Enum:        []string{"1024x1024", "1024x1536", "1536x1024"},
				},
			},
			Required: []string{"prompt", "image_url"},
		},
	}
}

// TranscribeAudio returns the tool definition for audio transcription.
func TranscribeAudio() ToolDef {
	return ToolDef{
		Name:        "transcribe_audio",
		Description: "Transcribe audio to text using speech recognition. Returns the transcription text or JSON.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"audio_url": {
					Type:        "string",
					Description: "URL of the audio file to transcribe",
				},
				"language": {
					Type:        "string",
					Description: "Language code of the audio (e.g., en, zh)",
				},
				"response_format": {
					Type:        "string",
					Description: "Output format",
					Enum:        []string{"json", "text", "srt", "vtt"},
				},
			},
			Required: []string{"audio_url"},
		},
	}
}
