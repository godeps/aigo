package fal

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Fal.ai models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelFluxDev,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "FLUX Dev", "zh": "FLUX 开发版"},
			Description: engine.DisplayName{"en": "Development model for image generation", "zh": "图片生成开发版模型"},
			Intro:       engine.DisplayName{"en": "FLUX Dev via Fal.ai is an open-weights development model from Black Forest Labs offering high image quality and prompt fidelity with flexible non-commercial usage rights.", "zh": "通过 Fal.ai 访问的 FLUX Dev 是 Black Forest Labs 的开放权重开发模型，提供高图像质量和提示词忠实度，支持灵活的非商业使用。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "image",
		},
		{
			Name:        ModelFluxSchnell,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "FLUX Schnell", "zh": "FLUX 快速版"},
			Description: engine.DisplayName{"en": "Fast image generation", "zh": "快速图片生成"},
			Intro:       engine.DisplayName{"en": "FLUX Schnell via Fal.ai is the fastest FLUX variant optimized for speed with Apache 2.0 licensing, generating quality images in 1–4 steps for rapid prototyping.", "zh": "通过 Fal.ai 访问的 FLUX Schnell 是最快的 FLUX 变体，采用 Apache 2.0 协议，1–4 步即可生成优质图像，适合快速原型设计。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "image",
		},
		{
			Name:        ModelFluxPro,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "FLUX Pro", "zh": "FLUX 专业版"},
			Description: engine.DisplayName{"en": "Professional image generation", "zh": "专业级图片生成"},
			Intro:       engine.DisplayName{"en": "FLUX Pro via Fal.ai delivers professional-grade image generation with superior detail, color accuracy, and text rendering for commercial design and marketing applications.", "zh": "通过 Fal.ai 访问的 FLUX Pro 提供专业级图像生成，具备卓越细节、色彩准确度和文字渲染，适用于商业设计和营销。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "image",
		},
		{
			Name:        ModelSDXL,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "Fast SDXL", "zh": "快速 SDXL"},
			Description: engine.DisplayName{"en": "Fast Stable Diffusion XL generation", "zh": "快速 SDXL 图片生成"},
			Intro:       engine.DisplayName{"en": "Fast SDXL via Fal.ai accelerates Stable Diffusion XL with optimized inference, producing high-resolution 1024px images in seconds for creative and commercial workflows.", "zh": "通过 Fal.ai 加速的快速 SDXL，优化推理后数秒生成 1024px 高分辨率图像，适合创意和商业工作流。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "image",
		},
		{
			Name:        ModelKling,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "Kling Video (Fal)", "zh": "可灵视频 (Fal)"},
			Description: engine.DisplayName{"en": "Kling video generation via Fal", "zh": "通过 Fal 调用可灵视频生成"},
			Intro:       engine.DisplayName{"en": "Kling video generation routed through Fal.ai's infrastructure, providing scalable access to Kuaishou's high-fidelity text-to-video and image-to-video capabilities.", "zh": "通过 Fal.ai 基础设施路由的可灵视频生成，提供对快手高保真文生视频和图生视频能力的可扩展访问。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "video",
		},
		{
			Name:        ModelMinimax,
			Provider:    "fal",
			DisplayName: engine.DisplayName{"en": "Minimax Video (Fal)", "zh": "Minimax 视频 (Fal)"},
			Description: engine.DisplayName{"en": "Minimax video generation via Fal", "zh": "通过 Fal 调用 Minimax 视频生成"},
			Intro:       engine.DisplayName{"en": "MiniMax video generation accessed through Fal.ai, enabling dynamic text-to-video synthesis with consistent motion quality and cinematic scene generation.", "zh": "通过 Fal.ai 访问的 MiniMax 视频生成，实现具有一致动作质量和影视场景生成的动态文生视频合成。"},
			DocURL:      "https://fal.ai/models",
			Capability:  "video",
		},
	}
}
