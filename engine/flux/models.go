package flux

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all FLUX models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelProUltra,
			Provider:    "flux",
			DisplayName: engine.DisplayName{"en": "FLUX Pro 1.1 Ultra", "zh": "FLUX Pro 1.1 旗舰版"},
			Description: engine.DisplayName{"en": "Highest quality image generation", "zh": "最高质量图片生成"},
			Intro:       engine.DisplayName{"en": "FLUX Pro 1.1 Ultra is Black Forest Labs' highest-tier model delivering photorealistic images up to 4MP resolution with unmatched prompt adherence and fine detail reproduction.", "zh": "FLUX Pro 1.1 Ultra 是 Black Forest Labs 最高端模型，可生成高达 4MP 分辨率的写实图像，具备无与伦比的提示词遵循度和精细细节还原。"},
			DocURL:      "https://docs.bfl.ml/",
			Capability:  "image",
		},
		{
			Name:        ModelPro11,
			Provider:    "flux",
			DisplayName: engine.DisplayName{"en": "FLUX Pro 1.1", "zh": "FLUX Pro 1.1"},
			Description: engine.DisplayName{"en": "Professional image generation", "zh": "专业级图片生成"},
			Intro:       engine.DisplayName{"en": "FLUX Pro 1.1 improves upon the original Pro with 6× faster generation speed and enhanced image quality, making it the preferred choice for high-volume commercial image production.", "zh": "FLUX Pro 1.1 在原版 Pro 基础上提升 6 倍生成速度并改善图像质量，是高批量商业图像制作的首选。"},
			DocURL:      "https://docs.bfl.ml/",
			Capability:  "image",
		},
		{
			Name:        ModelPro,
			Provider:    "flux",
			DisplayName: engine.DisplayName{"en": "FLUX Pro", "zh": "FLUX Pro"},
			Description: engine.DisplayName{"en": "High-quality image generation", "zh": "高质量图片生成"},
			Intro:       engine.DisplayName{"en": "FLUX Pro is the original state-of-the-art model from Black Forest Labs combining a 12B parameter flow transformer with exceptional visual quality for professional creative work.", "zh": "FLUX Pro 是 Black Forest Labs 原版旗舰模型，结合 120 亿参数流式变换器和卓越视觉质量，适用于专业创意工作。"},
			DocURL:      "https://docs.bfl.ml/",
			Capability:  "image",
		},
		{
			Name:        ModelDev,
			Provider:    "flux",
			DisplayName: engine.DisplayName{"en": "FLUX Dev", "zh": "FLUX 开发版"},
			Description: engine.DisplayName{"en": "Development model for image generation", "zh": "图片生成开发版"},
			Intro:       engine.DisplayName{"en": "FLUX Dev is an open-weights guidance-distilled model offering near-Pro quality with non-commercial licensing, widely used for research, experimentation, and fine-tuning pipelines.", "zh": "FLUX Dev 是开放权重的引导蒸馏模型，提供接近 Pro 的质量，采用非商业授权，广泛用于研究、实验和微调流程。"},
			DocURL:      "https://docs.bfl.ml/",
			Capability:  "image",
		},
	}
}
