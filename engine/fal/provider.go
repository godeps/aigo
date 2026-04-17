package fal

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("fal", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			QueueURL:          cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for fal.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "fal",
		Configs: []engine.ProviderConfig{
			{
				Name:        "fal",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"FAL_KEY"},
				DisplayName: engine.LookupDisplayName("fal"),
			},
		},
	}
}
