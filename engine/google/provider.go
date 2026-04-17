package google

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("google", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for google.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "google",
		Configs: []engine.ProviderConfig{
			{
				Name:        "google-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"GOOGLE_API_KEY"},
				DisplayName: engine.LookupDisplayName("google"),
			},
		},
	}
}
