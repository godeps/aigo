package ark

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("ark", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for ark.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "ark",
		Configs: []engine.ProviderConfig{
			{
				Name:        "ark-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"ARK_API_KEY"},
				DisplayName: engine.LookupDisplayName("ark"),
			},
		},
	}
}
