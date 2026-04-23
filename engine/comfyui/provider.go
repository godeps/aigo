package comfyui

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
	engine.RegisterFactory("comfyui", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIKey:            cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for comfyui.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "comfyui",
		Configs: []engine.ProviderConfig{
			{
				Name:        "comfyui",
				Engine:      New(Config{WaitForCompletion: true}),
				EnvVars:     []string{"COMFY_CLOUD_API_KEY"},
				DisplayName: engine.LookupDisplayName("comfyui"),
			},
		},
	}
}
