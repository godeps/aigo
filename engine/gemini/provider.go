package gemini

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("gemini", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for gemini.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "gemini",
		Configs: []engine.ProviderConfig{
			{
				Name:        "gemini",
				Engine:      New(Config{}),
				EnvVars:     []string{"GEMINI_API_KEY"},
				DisplayName: engine.LookupDisplayName("gemini"),
			},
		},
	}
}
