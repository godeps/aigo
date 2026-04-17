package stability

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("stability", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for stability.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "stability",
		Configs: []engine.ProviderConfig{
			{
				Name:        "stability-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"STABILITY_API_KEY"},
				DisplayName: engine.LookupDisplayName("stability"),
			},
		},
	}
}
