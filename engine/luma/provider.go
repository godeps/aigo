package luma

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("luma", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for luma.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "luma",
		Configs: []engine.ProviderConfig{
			{
				Name:        "luma-video",
				Engine:      New(Config{Model: ModelRay2, WaitForCompletion: true}),
				EnvVars:     []string{"LUMA_API_KEY"},
				DisplayName: engine.LookupDisplayName("luma"),
			},
			{
				Name:        "luma-image",
				Engine:      New(Config{Model: ModelPhoton1, WaitForCompletion: true}),
				EnvVars:     []string{"LUMA_API_KEY"},
				DisplayName: engine.LookupDisplayName("luma"),
			},
		},
	}
}
