package openai

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("openai", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			HTTPClient:        cfg.ClientWithHooks(),
			Model:             cfg.Model,
			Quality:           cfg.MetaAny(cfg.Quality, "quality"),
			Style:             cfg.MetaAny(cfg.Style, "style"),
			Background:        cfg.MetaAny(cfg.Background, "background"),
			OutputFormat:      cfg.MetaAny(cfg.OutputFormat, "outputFormat", "output_format"),
			Moderation:        cfg.MetaAny(cfg.Moderation, "moderation"),
			OutputCompression: cfg.MetaIntAny(cfg.OutputCompression, "outputCompression", "output_compression"),
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for openai.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "openai",
		Configs: []engine.ProviderConfig{
			{
				Name:        "openai-image",
				Engine:      New(Config{}),
				EnvVars:     []string{"OPENAI_API_KEY"},
				DisplayName: engine.LookupDisplayName("openai"),
			},
		},
	}
}
