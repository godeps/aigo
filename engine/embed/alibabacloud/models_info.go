package alibabacloud

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
}

// ModelInfos returns i18n metadata for DashScope embedding models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "text-embedding-v3",
			Provider:    "embed/alibabacloud",
			DisplayName: engine.DisplayName{"en": "Text Embedding V3", "zh": "通义文本嵌入 V3"},
			Description: engine.DisplayName{"en": "Multilingual text embedding with MRL (1024d)", "zh": "多语言文本嵌入，支持 MRL (1024维)"},
			Intro:       engine.DisplayName{"en": "DashScope's latest text embedding model with 1024 dimensions, multilingual support, and Matryoshka Representation Learning for flexible dimension truncation.", "zh": "百炼最新文本嵌入模型，1024 维，支持多语言和 MRL 灵活维度截断。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/",
			Capability:  "embedding",
		},
		{
			Name:        "text-embedding-v2",
			Provider:    "embed/alibabacloud",
			DisplayName: engine.DisplayName{"en": "Text Embedding V2", "zh": "通义文本嵌入 V2"},
			Description: engine.DisplayName{"en": "Text embedding model (1536d)", "zh": "文本嵌入模型 (1536维)"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/",
			Capability:  "embedding",
		},
		{
			Name:        "text-embedding-v1",
			Provider:    "embed/alibabacloud",
			DisplayName: engine.DisplayName{"en": "Text Embedding V1", "zh": "通义文本嵌入 V1"},
			Description: engine.DisplayName{"en": "Legacy text embedding model (1536d)", "zh": "旧版文本嵌入模型 (1536维)"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/",
			Capability:  "embedding",
			Deprecated:  true,
		},
		{
			Name:        "multimodal-embedding-one-peace-v1",
			Provider:    "embed/alibabacloud",
			DisplayName: engine.DisplayName{"en": "Multimodal Embedding ONE-PEACE V1", "zh": "多模态嵌入 ONE-PEACE V1"},
			Description: engine.DisplayName{"en": "Multimodal embedding for text, image, and audio", "zh": "文本、图片和音频多模态嵌入"},
			Intro:       engine.DisplayName{"en": "DashScope's multimodal embedding model based on ONE-PEACE architecture, supporting text, image, and audio inputs in a unified embedding space.", "zh": "百炼基于 ONE-PEACE 架构的多模态嵌入模型，在统一嵌入空间中支持文本、图片和音频输入。"},
			DocURL:      "https://help.aliyun.com/zh/model-studio/",
			Capability:  "embedding",
		},
	}
}
