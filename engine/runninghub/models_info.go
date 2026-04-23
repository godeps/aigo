package runninghub

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for the RunningHub platform.
// RunningHub models are cloud-hosted ComfyUI workflows; this registers
// a platform-level entry for discovery purposes.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "runninghub-comfyui",
			Provider:    "runninghub",
			DisplayName: engine.DisplayName{"en": "RunningHub ComfyUI", "zh": "RunningHub ComfyUI"},
			Description: engine.DisplayName{"en": "Cloud-hosted ComfyUI workflows on RunningHub", "zh": "RunningHub 云端 ComfyUI 工作流"},
			Intro:       engine.DisplayName{"en": "RunningHub provides cloud-hosted ComfyUI execution with managed infrastructure and API-driven workflow orchestration.", "zh": "RunningHub 提供云端 ComfyUI 执行，具备托管基础设施和 API 驱动的工作流编排。"},
			DocURL:      "https://www.runninghub.ai/",
			Capability:  "image",
		},
	}
}
