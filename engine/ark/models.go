package ark

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Ark (Volcengine) models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelSeedream3_0,
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedream 3.0", "zh": "Seedream 3.0"},
			Description: engine.DisplayName{"en": "High-quality image generation", "zh": "高质量图片生成"},
			Intro:       engine.DisplayName{"en": "Seedream 3.0 is ByteDance's flagship image generation model featuring exceptional prompt understanding, photorealistic rendering, and support for complex compositional scenes.", "zh": "Seedream 3.0 是字节跳动旗舰图像生成模型，具备卓越的提示词理解能力、写实渲染效果和对复杂构图场景的支持。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "image",
		},
		{
			Name:        ModelSeedream2_1,
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedream 2.1", "zh": "Seedream 2.1"},
			Description: engine.DisplayName{"en": "Image generation", "zh": "图片生成"},
			Intro:       engine.DisplayName{"en": "Seedream 2.1 delivers balanced image generation with strong aesthetic quality and broad style versatility, suitable for creative design, illustration, and content production.", "zh": "Seedream 2.1 提供均衡的图像生成能力，具备强大的美学质量和广泛的风格多样性，适用于创意设计、插画和内容制作。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "image",
		},
		{
			Name:        "doubao-seedance-2-0-260128",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 2.0", "zh": "Seedance 2.0"},
			Description: engine.DisplayName{"en": "High-quality video generation", "zh": "高质量视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 2.0 is ByteDance's latest video generation model with superior motion quality, multi-subject consistency, and support for text-to-video and image-to-video.", "zh": "Seedance 2.0 是字节跳动最新视频生成模型，具备卓越的运动质量、多主体一致性，支持文生视频和图生视频。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
		{
			Name:        "doubao-seedance-2-0-fast-260128",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 2.0 Fast", "zh": "Seedance 2.0 快速版"},
			Description: engine.DisplayName{"en": "Fast video generation", "zh": "快速视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 2.0 Fast delivers accelerated video generation with reduced latency while maintaining strong visual quality, ideal for real-time and batch workflows.", "zh": "Seedance 2.0 快速版提供加速视频生成，延迟更低同时保持优秀视觉质量，适合实时和批量工作流。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
		{
			Name:        "doubao-seedance-1-0-lite-250428",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 1.0 Lite", "zh": "Seedance 1.0 轻量版"},
			Description: engine.DisplayName{"en": "Lightweight video generation", "zh": "轻量视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 1.0 Lite is a cost-effective video generation model suitable for rapid prototyping and preview scenarios.", "zh": "Seedance 1.0 轻量版是一款高性价比的视频生成模型，适用于快速原型和预览场景。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
		{
			Name:        "doubao-seedance-1-5-pro-251215",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 1.5 Pro", "zh": "Seedance 1.5 Pro"},
			Description: engine.DisplayName{"en": "Professional video generation", "zh": "专业视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 1.5 Pro delivers professional-grade video generation with draft preview mode, offline inference, and audio generation support.", "zh": "Seedance 1.5 Pro 提供专业级视频生成，支持样片预览模式、离线推理和有声视频生成。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
		{
			Name:        "doubao-seedance-1-0-pro-250528",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 1.0 Pro", "zh": "Seedance 1.0 Pro"},
			Description: engine.DisplayName{"en": "Professional video generation", "zh": "专业视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 1.0 Pro offers high-quality video generation with rich motion control and flexible duration settings.", "zh": "Seedance 1.0 Pro 提供高质量视频生成，支持丰富的运动控制和灵活的时长设置。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
		{
			Name:        "doubao-seedance-1-0-pro-fast-251015",
			Provider:    "ark",
			DisplayName: engine.DisplayName{"en": "Seedance 1.0 Pro Fast", "zh": "Seedance 1.0 Pro 快速版"},
			Description: engine.DisplayName{"en": "Fast professional video generation", "zh": "快速专业视频生成"},
			Intro:       engine.DisplayName{"en": "Seedance 1.0 Pro Fast provides accelerated video generation with reduced latency for time-sensitive workflows.", "zh": "Seedance 1.0 Pro 快速版提供加速视频生成，降低延迟，适合时间敏感的工作流。"},
			DocURL:      "https://www.volcengine.com/docs/6791/overview",
			Capability:  "video",
		},
	}
}
