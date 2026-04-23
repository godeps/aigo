package gemini

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
}

// ModelInfos returns i18n metadata for Gemini embedding models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "gemini-embedding-2-preview",
			Provider:    "embed/gemini",
			DisplayName: engine.DisplayName{"en": "Gemini Embedding 2 Preview", "zh": "Gemini Embedding 2 预览版"},
			Description: engine.DisplayName{"en": "Google Gemini text embedding model", "zh": "Google Gemini 文本嵌入模型"},
			Intro:       engine.DisplayName{"en": "Google's Gemini embedding model with strong multilingual performance for semantic search and retrieval.", "zh": "Google Gemini 嵌入模型，在语义搜索和检索中具备出色的多语言性能。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs/embeddings",
			Capability:  "embedding",
		},
	}
}
