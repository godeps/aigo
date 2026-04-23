package pika

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Pika models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelPika22,
			Provider:    "pika",
			DisplayName: engine.DisplayName{"en": "Pika 2.2", "zh": "Pika 2.2"},
			Description: engine.DisplayName{"en": "Latest video generation", "zh": "最新版视频生成"},
			Intro:       engine.DisplayName{"en": "Pika 2.2 is the latest generation model with enhanced motion realism, precise object control, and improved scene coherence for cinematic short-form video creation.", "zh": "Pika 2.2 是最新一代模型，增强动作真实感、精确对象控制和改进场景一致性，适用于影视级短视频创作。"},
			DocURL:      "https://docs.pika.art/",
			Capability:  "video",
		},
		{
			Name:        ModelPika21,
			Provider:    "pika",
			DisplayName: engine.DisplayName{"en": "Pika 2.1", "zh": "Pika 2.1"},
			Description: engine.DisplayName{"en": "Video generation", "zh": "视频生成"},
			Intro:       engine.DisplayName{"en": "Pika 2.1 delivers expressive video generation with strong character animation and visual effects capabilities, well-suited for social media content and creative short videos.", "zh": "Pika 2.1 提供富有表现力的视频生成，具备强大的角色动画和视觉效果能力，非常适合社交媒体内容和创意短视频。"},
			DocURL:      "https://docs.pika.art/",
			Capability:  "video",
		},
	}
}
