package minimax

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("minimax", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for minimax.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "minimax",
		Configs: []engine.ProviderConfig{
			{
				Name:        "minimax-video",
				Engine:      New(Config{}),
				EnvVars:     []string{"MINIMAX_API_KEY"},
				DisplayName: engine.LookupDisplayName("minimax"),
			},
			{
				Name:        "minimax-music",
				Engine:      New(Config{}),
				EnvVars:     []string{"MINIMAX_API_KEY"},
				DisplayName: engine.LookupDisplayName("minimax"),
			},
		},
	}
}
