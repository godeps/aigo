package alibabacloud

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("alibabacloud", func(cfg engine.EngineConfig) (engine.Engine, error) {
		wait := cfg.WaitForCompletionOr(defaultWaitForModel(cfg.Model))
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: wait,
			PollInterval:      cfg.PollInterval,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// defaultWaitForModel returns the polling default for a DashScope model when
// the caller did not configure WaitForCompletion explicitly. Models routed
// through the X-DashScope-Async path (video/video edit/image edit/3D/ASR
// filetrans) are unusable without polling — calling code that does not poll
// gets back a task_id UUID instead of a media URL, which silently breaks
// downstream consumers (e.g. canvas writeback rendering an empty <video>).
//
// The classification mirrors the hand-built presets in DefaultProvider() so
// the auto-registered factory path produces the same behavior as the curated
// preset path.
func defaultWaitForModel(model string) bool {
	if model == "" {
		return false
	}
	if _, isEdit := editModels[model]; isEdit {
		return true
	}
	if model == ModelQwenASRFlashFiletrans {
		return true
	}
	switch model {
	case ModelWanTextToVideo,
		ModelWanImageToVideo,
		ModelWanReferenceVideo,
		ModelWanVideoEdit,
		ModelKlingV3Video,
		ModelKlingV3OmniVideo,
		ModelTripoP1,
		ModelTripoH31:
		return true
	}
	return false
}

// DefaultProvider returns preset engine configurations for alibabacloud.
//
// Each preset gets a capability-suffixed DisplayName so UI selectors can
// distinguish them at a glance (e.g. "阿里云百炼 · 3D").
func DefaultProvider() engine.Provider {
	base := engine.LookupDisplayName("alibabacloud")
	suffixed := func(en, zh string) engine.DisplayName {
		return engine.DisplayName{
			"en": base["en"] + " · " + en,
			"zh": base["zh"] + " · " + zh,
		}
	}
	return engine.Provider{
		Name: "alibabacloud",
		Configs: []engine.ProviderConfig{
			{
				Name:        "alibabacloud-image",
				Engine:      New(Config{Model: ModelQwenImage}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("Image", "图像"),
			},
			{
				Name:        "alibabacloud-image-edit",
				Engine:      New(Config{Model: ModelQwenImageEditPlus, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("Image Edit", "图像编辑"),
			},
			{
				Name:        "alibabacloud-video",
				Engine:      New(Config{Model: ModelWanTextToVideo, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("Video", "视频"),
			},
			{
				Name:        "alibabacloud-video-edit",
				Engine:      New(Config{Model: ModelWanVideoEdit, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("Video Edit", "视频编辑"),
			},
			{
				Name:        "alibabacloud-tts",
				Engine:      New(Config{Model: ModelQwenTTSFlash}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("TTS", "语音合成"),
			},
			{
				Name:        "alibabacloud-voice-design",
				Engine:      New(Config{Model: ModelQwenVoiceDesign}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("Voice Design", "声音设计"),
			},
			{
				Name:        "alibabacloud-asr",
				Engine:      New(Config{Model: ModelQwenASRFlash, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("ASR", "语音识别"),
			},
			{
				Name:        "alibabacloud-3d",
				Engine:      New(Config{Model: ModelTripoP1, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: suffixed("3D", "3D 资产"),
			},
		},
	}
}
