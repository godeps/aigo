package gpt4o

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all GPT-4o models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelGPT4o,
			Provider:    "gpt4o",
			DisplayName: engine.DisplayName{"en": "GPT-4o", "zh": "GPT-4o"},
			Description: engine.DisplayName{"en": "Multimodal image generation and understanding", "zh": "多模态图片生成与理解"},
			Intro:       engine.DisplayName{"en": "GPT-4o's native image generation capability produces photorealistic images with accurate text rendering and precise instruction following, suitable for UI mockups, illustrations, and visual content creation.", "zh": "GPT-4o 原生图像生成能力可生成写实图像，具备精准文字渲染和精确指令遵循，适用于 UI 原型、插画和视觉内容创作。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
		{
			Name:        ModelGPT4oMini,
			Provider:    "gpt4o",
			DisplayName: engine.DisplayName{"en": "GPT-4o Mini", "zh": "GPT-4o Mini"},
			Description: engine.DisplayName{"en": "Compact multimodal image generation", "zh": "轻量多模态图片生成"},
			Intro:       engine.DisplayName{"en": "GPT-4o Mini offers cost-efficient image generation with solid quality and instruction fidelity, ideal for high-volume applications and rapid visual prototyping at reduced cost.", "zh": "GPT-4o Mini 提供具有成本效益的图像生成，质量和指令忠实度良好，适合高批量应用和以较低成本进行快速视觉原型设计。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
		{
			Name:        ModelGPT4Turbo,
			Provider:    "gpt4o",
			DisplayName: engine.DisplayName{"en": "GPT-4 Turbo", "zh": "GPT-4 Turbo"},
			Description: engine.DisplayName{"en": "Fast multimodal generation", "zh": "快速多模态生成"},
			Intro:       engine.DisplayName{"en": "GPT-4 Turbo's image generation delivers strong visual understanding and creation capabilities with optimized throughput, suitable for applications requiring both vision comprehension and image output.", "zh": "GPT-4 Turbo 的图像生成具备强大的视觉理解和创作能力，优化吞吐量，适合同时需要视觉理解和图像输出的应用。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
	}
}
