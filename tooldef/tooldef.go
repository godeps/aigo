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
}

// Schema is a minimal JSON Schema representation.
type Schema struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	Default     string            `json:"default,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
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
			if s == e {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("parameter %q value %q is not valid, must be one of: %s",
				name, s, strings.Join(prop.Enum, ", "))
		}
	}
	return nil
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
					Default:     "1024x1024",
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

// Generate3D returns the tool definition for 3D model generation.
func Generate3D() ToolDef {
	return ToolDef{
		Name:        "generate_3d",
		Description: "Generate a 3D model from a text prompt or reference image. Returns a URL to the generated model file (GLB, FBX, OBJ, or USDZ).",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Text description of the 3D model to generate",
				},
				"image_url": {
					Type:        "string",
					Description: "URL of a reference image for image-to-3D generation",
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
					Description: "Video dimensions. Use '*' as separator (not 'x')",
					Enum:        []string{"1280*720", "960*960", "720*1280", "1920*1080", "1080*1920"},
					Default:     "1280*720",
				},
				"aspect_ratio": {
					Type:        "string",
					Description: "Video aspect ratio",
					Enum:        []string{"16:9", "9:16", "1:1", "4:3", "3:4"},
				},
				"resolution": {
					Type:        "string",
					Description: "Video resolution quality",
					Enum:        []string{"480P", "720P", "1080P"},
					Default:     "720P",
				},
				"reference_image": {
					Type:        "string",
					Description: "URL of a reference image for image-to-video generation",
				},
				"reference_images": {
					Type:        "array",
					Description: "URLs of reference images for multi-image-to-video generation",
					Items: &Schema{
						Type: "string",
					},
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
					Description: "Voice to use. Cherry: female Chinese, Serena: female Chinese/English, Ethan: male English, Chelsie: female English",
					Enum:        []string{"Cherry", "Serena", "Ethan", "Chelsie"},
				},
				"language": {
					Type:        "string",
					Description: "Language code (e.g., zh, en)",
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
					Enum:        []string{"qwen3-tts-flash", "qwen3-tts-instruct-flash"},
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
		Description: "Edit an existing video based on a text prompt, optionally with reference images. Returns a URL to the edited video.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Text description of the desired edit",
				},
				"video_url": {
					Type:        "string",
					Description: "URL of the source video to edit",
				},
				"reference_image": {
					Type:        "string",
					Description: "URL of a reference image for style guidance (optional)",
				},
				"size": {
					Type:        "string",
					Description: "Output video dimensions. Use '*' as separator",
					Enum:        []string{"1280*720", "960*960", "720*1280", "1920*1080", "1080*1920"},
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
		Description: "Generate music from a text prompt describing style/mood, optionally with lyrics. Returns a URL or audio data.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"prompt": {
					Type:        "string",
					Description: "Music style and mood description (e.g., 'indie folk, melancholy, introspective')",
				},
				"lyrics": {
					Type:        "string",
					Description: "Song lyrics with section markers like [verse], [chorus], [bridge]",
				},
				"is_instrumental": {
					Type:        "boolean",
					Description: "Generate instrumental music without vocals",
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
