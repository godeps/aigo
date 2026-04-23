package luma

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Luma models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelRay2,
			Provider:    "luma",
			DisplayName: engine.DisplayName{"en": "Ray 2", "zh": "Ray 2"},
			Description: engine.DisplayName{"en": "High-quality video generation", "zh": "高质量视频生成"},
			Intro:       engine.DisplayName{"en": "Luma Ray 2 produces cinematic-quality video with fluid motion, accurate physics, and strong creative direction support, ideal for film pre-visualization, advertising, and content production.", "zh": "Luma Ray 2 生成影视级视频，具备流畅动作、精准物理效果和强大的创意方向支持，适合电影预可视化、广告和内容制作。"},
			DocURL:      "https://docs.lumalabs.ai/docs/api",
			Capability:  "video",
		},
		{
			Name:        ModelRayFlash2,
			Provider:    "luma",
			DisplayName: engine.DisplayName{"en": "Ray Flash 2", "zh": "Ray Flash 2"},
			Description: engine.DisplayName{"en": "Fast video generation", "zh": "快速视频生成"},
			Intro:       engine.DisplayName{"en": "Luma Ray Flash 2 is optimized for speed with significantly reduced generation time while retaining strong visual quality, suited for rapid iteration and real-time content workflows.", "zh": "Luma Ray Flash 2 针对速度优化，大幅缩短生成时间同时保持强视觉质量，适合快速迭代和实时内容工作流。"},
			DocURL:      "https://docs.lumalabs.ai/docs/api",
			Capability:  "video",
		},
		{
			Name:        ModelPhoton1,
			Provider:    "luma",
			DisplayName: engine.DisplayName{"en": "Photon 1", "zh": "Photon 1"},
			Description: engine.DisplayName{"en": "High-quality image generation", "zh": "高质量图片生成"},
			Intro:       engine.DisplayName{"en": "Luma Photon 1 is a high-quality image generation model emphasizing vivid color, compositional accuracy, and photorealistic rendering for creative and commercial imagery.", "zh": "Luma Photon 1 是高质量图像生成模型，注重生动色彩、构图准确性和写实渲染，适用于创意和商业图像。"},
			DocURL:      "https://docs.lumalabs.ai/docs/api",
			Capability:  "image",
		},
		{
			Name:        ModelPhotonFlash1,
			Provider:    "luma",
			DisplayName: engine.DisplayName{"en": "Photon Flash 1", "zh": "Photon Flash 1"},
			Description: engine.DisplayName{"en": "Fast image generation", "zh": "快速图片生成"},
			Intro:       engine.DisplayName{"en": "Luma Photon Flash 1 accelerates image generation with minimal quality trade-off, enabling fast visual prototyping and high-throughput batch image workflows.", "zh": "Luma Photon Flash 1 以最小质量损耗加速图像生成，支持快速视觉原型设计和高吞吐量批量图像工作流。"},
			DocURL:      "https://docs.lumalabs.ai/docs/api",
			Capability:  "image",
		},
	}
}
