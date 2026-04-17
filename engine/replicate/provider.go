package replicate

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("replicate", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for replicate.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "replicate",
		Configs: []engine.ProviderConfig{
			{
				Name:        "replicate",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"REPLICATE_API_TOKEN"},
				DisplayName: engine.LookupDisplayName("replicate"),
			},
		},
	}
}
