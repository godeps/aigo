package comfyui

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for the ComfyUI server integration.
// ComfyUI models are user-defined workflow graphs; this registers
// a platform-level entry for discovery purposes.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "comfyui-workflow",
			Provider:    "comfyui",
			DisplayName: engine.DisplayName{"en": "ComfyUI Workflow", "zh": "ComfyUI 工作流"},
			Description: engine.DisplayName{"en": "Custom ComfyUI workflow execution via WebSocket", "zh": "通过 WebSocket 执行自定义 ComfyUI 工作流"},
			Intro:       engine.DisplayName{"en": "ComfyUI server integration supports custom workflow graphs for flexible image and video generation via WebSocket connection.", "zh": "ComfyUI 服务集成支持自定义工作流图，通过 WebSocket 连接实现灵活的图片和视频生成。"},
			DocURL:      "https://docs.comfy.org/",
			Capability:  "image",
		},
	}
}
