package flux

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("flux", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for flux.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "flux",
		Configs: []engine.ProviderConfig{
			{
				Name:        "flux-image",
				Engine:      New(Config{Model: ModelDev}),
				EnvVars:     []string{"BFL_API_KEY"},
				DisplayName: engine.LookupDisplayName("flux"),
			},
		},
	}
}
