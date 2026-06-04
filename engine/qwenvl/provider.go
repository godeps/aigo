package qwenvl

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("qwenvl", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.BaseURL,
			HTTPClient: cfg.ClientWithHooks(),
			Model:      cfg.Model,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for Qwen-VL.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "qwenvl",
		Configs: []engine.ProviderConfig{
			{
				Name:        "qwenvl",
				Engine:      New(Config{}),
				EnvVars:     []string{"DASHSCOPE_API_KEY"},
				DisplayName: engine.LookupDisplayName("qwenvl"),
			},
		},
	}
}
