package recraft

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Recraft models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelRecraftV3,
			Provider:    "recraft",
			DisplayName: engine.DisplayName{"en": "Recraft V3", "zh": "Recraft V3"},
			Description: engine.DisplayName{"en": "Professional vector and raster image generation", "zh": "专业矢量和光栅图片生成"},
			Intro:       engine.DisplayName{"en": "Recraft V3 uniquely supports both vector SVG and raster image generation with exceptional brand consistency, style locking, and precise text rendering — purpose-built for designers and brand teams.", "zh": "Recraft V3 独特支持矢量 SVG 和光栅图像生成，具备卓越的品牌一致性、风格锁定和精准文字渲染，专为设计师和品牌团队打造。"},
			DocURL:      "https://www.recraft.ai/docs",
			Capability:  "image",
		},
		{
			Name:        ModelRecraft20B,
			Provider:    "recraft",
			DisplayName: engine.DisplayName{"en": "Recraft 20B", "zh": "Recraft 20B"},
			Description: engine.DisplayName{"en": "Large model image generation", "zh": "大模型图片生成"},
			Intro:       engine.DisplayName{"en": "Recraft 20B is a 20-billion parameter image generation model delivering state-of-the-art quality with deep aesthetic understanding, fine detail, and consistent style for professional creative work.", "zh": "Recraft 20B 是 200 亿参数图像生成模型，具备深度审美理解、精细细节和一致风格，为专业创意工作提供最先进质量。"},
			DocURL:      "https://www.recraft.ai/docs",
			Capability:  "image",
		},
	}
}
