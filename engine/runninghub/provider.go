package runninghub

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("runninghub", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for runninghub.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "runninghub",
		Configs: []engine.ProviderConfig{
			{
				Name:        "runninghub",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"RH_API_KEY"},
				DisplayName: engine.LookupDisplayName("runninghub"),
			},
		},
	}
}
