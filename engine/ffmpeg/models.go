package ffmpeg

import "github.com/godeps/aigo/engine"

const (
	ModelFFmpegSFX = "ffmpeg-sfx"
	ModelFFmpegMix = "ffmpeg-audio-mix"
)

// ModelInfos returns i18n metadata for FFmpeg models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelFFmpegSFX,
			Provider:    "ffmpeg",
			DisplayName: engine.DisplayName{"en": "FFmpeg SFX", "zh": "FFmpeg 音效"},
			Description: engine.DisplayName{"en": "Basic sound effects via FFmpeg lavfi", "zh": "基于 FFmpeg lavfi 的基础音效生成"},
			Intro:       engine.DisplayName{"en": "Generate simple sound effects locally using FFmpeg's built-in audio sources (sine waves, noise generators). Best as a fast fallback when AI-generated SFX is not needed.", "zh": "使用 FFmpeg 内置音频源（正弦波、噪声生成器）在本地生成简单音效。适合不需要 AI 生成音效时的快速备选方案。"},
			Capability:  "sfx",
		},
		{
			Name:        ModelFFmpegMix,
			Provider:    "ffmpeg",
			DisplayName: engine.DisplayName{"en": "FFmpeg Audio Mix", "zh": "FFmpeg 音频混合"},
			Description: engine.DisplayName{"en": "Mix multiple audio tracks with FFmpeg", "zh": "使用 FFmpeg 混合多音轨"},
			Intro:       engine.DisplayName{"en": "Combine multiple audio sources (music, voice, sound effects) into a single track with per-track volume and delay controls using FFmpeg's amix filter.", "zh": "使用 FFmpeg amix 滤镜将多个音频源（音乐、语音、音效）混合为单轨，支持逐轨音量和延迟控制。"},
			Capability:  "audio_mix",
		},
	}
}
