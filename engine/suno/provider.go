package suno

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("suno", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for suno.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "suno",
		Configs: []engine.ProviderConfig{
			{
				Name:        "suno-music",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"SUNO_API_KEY"},
				DisplayName: engine.LookupDisplayName("suno"),
			},
		},
	}
}
