package elevenlabs

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all ElevenLabs models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelMultilingualV2,
			Provider:    "elevenlabs",
			DisplayName: engine.DisplayName{"en": "Multilingual V2", "zh": "多语种 V2"},
			Description: engine.DisplayName{"en": "Highest quality multilingual TTS", "zh": "最高质量多语种语音合成"},
			Intro:       engine.DisplayName{"en": "ElevenLabs' highest-fidelity multilingual TTS model supporting 29 languages with nuanced emotional expression, ideal for audiobooks, dubbing, and premium voice content.", "zh": "ElevenLabs 最高保真度多语种语音合成模型，支持 29 种语言，具备细腻情感表达，适合有声书、配音和高品质语音内容。"},
			DocURL:      "https://elevenlabs.io/docs/api-reference/text-to-speech",
			Capability:  "tts",
		},
		{
			Name:        ModelTurboV25,
			Provider:    "elevenlabs",
			DisplayName: engine.DisplayName{"en": "Turbo V2.5", "zh": "极速 V2.5"},
			Description: engine.DisplayName{"en": "Low-latency TTS optimized for speed", "zh": "低延迟快速语音合成"},
			Intro:       engine.DisplayName{"en": "Turbo V2.5 balances speed and quality with sub-300ms latency, making it the go-to model for real-time conversational agents and live voice applications.", "zh": "Turbo V2.5 以低于 300ms 的延迟平衡速度与质量，是实时对话智能体和直播语音应用的首选模型。"},
			DocURL:      "https://elevenlabs.io/docs/api-reference/text-to-speech",
			Capability:  "tts",
		},
		{
			Name:        ModelFlashV25,
			Provider:    "elevenlabs",
			DisplayName: engine.DisplayName{"en": "Flash V2.5", "zh": "闪电 V2.5"},
			Description: engine.DisplayName{"en": "Ultra-fast TTS with minimal latency", "zh": "超快速语音合成"},
			Intro:       engine.DisplayName{"en": "Flash V2.5 is ElevenLabs' fastest model with ultra-minimal latency under 75ms, designed for latency-critical pipelines such as voice gaming and live interaction.", "zh": "Flash V2.5 是 ElevenLabs 最快的模型，延迟低于 75ms，专为语音游戏和实时互动等对延迟极敏感的场景设计。"},
			DocURL:      "https://elevenlabs.io/docs/api-reference/text-to-speech",
			Capability:  "tts",
		},
		{
			Name:        ModelMultilingualSTS,
			Provider:    "elevenlabs",
			DisplayName: engine.DisplayName{"en": "Multilingual STS V2", "zh": "多语种语音转换 V2"},
			Description: engine.DisplayName{"en": "Speech-to-speech voice conversion", "zh": "语音到语音转换"},
			Intro:       engine.DisplayName{"en": "Multilingual speech-to-speech voice conversion model that transfers vocal style and identity across languages while preserving the original speaker's emotion and delivery.", "zh": "多语种语音转换模型，在跨语言转换声音风格和音色时保留原始说话人的情感和表达方式。"},
			DocURL:      "https://elevenlabs.io/docs/api-reference/text-to-speech",
			Capability:  "tts",
		},
	}
}
