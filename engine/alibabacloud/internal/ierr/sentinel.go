package ierr

import "errors"

// 根包 aliyun 会重新导出这些变量，对外 API 不变。
var (
	ErrMissingPrompt      = errors.New("aliyun: prompt not found in workflow graph")
	ErrMissingReference   = errors.New("aliyun: reference media not found in workflow graph")
	ErrMissingVoice       = errors.New("aliyun: TTS voice not found in workflow graph")
	ErrMissingVoiceDesign = errors.New("aliyun: voice design fields missing (voice_prompt, preview_text, target_model)")
	ErrMissingAudioURL    = errors.New("aliyun: audio URL not found in workflow graph")
	ErrUnsupportedModel   = errors.New("aliyun: unsupported model")

	// Tripo 3D 专用：必须提供 prompt / image / images 三者之一。
	ErrMissingTripoInput = errors.New("aliyun tripo: provide one of prompt, image, or images (2-4)")
	// Tripo 3D 多图模式上限 4 张。
	ErrTooManyTripoImages = errors.New("aliyun tripo: images must be 2-4 entries")
	// Tripo 3D prompt 字符上限 1024（UTF-8 字符数）。
	ErrTripoPromptTooLong = errors.New("aliyun tripo: prompt exceeds 1024 characters")

	// Wan video-edit 必须恰好包含 1 个 video。
	ErrWanVideoEditMissingVideo = errors.New("aliyun wan: video-edit requires exactly 1 video")
	// Wan video-edit 参考图上限 4 张。
	ErrWanVideoEditTooManyImages = errors.New("aliyun wan: video-edit reference images must be 0-4")

	// HappyHorse r2v 参考图上限 9 张。
	ErrTooManyHappyHorseImages = errors.New("aliyun happyhorse: reference images must be 1-9")
	// HappyHorse video-edit 必须恰好包含 1 个 video。
	ErrHappyHorseVideoEditMissingVideo = errors.New("aliyun happyhorse: video-edit requires exactly 1 video")
	// HappyHorse video-edit 参考图上限 5 张。
	ErrHappyHorseVideoEditTooManyImages = errors.New("aliyun happyhorse: video-edit reference images must be 0-5")
)
