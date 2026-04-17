package hailuo

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("hailuo", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for hailuo.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "hailuo",
		Configs: []engine.ProviderConfig{
			{
				Name:        "hailuo-video",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"HAILUO_API_KEY"},
				DisplayName: engine.LookupDisplayName("hailuo"),
			},
		},
	}
}
