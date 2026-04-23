package openrouter

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all OpenRouter models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelGPT5Image,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "GPT-5 Image", "zh": "GPT-5 图片"},
			Description: engine.DisplayName{"en": "GPT-5 image generation via OpenRouter", "zh": "通过 OpenRouter 调用 GPT-5 图片生成"},
			Intro:       engine.DisplayName{"en": "GPT-5 image generation routed through OpenRouter, providing unified API access to OpenAI's most advanced multimodal image synthesis with precise instruction following and photorealistic output.", "zh": "通过 OpenRouter 路由的 GPT-5 图像生成，提供对 OpenAI 最先进多模态图像合成的统一 API 访问，具备精准指令遵循和写实输出。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "image",
		},
		{
			Name:        ModelGPT5ImageMini,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "GPT-5 Image Mini", "zh": "GPT-5 图片 Mini"},
			Description: engine.DisplayName{"en": "Compact GPT-5 image generation", "zh": "轻量 GPT-5 图片生成"},
			Intro:       engine.DisplayName{"en": "GPT-5 Image Mini via OpenRouter provides cost-efficient access to GPT-5's image capabilities with reduced latency, suited for high-volume image generation and rapid design iteration.", "zh": "通过 OpenRouter 访问的 GPT-5 Image Mini 以较低延迟提供具有成本效益的 GPT-5 图像能力，适合高批量图像生成和快速设计迭代。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "image",
		},
		{
			Name:        ModelGeminiFlashImage,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "Gemini Flash Image", "zh": "Gemini Flash 图片"},
			Description: engine.DisplayName{"en": "Gemini Flash image generation via OpenRouter", "zh": "通过 OpenRouter 调用 Gemini Flash 图片生成"},
			Intro:       engine.DisplayName{"en": "Gemini Flash image generation via OpenRouter enables fast, low-latency image synthesis leveraging Google's Gemini multimodal capabilities through a unified gateway.", "zh": "通过 OpenRouter 的 Gemini Flash 图像生成，借助统一网关实现快速低延迟图像合成，利用 Google Gemini 多模态能力。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "image",
		},
		{
			Name:        ModelGemini3ProImage,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "Gemini 3 Pro Image", "zh": "Gemini 3 Pro 图片"},
			Description: engine.DisplayName{"en": "Gemini 3 Pro image generation via OpenRouter", "zh": "通过 OpenRouter 调用 Gemini 3 Pro 图片生成"},
			Intro:       engine.DisplayName{"en": "Gemini 3 Pro image generation via OpenRouter delivers premium image quality from Google's Pro-tier model through a single API endpoint, ideal for professional creative applications.", "zh": "通过 OpenRouter 的 Gemini 3 Pro 图像生成，通过单一 API 端点提供 Google Pro 级模型的高端图像质量，适合专业创意应用。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "image",
		},
		{
			Name:        ModelGPTAudio,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "GPT Audio", "zh": "GPT 音频"},
			Description: engine.DisplayName{"en": "GPT audio generation via OpenRouter", "zh": "通过 OpenRouter 调用 GPT 音频生成"},
			Intro:       engine.DisplayName{"en": "GPT Audio via OpenRouter provides natural-sounding text-to-speech using OpenAI's TTS models through a unified API, supporting multiple voices for voiceovers, assistants, and narration.", "zh": "通过 OpenRouter 的 GPT 音频，利用统一 API 使用 OpenAI TTS 模型提供自然语音合成，支持多种声音用于配音、助手和旁白。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "tts",
		},
		{
			Name:        ModelGPTAudioMini,
			Provider:    "openrouter",
			DisplayName: engine.DisplayName{"en": "GPT Audio Mini", "zh": "GPT 音频 Mini"},
			Description: engine.DisplayName{"en": "Compact GPT audio generation", "zh": "轻量 GPT 音频生成"},
			Intro:       engine.DisplayName{"en": "GPT Audio Mini via OpenRouter offers cost-efficient TTS with reduced latency, suitable for real-time voice applications and high-volume speech synthesis workloads.", "zh": "通过 OpenRouter 的 GPT 音频 Mini 提供具有成本效益的低延迟语音合成，适合实时语音应用和高批量语音合成工作负载。"},
			DocURL:      "https://openrouter.ai/docs/requests",
			Capability:  "tts",
		},
	}
}
