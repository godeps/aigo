package qwenvl

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Qwen-VL models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelQwen36Plus,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen 3.6 Plus", "zh": "通义千问 3.6 Plus"},
			Description: engine.DisplayName{"en": "Multimodal understanding with text, image, and video", "zh": "多模态理解，支持文本、图片和视频"},
			Intro:       engine.DisplayName{"en": "Qwen 3.6 Plus is Alibaba's latest multimodal model delivering strong vision-language understanding across text, image, and video inputs with balanced speed and accuracy.", "zh": "通义千问 3.6 Plus 是阿里巴巴最新多模态模型，在文本、图片和视频输入上提供强大的视觉语言理解，兼顾速度与准确度。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
		{
			Name:        ModelQwenVLMax,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen-VL Max", "zh": "通义千问 VL Max"},
			Description: engine.DisplayName{"en": "Highest-quality multimodal understanding", "zh": "最高质量多模态理解"},
			Intro:       engine.DisplayName{"en": "Qwen-VL Max is the most capable vision-language model in the Qwen family, excelling at complex video analysis, detailed image description, and nuanced multimodal reasoning tasks.", "zh": "通义千问 VL Max 是 Qwen 系列中能力最强的视觉语言模型，擅长复杂视频分析、精细图片描述和复杂多模态推理任务。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
		{
			Name:        ModelQwenVLPlus,
			Provider:    "qwenvl",
			DisplayName: engine.DisplayName{"en": "Qwen-VL Plus", "zh": "通义千问 VL Plus"},
			Description: engine.DisplayName{"en": "Balanced multimodal understanding", "zh": "均衡多模态理解"},
			Intro:       engine.DisplayName{"en": "Qwen-VL Plus offers a good balance of quality and cost for multimodal understanding tasks including video comprehension, image analysis, and visual question answering.", "zh": "通义千问 VL Plus 在多模态理解任务中提供质量与成本的良好平衡，包括视频理解、图片分析和视觉问答。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/qwen-vl-api-reference",
			Capability:  "video_understanding",
		},
	}
}
