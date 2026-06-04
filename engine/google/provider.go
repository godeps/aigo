package google

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("google", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.BaseURL,
			HTTPClient: cfg.HTTPClient,
			Model:      cfg.Model,
		})
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for google.
func DefaultProvider() engine.Provider {
	e, _ := New(Config{})
	return engine.Provider{
		Name: "google",
		Configs: []engine.ProviderConfig{
			{
				Name:        "google-image",
				Engine:      e,
				EnvVars:     []string{"GOOGLE_API_KEY"},
				DisplayName: engine.LookupDisplayName("google"),
			},
		},
	}
}
