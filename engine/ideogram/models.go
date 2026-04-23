package ideogram

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Ideogram models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelV2A,
			Provider:    "ideogram",
			DisplayName: engine.DisplayName{"en": "Ideogram V2A", "zh": "Ideogram V2A"},
			Description: engine.DisplayName{"en": "Advanced image generation with text rendering", "zh": "增强版图片生成（含文字渲染）"},
			Intro:       engine.DisplayName{"en": "Ideogram V2A is the most capable Ideogram model with industry-leading in-image text rendering accuracy, strong prompt adherence, and diverse aesthetic styles for design and branding.", "zh": "Ideogram V2A 是 Ideogram 最强模型，具备业界领先的图像内文字渲染精度、强提示词遵循度和多样美学风格，适用于设计和品牌宣传。"},
			DocURL:      "https://developer.ideogram.ai/api-reference/generate-image",
			Capability:  "image",
		},
		{
			Name:        ModelV2ATurbo,
			Provider:    "ideogram",
			DisplayName: engine.DisplayName{"en": "Ideogram V2A Turbo", "zh": "Ideogram V2A 极速版"},
			Description: engine.DisplayName{"en": "Fast image generation with text rendering", "zh": "快速图片生成（含文字渲染）"},
			Intro:       engine.DisplayName{"en": "Ideogram V2A Turbo delivers the same text-rendering strengths of V2A at 2× the speed, optimized for real-time design workflows and applications requiring fast turnaround.", "zh": "Ideogram V2A 极速版以两倍速度提供与 V2A 相同的文字渲染优势，专为需要快速周转的实时设计工作流和应用优化。"},
			DocURL:      "https://developer.ideogram.ai/api-reference/generate-image",
			Capability:  "image",
		},
		{
			Name:        ModelV2,
			Provider:    "ideogram",
			DisplayName: engine.DisplayName{"en": "Ideogram V2", "zh": "Ideogram V2"},
			Description: engine.DisplayName{"en": "Image generation", "zh": "图片生成"},
			Intro:       engine.DisplayName{"en": "Ideogram V2 excels at generating images with accurate embedded text, making it uniquely suited for creating logos, posters, typographic art, and marketing creatives.", "zh": "Ideogram V2 擅长生成含精准内嵌文字的图像，特别适合创作 Logo、海报、文字艺术和营销素材。"},
			DocURL:      "https://developer.ideogram.ai/api-reference/generate-image",
			Capability:  "image",
		},
		{
			Name:        ModelV2Turbo,
			Provider:    "ideogram",
			DisplayName: engine.DisplayName{"en": "Ideogram V2 Turbo", "zh": "Ideogram V2 极速版"},
			Description: engine.DisplayName{"en": "Fast image generation", "zh": "快速图片生成"},
			Intro:       engine.DisplayName{"en": "Ideogram V2 Turbo offers accelerated generation with retained text rendering quality, enabling rapid iteration on design concepts and high-throughput creative pipelines.", "zh": "Ideogram V2 极速版在保持文字渲染质量的同时加速生成，支持设计概念的快速迭代和高吞吐量创意流程。"},
			DocURL:      "https://developer.ideogram.ai/api-reference/generate-image",
			Capability:  "image",
		},
	}
}
