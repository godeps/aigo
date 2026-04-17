package volcvoice

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("volcvoice", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			AccessToken: cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for volcvoice.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "volcvoice",
		Configs: []engine.ProviderConfig{
			{
				Name:        "volcvoice-tts",
				Engine:      New(Config{}),
				EnvVars:     []string{"VOLC_SPEECH_ACCESS_TOKEN"},
				DisplayName: engine.LookupDisplayName("volcvoice"),
			},
		},
	}
}
