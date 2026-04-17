package newapi

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("newapi", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for newapi.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "newapi",
		Configs: []engine.ProviderConfig{
			{
				Name:        "newapi",
				Engine:      New(Config{}),
				EnvVars:     []string{"NEWAPI_API_KEY"},
				DisplayName: engine.LookupDisplayName("newapi"),
			},
		},
	}
}
