package hedra

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("hedra", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for hedra.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "hedra",
		Configs: []engine.ProviderConfig{
			{
				Name:        "hedra-video",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"HEDRA_API_KEY"},
				DisplayName: engine.LookupDisplayName("hedra"),
			},
		},
	}
}
