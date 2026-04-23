package hedra

import "github.com/godeps/aigo/engine"

const modelCharacterV1 = "hedra-character-v1"

// ModelInfos returns i18n metadata for all Hedra models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        modelCharacterV1,
			Provider:    "hedra",
			DisplayName: engine.DisplayName{"en": "Hedra Character V1", "zh": "Hedra 角色 V1"},
			Description: engine.DisplayName{"en": "Character-driven video generation", "zh": "角色驱动视频生成"},
			Intro:       engine.DisplayName{"en": "Hedra Character V1 generates expressive talking-head videos by combining a portrait image with audio input, producing lip-synced character animations for digital avatars, education, and entertainment.", "zh": "Hedra 角色 V1 通过结合人像图片和音频输入生成富有表情的说话头像视频，为数字虚拟形象、教育和娱乐制作口型同步的角色动画。"},
			DocURL:      "https://docs.hedra.com/",
			Capability:  "video",
		},
	}
}
