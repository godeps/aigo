package minimax

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all MiniMax models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelMusic26,
			Provider:    "minimax",
			DisplayName: engine.DisplayName{"en": "Music 2.6", "zh": "音乐 2.6"},
			Description: engine.DisplayName{"en": "Music generation", "zh": "音乐生成"},
			Intro:       engine.DisplayName{"en": "MiniMax Music 2.6 generates full-length songs from text prompts with controllable genre, mood, and instrumentation, supporting both lyric input and purely instrumental compositions.", "zh": "MiniMax 音乐 2.6 从文字提示生成完整歌曲，可控制曲风、情绪和乐器编排，支持歌词输入和纯器乐作品。"},
			DocURL:      "https://intl.minimaxi.com/document/music-generation",
			Capability:  "music",
		},
		{
			Name:        ModelMusicCover,
			Provider:    "minimax",
			DisplayName: engine.DisplayName{"en": "Music Cover", "zh": "音乐翻唱"},
			Description: engine.DisplayName{"en": "Music cover generation", "zh": "音乐翻唱生成"},
			Intro:       engine.DisplayName{"en": "MiniMax Music Cover generates AI-powered song covers by applying a new vocal style or singer identity to an existing track, enabling creative reinterpretations and style transfers.", "zh": "MiniMax 音乐翻唱通过将新的演唱风格或歌手音色应用到现有曲目，生成 AI 翻唱版本，支持创意再诠释和风格迁移。"},
			DocURL:      "https://intl.minimaxi.com/document/music-generation",
			Capability:  "music",
		},
		{
			Name:        ModelMusic26Free,
			Provider:    "minimax",
			DisplayName: engine.DisplayName{"en": "Music 2.6 Free", "zh": "音乐 2.6 免费版"},
			Description: engine.DisplayName{"en": "Free music generation", "zh": "免费音乐生成"},
			Intro:       engine.DisplayName{"en": "MiniMax Music 2.6 Free provides accessible music generation with the same core capabilities as Music 2.6 at no cost, ideal for personal projects and exploring AI music creation.", "zh": "MiniMax 音乐 2.6 免费版以零成本提供与音乐 2.6 相同的核心能力，适合个人项目和探索 AI 音乐创作。"},
			DocURL:      "https://intl.minimaxi.com/document/music-generation",
			Capability:  "music",
		},
		{
			Name:        ModelMusicCoverFree,
			Provider:    "minimax",
			DisplayName: engine.DisplayName{"en": "Music Cover Free", "zh": "音乐翻唱免费版"},
			Description: engine.DisplayName{"en": "Free music cover generation", "zh": "免费音乐翻唱生成"},
			Intro:       engine.DisplayName{"en": "MiniMax Music Cover Free offers AI song cover generation at no cost, allowing users to experiment with vocal style transfers and creative reinterpretations without commercial licensing.", "zh": "MiniMax 音乐翻唱免费版提供免费 AI 歌曲翻唱生成，允许用户在无商业授权的情况下体验演唱风格迁移和创意再诠释。"},
			DocURL:      "https://intl.minimaxi.com/document/music-generation",
			Capability:  "music",
		},
	}
}
