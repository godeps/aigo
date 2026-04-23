package midjourney

import "github.com/godeps/aigo/engine"

const modelMidjourney = "midjourney"

// ModelInfos returns i18n metadata for all MidJourney models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        modelMidjourney,
			Provider:    "midjourney",
			DisplayName: engine.DisplayName{"en": "Midjourney", "zh": "Midjourney"},
			Description: engine.DisplayName{"en": "AI image generation", "zh": "AI 图片生成"},
			Intro:       engine.DisplayName{"en": "Midjourney is renowned for its distinctive artistic aesthetic and exceptional image quality, excelling at creating evocative illustrations, concept art, and stylized visuals for creative professionals.", "zh": "Midjourney 以其独特的艺术美学和卓越图像质量著称，擅长为创意专业人士创作富有感染力的插画、概念艺术和风格化视觉作品。"},
			DocURL:      "https://docs.goapi.ai/docs/midjourney-api",
			Capability:  "image",
		},
	}
}
