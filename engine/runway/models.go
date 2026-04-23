package runway

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Runway models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelGen4Turbo,
			Provider:    "runway",
			DisplayName: engine.DisplayName{"en": "Gen-4 Turbo", "zh": "Gen-4 极速版"},
			Description: engine.DisplayName{"en": "Latest fast video generation", "zh": "最新快速视频生成"},
			Intro:       engine.DisplayName{"en": "Runway Gen-4 Turbo is the latest and fastest video generation model with cinematic motion quality, consistent character rendering, and advanced scene understanding for professional video production.", "zh": "Runway Gen-4 极速版是最新最快的视频生成模型，具备影视级动作质量、一致的角色渲染和先进场景理解，适用于专业视频制作。"},
			DocURL:      "https://docs.dev.runwayml.com/",
			Capability:  "video",
		},
		{
			Name:        ModelGen3ATurbo,
			Provider:    "runway",
			DisplayName: engine.DisplayName{"en": "Gen-3 Alpha Turbo", "zh": "Gen-3 Alpha 极速版"},
			Description: engine.DisplayName{"en": "Fast video generation", "zh": "快速视频生成"},
			Intro:       engine.DisplayName{"en": "Runway Gen-3 Alpha Turbo offers accelerated video generation with strong temporal consistency and expressive motion, suited for rapid creative iteration and social content production.", "zh": "Runway Gen-3 Alpha 极速版提供加速视频生成，具备强时序一致性和富有表现力的动作，适合快速创意迭代和社交内容制作。"},
			DocURL:      "https://docs.dev.runwayml.com/",
			Capability:  "video",
		},
	}
}
