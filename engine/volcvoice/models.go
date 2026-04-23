package volcvoice

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Volcengine Speech models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelTTSMega,
			Provider:    "volcvoice",
			DisplayName: engine.DisplayName{"en": "Volcano Mega TTS", "zh": "火山大模型语音合成"},
			Description: engine.DisplayName{"en": "Large model TTS with natural voice", "zh": "大模型自然语音合成"},
			Intro:       engine.DisplayName{"en": "Volcano Mega TTS is Volcengine's flagship large-model TTS delivering ultra-natural prosody and emotional richness with broad speaker diversity, ideal for premium content and entertainment applications.", "zh": "火山大模型语音合成是火山引擎旗舰大模型语音合成，提供超自然韵律和丰富情感，说话人多样性广泛，适合高端内容和娱乐应用。"},
			DocURL:      "https://www.volcengine.com/docs/6561/overview",
			Capability:  "tts",
		},
		{
			Name:        ModelTTSIcl,
			Provider:    "volcvoice",
			DisplayName: engine.DisplayName{"en": "Volcano ICL TTS", "zh": "火山上下文学习语音合成"},
			Description: engine.DisplayName{"en": "In-context learning TTS with voice cloning", "zh": "上下文学习语音克隆合成"},
			Intro:       engine.DisplayName{"en": "Volcano ICL TTS uses in-context learning to clone voices from short audio samples, enabling personalized speech synthesis that closely matches a target speaker's timbre and style.", "zh": "火山上下文学习语音合成通过上下文学习从短音频样本克隆声音，实现紧密匹配目标说话人音色和风格的个性化语音合成。"},
			DocURL:      "https://www.volcengine.com/docs/6561/overview",
			Capability:  "tts",
		},
		{
			Name:        ModelTTSDefault,
			Provider:    "volcvoice",
			DisplayName: engine.DisplayName{"en": "Volcano TTS", "zh": "火山语音合成"},
			Description: engine.DisplayName{"en": "Standard TTS synthesis", "zh": "标准语音合成"},
			Intro:       engine.DisplayName{"en": "Volcano TTS provides reliable standard speech synthesis with a wide selection of pre-built voices and language support, suitable for general-purpose voiceover and interactive application needs.", "zh": "火山语音合成提供可靠的标准语音合成，具有丰富的预置声音选择和语言支持，适用于通用配音和互动应用需求。"},
			DocURL:      "https://www.volcengine.com/docs/6561/overview",
			Capability:  "tts",
		},
		{
			Name:        ModelASR,
			Provider:    "volcvoice",
			DisplayName: engine.DisplayName{"en": "Volcano ASR", "zh": "火山语音识别"},
			Description: engine.DisplayName{"en": "Speech recognition", "zh": "语音识别"},
			Intro:       engine.DisplayName{"en": "Volcano ASR delivers accurate real-time speech recognition with support for Chinese, English, and mixed-language input, optimized for voice assistants and transcription applications.", "zh": "火山语音识别提供准确的实时语音识别，支持中文、英文和混合语言输入，专为语音助手和转写应用优化。"},
			DocURL:      "https://www.volcengine.com/docs/6561/overview",
			Capability:  "asr",
		},
		{
			Name:        ModelASRLarge,
			Provider:    "volcvoice",
			DisplayName: engine.DisplayName{"en": "Volcano ASR Pro", "zh": "火山语音识别 Pro"},
			Description: engine.DisplayName{"en": "Professional speech recognition", "zh": "专业语音识别"},
			Intro:       engine.DisplayName{"en": "Volcano ASR Pro is the enhanced large-model speech recognition engine with superior accuracy in noisy environments, domain adaptation, and speaker diarization for professional transcription scenarios.", "zh": "火山语音识别 Pro 是增强型大模型语音识别引擎，在嘈杂环境中具备更高准确率、领域适应和说话人分离能力，适用于专业转写场景。"},
			DocURL:      "https://www.volcengine.com/docs/6561/overview",
			Capability:  "asr",
		},
	}
}
