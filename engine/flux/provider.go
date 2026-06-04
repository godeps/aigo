package flux

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("flux", func(cfg engine.EngineConfig) (engine.Engine, error) {
		// All FLUX image generations are async — without polling the engine
		// returns a task_id UUID instead of an image URL.
		wait := cfg.WaitForCompletionOr(true)
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			HTTPClient:        cfg.HTTPClient,
			Model:             cfg.Model,
			WaitForCompletion: wait,
			PollInterval:      cfg.PollInterval,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for flux.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "flux",
		Configs: []engine.ProviderConfig{
			{
				Name:        "flux-image",
				Engine:      New(Config{Model: ModelDev}),
				EnvVars:     []string{"BFL_API_KEY"},
				DisplayName: engine.LookupDisplayName("flux"),
			},
		},
	}
}
