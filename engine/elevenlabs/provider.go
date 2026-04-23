package elevenlabs

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("elevenlabs", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			VoiceID: cfg.Meta("voiceId", ""),
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for elevenlabs.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "elevenlabs",
		Configs: []engine.ProviderConfig{
			{
				Name:        "elevenlabs-tts",
				Engine:      New(Config{Model: ModelMultilingualV2}),
				EnvVars:     []string{"ELEVENLABS_API_KEY"},
				DisplayName: engine.LookupDisplayName("elevenlabs"),
			},
		},
	}
}
