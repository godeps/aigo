package openai

import "github.com/godeps/aigo/engine"

const (
	modelDallE3    = "dall-e-3"
	modelDallE2    = "dall-e-2"
	modelGPTImage2 = "gpt-image-2"
)

// ModelInfos returns i18n metadata for all OpenAI image models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        modelGPTImage2,
			Provider:    "openai",
			DisplayName: engine.DisplayName{"en": "GPT Image 2", "zh": "GPT Image 2"},
			Description: engine.DisplayName{"en": "Next-generation natively multimodal image model", "zh": "新一代原生多模态图片生成模型"},
			Intro:       engine.DisplayName{"en": "GPT Image 2 is OpenAI's natively multimodal image model, delivering strong prompt adherence, refined typography, and photorealistic detail. Returns base64-encoded images and supports transparent backgrounds, multiple output formats, and quality levels (low/medium/high/auto).", "zh": "GPT Image 2 是 OpenAI 的原生多模态图片模型，具备出色的提示词遵循度、文字排版细节与照片级真实感。返回 base64 图像，支持透明背景、多种输出格式以及 low/medium/high/auto 多档质量。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
		{
			Name:        modelDallE3,
			Provider:    "openai",
			DisplayName: engine.DisplayName{"en": "DALL-E 3", "zh": "DALL-E 3"},
			Description: engine.DisplayName{"en": "Advanced image generation", "zh": "高级图片生成"},
			Intro:       engine.DisplayName{"en": "DALL-E 3 delivers significantly improved prompt adherence over its predecessor with native integration of ChatGPT for automatic prompt rewriting, excelling at detailed scenes, typography, and artistic styles.", "zh": "DALL-E 3 相比前代大幅提升提示词遵循度，原生集成 ChatGPT 自动改写提示词，擅长细节场景、文字排版和艺术风格。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
		{
			Name:        modelDallE2,
			Provider:    "openai",
			DisplayName: engine.DisplayName{"en": "DALL-E 2", "zh": "DALL-E 2"},
			Description: engine.DisplayName{"en": "Image generation", "zh": "图片生成"},
			Intro:       engine.DisplayName{"en": "DALL-E 2 supports text-to-image generation, image editing via inpainting, and image variations, offering a cost-effective option for straightforward creative and editing tasks.", "zh": "DALL-E 2 支持文生图、通过局部重绘进行图片编辑和图像变体生成，为简单创意和编辑任务提供具有成本效益的选择。"},
			DocURL:      "https://platform.openai.com/docs/guides/images",
			Capability:  "image",
		},
	}
}
