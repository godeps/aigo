package jina

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
}

// ModelInfos returns i18n metadata for Jina embedding models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "jina-clip-v2",
			Provider:    "embed/jina",
			DisplayName: engine.DisplayName{"en": "Jina CLIP V2", "zh": "Jina CLIP V2"},
			Description: engine.DisplayName{"en": "Multilingual multimodal embedding model", "zh": "多语言多模态嵌入模型"},
			Intro:       engine.DisplayName{"en": "Jina CLIP V2 is a multilingual multimodal embedding model supporting text and image inputs for cross-modal retrieval.", "zh": "Jina CLIP V2 是多语言多模态嵌入模型，支持文本和图片输入的跨模态检索。"},
			DocURL:      "https://jina.ai/embeddings/",
			Capability:  "embedding",
		},
	}
}
