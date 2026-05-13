package ark

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("ark", func(cfg engine.EngineConfig) (engine.Engine, error) {
		// Smart default: image models are sync; everything else (video) is
		// async and silently returns a task_id without polling.
		wait := cfg.WaitForCompletionOr(!imageModels[cfg.Model])
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

// DefaultProvider returns preset engine configurations for ark.
func DefaultProvider() engine.Provider {
	base := engine.LookupDisplayName("ark")
	suffixed := func(en, zh string) engine.DisplayName {
		return engine.DisplayName{
			"en": base["en"] + " · " + en,
			"zh": base["zh"] + " · " + zh,
		}
	}
	return engine.Provider{
		Name: "ark",
		Configs: []engine.ProviderConfig{
			{
				Name:        "ark-image",
				Engine:      New(Config{Model: ModelSeedream3_0}),
				EnvVars:     []string{"ARK_API_KEY"},
				DisplayName: suffixed("Image", "图像"),
			},
			{
				Name:        "ark-video",
				Engine:      New(Config{Model: "doubao-seedance-2-0-260128", WaitForCompletion: true}),
				EnvVars:     []string{"ARK_API_KEY"},
				DisplayName: suffixed("Video", "视频"),
			},
			{
				Name:        "ark-video-fast",
				Engine:      New(Config{Model: "doubao-seedance-2-0-fast-260128", WaitForCompletion: true}),
				EnvVars:     []string{"ARK_API_KEY"},
				DisplayName: suffixed("Video Fast", "快速视频"),
			},
		},
	}
}
