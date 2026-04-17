package kling

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("kling", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for kling.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "kling",
		Configs: []engine.ProviderConfig{
			{
				Name:        "kling-video",
				Engine:      New(Config{Model: ModelKlingV2Master, Endpoint: EndpointText2Video, WaitForCompletion: true}),
				EnvVars:     []string{"KLING_API_KEY"},
				DisplayName: engine.LookupDisplayName("kling"),
			},
			{
				Name:        "kling-image",
				Engine:      New(Config{Model: ModelKlingV2Master, Endpoint: EndpointImage, WaitForCompletion: true}),
				EnvVars:     []string{"KLING_API_KEY"},
				DisplayName: engine.LookupDisplayName("kling"),
			},
		},
	}
}
