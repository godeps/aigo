package stability

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Stability AI models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelSD35Large,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "SD 3.5 Large", "zh": "SD 3.5 大模型"},
			Description: engine.DisplayName{"en": "Large Stable Diffusion 3.5 model", "zh": "Stable Diffusion 3.5 大模型"},
			Intro:       engine.DisplayName{"en": "Stable Diffusion 3.5 Large is Stability AI's most capable open model with 8B parameters, delivering top-tier prompt adherence, fine detail, and diverse artistic styles for professional creative work.", "zh": "Stable Diffusion 3.5 Large 是 Stability AI 最强大的开放模型，80 亿参数，提供顶级提示词遵循度、精细细节和多样艺术风格，适用于专业创意工作。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
		{
			Name:        ModelSD35LargeTurbo,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "SD 3.5 Large Turbo", "zh": "SD 3.5 大模型极速版"},
			Description: engine.DisplayName{"en": "Fast large SD 3.5 generation", "zh": "快速大模型图片生成"},
			Intro:       engine.DisplayName{"en": "SD 3.5 Large Turbo uses distillation to generate high-quality images in just 4 steps at near-Large quality, optimized for speed-sensitive workflows without sacrificing visual fidelity.", "zh": "SD 3.5 Large Turbo 通过蒸馏技术仅需 4 步即可生成接近 Large 质量的高品质图像，专为对速度敏感的工作流优化而不牺牲视觉保真度。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
		{
			Name:        ModelSD35Medium,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "SD 3.5 Medium", "zh": "SD 3.5 中型"},
			Description: engine.DisplayName{"en": "Balanced SD 3.5 model", "zh": "平衡型 SD 3.5 模型"},
			Intro:       engine.DisplayName{"en": "SD 3.5 Medium balances quality and resource efficiency with a 2.5B parameter architecture, making it well-suited for local deployment and consumer hardware while retaining strong image generation capabilities.", "zh": "SD 3.5 Medium 以 25 亿参数架构平衡质量与资源效率，适合本地部署和消费级硬件，同时保留强大的图像生成能力。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
		{
			Name:        ModelSD3Turbo,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "SD 3 Turbo", "zh": "SD 3 极速版"},
			Description: engine.DisplayName{"en": "Fast SD 3 generation", "zh": "快速 SD 3 图片生成"},
			Intro:       engine.DisplayName{"en": "SD 3 Turbo is a fast distilled variant of Stable Diffusion 3 producing quality images in minimal steps, offering a cost-effective solution for high-throughput creative and commercial pipelines.", "zh": "SD 3 极速版是 Stable Diffusion 3 的快速蒸馏变体，以最少步骤生成优质图像，为高吞吐量创意和商业流程提供具有成本效益的解决方案。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
		{
			Name:        ModelImageCore,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "Stable Image Core", "zh": "Stable Image 核心版"},
			Description: engine.DisplayName{"en": "Core image generation model", "zh": "核心图片生成模型"},
			Intro:       engine.DisplayName{"en": "Stable Image Core provides reliable, consistent image generation optimized for speed and broad style coverage, suitable as a dependable foundation for production image generation workflows.", "zh": "Stable Image Core 提供可靠一致的图像生成，针对速度和广泛风格覆盖优化，适合作为生产图像生成工作流的可靠基础。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
		{
			Name:        ModelImageUltra,
			Provider:    "stability",
			DisplayName: engine.DisplayName{"en": "Stable Image Ultra", "zh": "Stable Image 旗舰版"},
			Description: engine.DisplayName{"en": "Ultra quality image generation", "zh": "旗舰级图片生成"},
			Intro:       engine.DisplayName{"en": "Stable Image Ultra delivers Stability AI's highest image quality with exceptional photorealism, rich color depth, and intricate detail for premium commercial photography and advertising.", "zh": "Stable Image Ultra 提供 Stability AI 最高图像质量，具备卓越写实效果、丰富色彩深度和精致细节，适用于高端商业摄影和广告。"},
			DocURL:      "https://platform.stability.ai/docs/api-reference",
			Capability:  "image",
		},
	}
}
