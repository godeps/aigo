package gpt4o

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("gpt4o", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for gpt4o.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "gpt4o",
		Configs: []engine.ProviderConfig{
			{
				Name:        "gpt4o",
				Engine:      New(Config{}),
				EnvVars:     []string{"OPENAI_API_KEY"},
				DisplayName: engine.LookupDisplayName("gpt4o"),
			},
		},
	}
}
