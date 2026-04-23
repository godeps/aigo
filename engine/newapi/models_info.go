package newapi

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all known NewAPI gateway models.
// Model names are prefixed with "newapi/" to avoid conflicts with native engine registrations.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "newapi/gpt-image-2",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "GPT Image 2 (NewAPI)", "zh": "GPT Image 2 (NewAPI)"},
			Description: engine.DisplayName{"en": "OpenAI gpt-image-2 image generation via NewAPI", "zh": "通过 NewAPI 调用 OpenAI gpt-image-2 图片生成"},
			Capability:  "image",
		},
		{
			Name:        "newapi/kling-v2-master",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Kling V2 Master (NewAPI)", "zh": "可灵 V2 大师版 (NewAPI)"},
			Description: engine.DisplayName{"en": "Kling text-to-video via NewAPI gateway", "zh": "通过 NewAPI 网关调用可灵文生视频"},
			Capability:  "video",
		},
		{
			Name:        "newapi/kling-v1-6-pro",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Kling V1.6 Pro (NewAPI)", "zh": "可灵 V1.6 专业版 (NewAPI)"},
			Description: engine.DisplayName{"en": "Kling image-to-video via NewAPI gateway", "zh": "通过 NewAPI 网关调用可灵图生视频"},
			Capability:  "video",
		},
		{
			Name:        "newapi/jimeng-2.1-pro",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Jimeng 2.1 Pro (NewAPI)", "zh": "即梦 2.1 专业版 (NewAPI)"},
			Description: engine.DisplayName{"en": "Jimeng video generation via NewAPI gateway", "zh": "通过 NewAPI 网关调用即梦视频生成"},
			Capability:  "video",
		},
		{
			Name:        "newapi/sora",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Sora (NewAPI)", "zh": "Sora (NewAPI)"},
			Description: engine.DisplayName{"en": "OpenAI Sora video generation via NewAPI", "zh": "通过 NewAPI 调用 OpenAI Sora 视频生成"},
			Capability:  "video",
		},
		{
			Name:        "newapi/qwen-max-vl",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Qwen Max VL (NewAPI)", "zh": "通义千问 VL (NewAPI)"},
			Description: engine.DisplayName{"en": "Qwen image generation via NewAPI gateway", "zh": "通过 NewAPI 网关调用通义图片生成"},
			Capability:  "image",
		},
		{
			Name:        "newapi/gemini-2.0-flash",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Gemini 2.0 Flash (NewAPI)", "zh": "Gemini 2.0 Flash (NewAPI)"},
			Description: engine.DisplayName{"en": "Gemini image generation via NewAPI gateway", "zh": "通过 NewAPI 网关调用 Gemini 图片生成"},
			Capability:  "image",
		},
		{
			Name:        "newapi/tts-1",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "TTS-1 (NewAPI)", "zh": "TTS-1 (NewAPI)"},
			Description: engine.DisplayName{"en": "OpenAI TTS via NewAPI gateway", "zh": "通过 NewAPI 网关调用 OpenAI 语音合成"},
			Capability:  "tts",
		},
		{
			Name:        "newapi/tts-1-hd",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "TTS-1 HD (NewAPI)", "zh": "TTS-1 HD (NewAPI)"},
			Description: engine.DisplayName{"en": "OpenAI high-definition TTS via NewAPI", "zh": "通过 NewAPI 调用 OpenAI 高清语音合成"},
			Capability:  "tts",
		},
		{
			Name:        "newapi/whisper-1",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Whisper 1 (NewAPI)", "zh": "Whisper 1 (NewAPI)"},
			Description: engine.DisplayName{"en": "OpenAI Whisper ASR via NewAPI gateway", "zh": "通过 NewAPI 网关调用 OpenAI Whisper 语音识别"},
			Capability:  "asr",
		},
		{
			Name:        "newapi/whisper-large-v3",
			Provider:    "newapi",
			DisplayName: engine.DisplayName{"en": "Whisper Large V3 (NewAPI)", "zh": "Whisper Large V3 (NewAPI)"},
			Description: engine.DisplayName{"en": "Whisper Large V3 ASR via NewAPI gateway", "zh": "通过 NewAPI 网关调用 Whisper Large V3 语音识别"},
			Capability:  "asr",
		},
	}
}
