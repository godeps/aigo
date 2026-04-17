package midjourney

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("midjourney", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for midjourney.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "midjourney",
		Configs: []engine.ProviderConfig{
			{
				Name:        "midjourney-image",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"GOAPI_KEY"},
				DisplayName: engine.LookupDisplayName("midjourney"),
			},
		},
	}
}
