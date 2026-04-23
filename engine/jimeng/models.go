package jimeng

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Jimeng models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelJimeng21,
			Provider:    "jimeng",
			DisplayName: engine.DisplayName{"en": "Jimeng 2.1", "zh": "即梦 2.1"},
			Description: engine.DisplayName{"en": "Image generation", "zh": "图片生成"},
			Intro:       engine.DisplayName{"en": "Jimeng 2.1 is ByteDance's latest image generation model with enhanced aesthetic quality, improved Chinese cultural style understanding, and precise prompt-to-image alignment.", "zh": "即梦 2.1 是字节跳动最新图像生成模型，具备增强的美学质量、改进的中国文化风格理解和精准的提示词到图像对应能力。"},
			DocURL:      "https://www.volcengine.com/docs/jimeng/open-api-overview",
			Capability:  "image",
		},
		{
			Name:        ModelJimeng20Pro,
			Provider:    "jimeng",
			DisplayName: engine.DisplayName{"en": "Jimeng 2.0 Pro", "zh": "即梦 2.0 Pro"},
			Description: engine.DisplayName{"en": "Professional image generation", "zh": "专业级图片生成"},
			Intro:       engine.DisplayName{"en": "Jimeng 2.0 Pro delivers professional-grade image generation with superior detail rendering and strong support for artistic styles, architecture visualization, and commercial design.", "zh": "即梦 2.0 Pro 提供专业级图像生成，具备卓越细节渲染，强力支持艺术风格、建筑可视化和商业设计。"},
			DocURL:      "https://www.volcengine.com/docs/jimeng/open-api-overview",
			Capability:  "image",
		},
	}
}
