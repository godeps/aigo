package jimeng

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("jimeng", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			Endpoint:          cfg.Meta("endpoint", ""),
			WaitForCompletion: true,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for jimeng.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "jimeng",
		Configs: []engine.ProviderConfig{
			{
				Name:        "jimeng-image",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"JIMENG_API_KEY"},
				DisplayName: engine.LookupDisplayName("jimeng"),
			},
		},
	}
}
