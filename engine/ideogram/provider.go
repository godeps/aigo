package ideogram

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("ideogram", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for ideogram.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "ideogram",
		Configs: []engine.ProviderConfig{
			{
				Name:        "ideogram-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"IDEOGRAM_API_KEY"},
				DisplayName: engine.LookupDisplayName("ideogram"),
			},
		},
	}
}
