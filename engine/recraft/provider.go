package recraft

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("recraft", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for recraft.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "recraft",
		Configs: []engine.ProviderConfig{
			{
				Name:        "recraft-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"RECRAFT_API_KEY"},
				DisplayName: engine.LookupDisplayName("recraft"),
			},
		},
	}
}
