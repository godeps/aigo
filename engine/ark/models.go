package ark

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Ark (Volcengine) models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelSeedream3_0,
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedream 3.0", "zh": "Seedream 3.0"},
			Description: engine.DisplayName{"en": "High-quality image generation", "zh": "高质量图片生成"},
			Intro:       engine.DisplayName{"en": "Seedream 3.0 is ByteDance's flagship image generation model featuring exceptional prompt understanding, photorealistic rendering, and support for complex compositional scenes.", "zh": "Seedream 3.0 是字节跳动旗舰图像生成模型，具备卓越的提示词理解能力、写实渲染效果和对复杂构图场景的支持。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "image",
		},
		{
			Name:        ModelSeedream2_1,
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedream 2.1", "zh": "Seedream 2.1"},
			Description: engine.DisplayName{"en": "Image generation", "zh": "图片生成"},
			Intro:       engine.DisplayName{"en": "Seedream 2.1 delivers balanced image generation with strong aesthetic quality and broad style versatility, suitable for creative design, illustration, and content production.", "zh": "Seedream 2.1 提供均衡的图像生成能力，具备强大的美学质量和广泛的风格多样性，适用于创意设计、插画和内容制作。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "image",
		},
	}
}
