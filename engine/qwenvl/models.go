package qwenvl

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Qwen-VL models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelQwen37Plus,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen 3.7 Plus", "zh": "通义千问 3.7 Plus"},
			Description: engine.DisplayName{"en": "Multimodal understanding with text, image, and video", "zh": "多模态理解，支持文本、图片和视频"},
			Intro:       engine.DisplayName{"en": "Qwen 3.7 Plus is Alibaba's latest multimodal model, a full upgrade from Qwen 3.6 Plus, delivering strong vision-language understanding across text, image, and video inputs with balanced speed and accuracy.", "zh": "通义千问 3.7 Plus 是阿里巴巴最新多模态模型，是通义千问 3.6 Plus 的完整升级版本，在文本、图片和视频输入上提供强大的视觉语言理解，兼顾速度与准确度。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
		{
			Name:        ModelQwen36Plus,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen 3.6 Plus", "zh": "通义千问 3.6 Plus"},
			Description: engine.DisplayName{"en": "Multimodal understanding with text, image, and video", "zh": "多模态理解，支持文本、图片和视频"},
			Intro:       engine.DisplayName{"en": "Qwen 3.6 Plus is Alibaba's multimodal model delivering strong vision-language understanding across text, image, and video inputs with balanced speed and accuracy.", "zh": "通义千问 3.6 Plus 是阿里巴巴多模态模型，在文本、图片和视频输入上提供强大的视觉语言理解，兼顾速度与准确度。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
		{
			Name:        ModelQwen36Flash,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen 3.6 Flash", "zh": "通义千问 3.6 Flash"},
			Description: engine.DisplayName{"en": "Fast multimodal understanding with text, image, and video", "zh": "快速多模态理解，支持文本、图片和视频"},
			Intro:       engine.DisplayName{"en": "Qwen 3.6 Flash is Alibaba's fast multimodal model optimized for speed and cost, delivering efficient vision-language understanding across text, image, and video inputs.", "zh": "通义千问 3.6 Flash 是阿里巴巴针对速度和成本优化的快速多模态模型，在文本、图片和视频输入上提供高效的视觉语言理解。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
		{
			Name:        ModelQwen35OmniPlus,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen 3.5 Omni Plus", "zh": "通义千问 3.5 Omni Plus"},
			Description: engine.DisplayName{"en": "Full multimodal model with text, image, video, and audio understanding", "zh": "全模态大模型，支持文本、图片、视频和音频理解"},
			Intro:       engine.DisplayName{"en": "Qwen 3.5 Omni Plus is Alibaba's omni-modal model capable of understanding text, images, video, and audio inputs, providing comprehensive multimodal reasoning and analysis.", "zh": "通义千问 3.5 Omni Plus 是阿里巴巴全模态大模型，支持文本、图片、视频和音频输入的理解，提供全面的多模态推理与分析能力。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-omni-api-reference",
			Capability:  "video_understanding",
		},
	}
}
