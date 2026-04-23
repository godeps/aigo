package liblib

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
	engine.RegisterFactory("liblib", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			AccessKey:         cfg.APIKey,
			SecretKey:         cfg.Meta("secretKey", ""),
			BaseURL:           cfg.BaseURL,
			Endpoint:          cfg.Meta("endpoint", ""),
			TemplateUUID:      cfg.Meta("templateUuid", ""),
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for liblib.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "liblib",
		Configs: []engine.ProviderConfig{
			{
				Name:        "liblib-image",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"LIBLIB_ACCESS_KEY", "LIBLIB_SECRET_KEY"},
				DisplayName: engine.LookupDisplayName("liblib"),
			},
		},
	}
}
