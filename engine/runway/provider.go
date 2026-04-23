package runway

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("runway", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for runway.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "runway",
		Configs: []engine.ProviderConfig{
			{
				Name:        "runway-video",
				Engine:      New(Config{Model: ModelGen4Turbo, WaitForCompletion: true}),
				EnvVars:     []string{"RUNWAY_API_KEY"},
				DisplayName: engine.LookupDisplayName("runway"),
			},
		},
	}
}
