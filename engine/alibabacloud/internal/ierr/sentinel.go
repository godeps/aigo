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
)
