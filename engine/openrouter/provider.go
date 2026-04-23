package openrouter

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("openrouter", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for openrouter.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "openrouter",
		Configs: []engine.ProviderConfig{
			{
				Name:        "openrouter",
				Engine:      New(Config{}),
				EnvVars:     []string{"OPENROUTER_API_KEY"},
				DisplayName: engine.LookupDisplayName("openrouter"),
			},
		},
	}
}
