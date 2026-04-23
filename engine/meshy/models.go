package meshy

import "github.com/godeps/aigo/engine"

const (
	modelText3D  = "text-to-3d"
	modelImage3D = "image-to-3d"
)

// ModelInfos returns i18n metadata for all Meshy models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        modelText3D,
			Provider:    "meshy",
			DisplayName: engine.DisplayName{"en": "Text to 3D", "zh": "文本生成 3D"},
			Description: engine.DisplayName{"en": "Text-to-3D model generation", "zh": "文本生成 3D 模型"},
			Intro:       engine.DisplayName{"en": "Meshy Text-to-3D generates production-ready 3D models from natural language descriptions with automated UV unwrapping and PBR texture generation, suitable for game assets and product visualization.", "zh": "Meshy 文本生成 3D 从自然语言描述生成可生产就绪的 3D 模型，自动 UV 展开和 PBR 纹理生成，适用于游戏资产和产品可视化。"},
			DocURL:      "https://docs.meshy.ai/",
			Capability:  "3d",
		},
		{
			Name:        modelImage3D,
			Provider:    "meshy",
			DisplayName: engine.DisplayName{"en": "Image to 3D", "zh": "图片生成 3D"},
			Description: engine.DisplayName{"en": "Image-to-3D model generation", "zh": "图片生成 3D 模型"},
			Intro:       engine.DisplayName{"en": "Meshy Image-to-3D reconstructs detailed 3D models from single or multi-view images, preserving surface texture and geometry for use in 3D pipelines, AR/VR, and e-commerce.", "zh": "Meshy 图片生成 3D 从单视图或多视图图像重建详细 3D 模型，保留表面纹理和几何形状，适用于 3D 流程、AR/VR 和电商。"},
			DocURL:      "https://docs.meshy.ai/",
			Capability:  "3d",
		},
	}
}
