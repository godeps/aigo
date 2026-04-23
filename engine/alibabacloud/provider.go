package alibabacloud

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("alibabacloud", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for alibabacloud.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "alibabacloud",
		Configs: []engine.ProviderConfig{
			{
				Name:        "alibabacloud-image",
				Engine:      New(Config{Model: ModelQwenImage}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: engine.LookupDisplayName("alibabacloud"),
			},
			{
				Name:        "alibabacloud-video",
				Engine:      New(Config{Model: ModelWanTextToVideo, WaitForCompletion: true}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: engine.LookupDisplayName("alibabacloud"),
			},
			{
				Name:        "alibabacloud-tts",
				Engine:      New(Config{Model: ModelQwenTTSFlash}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: engine.LookupDisplayName("alibabacloud"),
			},
		},
	}
}
