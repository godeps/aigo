package google

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Google Imagen models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelImagen3Generate002,
			Provider:    "google",
			DisplayName: engine.DisplayName{"en": "Imagen 3 (002)", "zh": "Imagen 3 (002)"},
			Description: engine.DisplayName{"en": "Latest Imagen 3 image generation", "zh": "最新 Imagen 3 图片生成"},
			Intro:       engine.DisplayName{"en": "Imagen 3 (002) is Google's most capable image generation model with state-of-the-art photorealism, precise text rendering in images, and fine-grained artistic style control.", "zh": "Imagen 3 (002) 是 Google 最强大的图像生成模型，具备业界领先的写实效果、图像内精准文字渲染和精细艺术风格控制。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs/imagen",
			Capability:  "image",
		},
		{
			Name:        ModelImagen3Generate001,
			Provider:    "google",
			DisplayName: engine.DisplayName{"en": "Imagen 3 (001)", "zh": "Imagen 3 (001)"},
			Description: engine.DisplayName{"en": "Imagen 3 image generation", "zh": "Imagen 3 图片生成"},
			Intro:       engine.DisplayName{"en": "Imagen 3 (001) provides high-quality photorealistic image generation with strong compositional understanding and detailed texture rendering for creative and commercial projects.", "zh": "Imagen 3 (001) 提供高质量写实图像生成，具备强大的构图理解和详细纹理渲染，适用于创意和商业项目。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs/imagen",
			Capability:  "image",
		},
	}
}
