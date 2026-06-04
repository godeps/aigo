package ffmpeg

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterFactory("ffmpeg", func(cfg engine.EngineConfig) (engine.Engine, error) {
		mode := ModeMix
		if cfg.Capability == "sfx" {
			mode = ModeSFX
		}
		return New(Config{
			Mode:         mode,
			OutputFormat: cfg.Meta("outputFormat", "mp3"),
			HTTPClient:   cfg.HTTPClient,
		}), nil
	})
	engine.RegisterModelInfos(ModelInfos())
}

// DefaultProvider returns preset engine configurations for ffmpeg.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "ffmpeg",
		Configs: []engine.ProviderConfig{
			{
				Name:        "ffmpeg-sfx",
				Engine:      New(Config{Mode: ModeSFX}),
				DisplayName: engine.LookupDisplayName("ffmpeg"),
			},
			{
				Name:        "ffmpeg-audio-mix",
				Engine:      New(Config{Mode: ModeMix}),
				DisplayName: engine.LookupDisplayName("ffmpeg"),
			},
		},
	}
}
