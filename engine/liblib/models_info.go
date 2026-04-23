package liblib

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for the LibLibAI platform.
// LibLibAI models are user-configured ComfyUI workflows; this registers
// a platform-level entry for discovery purposes.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "liblib-comfyui",
			Provider:    "liblib",
			DisplayName: engine.DisplayName{"en": "LibLibAI ComfyUI", "zh": "哩布哩布AI ComfyUI"},
			Description: engine.DisplayName{"en": "User-configured ComfyUI workflows on LibLibAI", "zh": "哩布哩布AI 平台上用户配置的 ComfyUI 工作流"},
			Intro:       engine.DisplayName{"en": "LibLibAI hosts ComfyUI workflows with HMAC-SHA1 authentication, supporting custom image and video generation pipelines.", "zh": "哩布哩布AI 托管 ComfyUI 工作流，使用 HMAC-SHA1 认证，支持自定义图片和视频生成管线。"},
			DocURL:      "https://www.liblib.art/",
			Capability:  "image",
		},
	}
}
