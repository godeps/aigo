package ark

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("ark", func(cfg engine.EngineConfig) (engine.Engine, error) {
		// Smart default: image models are sync; everything else (video) is
		// async and silently returns a task_id without polling.
		wait := cfg.WaitForCompletionOr(!imageModels[cfg.Model])
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			WaitForCompletion: wait,
			PollInterval:      cfg.PollInterval,
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
