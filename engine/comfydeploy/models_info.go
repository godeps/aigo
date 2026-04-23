package comfydeploy

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for the ComfyDeploy platform.
// ComfyDeploy models are hosted ComfyUI workflows; this registers
// a platform-level entry for discovery purposes.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "comfydeploy-workflow",
			Provider:    "comfydeploy",
			DisplayName: engine.DisplayName{"en": "ComfyDeploy Workflow", "zh": "ComfyDeploy 工作流"},
			Description: engine.DisplayName{"en": "Hosted ComfyUI workflow execution on ComfyDeploy", "zh": "ComfyDeploy 托管的 ComfyUI 工作流执行"},
			Intro:       engine.DisplayName{"en": "ComfyDeploy provides hosted ComfyUI workflow execution with API access for production deployments.", "zh": "ComfyDeploy 提供托管的 ComfyUI 工作流执行，支持生产环境 API 访问。"},
			DocURL:      "https://docs.comfydeploy.com/",
			Capability:  "image",
		},
	}
}
