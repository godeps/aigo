package replicate

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for the Replicate platform.
// Replicate hosts a wide range of open-source models; this registers
// a platform-level entry for discovery purposes.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "replicate-model",
			Provider:    "replicate",
			DisplayName: engine.DisplayName{"en": "Replicate Model", "zh": "Replicate 模型"},
			Description: engine.DisplayName{"en": "Open-source model execution on Replicate", "zh": "Replicate 平台开源模型执行"},
			Intro:       engine.DisplayName{"en": "Replicate hosts and runs open-source machine learning models in the cloud with a simple prediction API.", "zh": "Replicate 在云端托管和运行开源机器学习模型，提供简洁的预测 API。"},
			DocURL:      "https://replicate.com/docs",
			Capability:  "image",
		},
	}
}
