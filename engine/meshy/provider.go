package meshy

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("meshy", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Endpoint:          cfg.Meta("endpoint", ""),
			WaitForCompletion: true,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for meshy.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "meshy",
		Configs: []engine.ProviderConfig{
			{
				Name:        "meshy-3d",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"MESHY_API_KEY"},
				DisplayName: engine.LookupDisplayName("meshy"),
			},
		},
	}
}
