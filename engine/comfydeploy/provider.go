package comfydeploy

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
	engine.RegisterFactory("comfydeploy", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return New(Config{
			APIToken:          cfg.APIKey,
			BaseURL:           cfg.BaseURL,
			DeploymentID:      cfg.Meta("deploymentId", ""),
			WaitForCompletion: true,
		}), nil
	})
}

// DefaultProvider returns preset engine configurations for comfydeploy.
func DefaultProvider() engine.Provider {
	return engine.Provider{
		Name: "comfydeploy",
		Configs: []engine.ProviderConfig{
			{
				Name:        "comfydeploy",
				Engine:      New(Config{}),
				EnvVars:     []string{"COMFYDEPLOY_API_TOKEN"},
				DisplayName: engine.LookupDisplayName("comfydeploy"),
			},
		},
	}
}
