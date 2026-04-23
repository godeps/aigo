package suno

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Suno models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelChirpV4,
			Provider:    "suno",
			DisplayName: engine.DisplayName{"en": "Chirp V4", "zh": "Chirp V4"},
			Description: engine.DisplayName{"en": "Latest music generation", "zh": "最新版音乐生成"},
			Intro:       engine.DisplayName{"en": "Suno Chirp V4 is the most advanced music generation model producing full songs with rich instrumentation, dynamic arrangements, and expressive vocals from text descriptions across all genres.", "zh": "Suno Chirp V4 是最先进的音乐生成模型，从文字描述生成具有丰富配器、动态编曲和富有表现力人声的完整歌曲，覆盖所有音乐风格。"},
			DocURL:      "https://docs.suno.com/docs/api",
			Capability:  "music",
		},
		{
			Name:        ModelChirpV35,
			Provider:    "suno",
			DisplayName: engine.DisplayName{"en": "Chirp V3.5", "zh": "Chirp V3.5"},
			Description: engine.DisplayName{"en": "Music generation", "zh": "音乐生成"},
			Intro:       engine.DisplayName{"en": "Suno Chirp V3.5 generates full-length original songs with strong melodic structure and vocal clarity, offering a proven and reliable option for AI music creation projects.", "zh": "Suno Chirp V3.5 生成具有强旋律结构和清晰人声的完整原创歌曲，为 AI 音乐创作项目提供经过验证的可靠选择。"},
			DocURL:      "https://docs.suno.com/docs/api",
			Capability:  "music",
		},
	}
}
