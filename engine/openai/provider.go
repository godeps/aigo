package openai

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("openai", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for openai.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "openai",
		Configs: []engine.ProviderConfig{
			{
				Name:        "openai-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"OPENAI_API_KEY"},
				DisplayName: engine.LookupDisplayName("openai"),
			},
		},
	}
}
