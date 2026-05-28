package newapi

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
	engine.RegisterFactory("newapi", func(cfg engine.EngineConfig) (engine.Engine, error) {
		// Resolve route and kind via three-tier decision:
		// knownModels > model name inference > cfg.Capability fallback.
		route, kind, _ := LookupRoute(cfg.Model, cfg.Capability)

		// Most New API gateway routes (Kling/Jimeng/Sora) are async; without
		// polling the engine returns a task_id UUID instead of a media URL.
		wait := cfg.WaitForCompletionOr(true)
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			Model:             cfg.Model,
			Route:             route,
			Kind:              kind,
			WaitForCompletion: wait,
			PollInterval:      cfg.PollInterval,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for newapi.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "newapi",
		Configs: []engine.ProviderConfig{
			{
				Name:        "newapi",
				Engine:      New(Config{}),
				EnvVars:     []string{"NEWAPI_API_KEY"},
				DisplayName: engine.LookupDisplayName("newapi"),
			},
		},
	}
}
