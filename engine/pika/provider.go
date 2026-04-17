package pika

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("pika", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for pika.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "pika",
		Configs: []engine.ProviderConfig{
			{
				Name:        "pika-video",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"PIKA_API_KEY"},
				DisplayName: engine.LookupDisplayName("pika"),
			},
		},
	}
}
