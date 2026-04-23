package openai

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
}

// ModelInfos returns i18n metadata for OpenAI embedding models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "text-embedding-3-small",
			Provider:    "embed/openai",
			DisplayName: engine.DisplayName{"en": "Text Embedding 3 Small", "zh": "Text Embedding 3 Small"},
			Description: engine.DisplayName{"en": "Efficient text embedding model (1536d)", "zh": "高效文本嵌入模型 (1536维)"},
			Intro:       engine.DisplayName{"en": "OpenAI's latest small embedding model with strong performance, supporting MRL dimension truncation for flexible storage.", "zh": "OpenAI 最新小型嵌入模型，性能优异，支持 MRL 维度截断以灵活存储。"},
			DocURL:      "https://platform.openai.com/docs/guides/embeddings",
			Capability:  "embedding",
		},
		{
			Name:        "text-embedding-3-large",
			Provider:    "embed/openai",
			DisplayName: engine.DisplayName{"en": "Text Embedding 3 Large", "zh": "Text Embedding 3 Large"},
			Description: engine.DisplayName{"en": "High-performance text embedding model (3072d)", "zh": "高性能文本嵌入模型 (3072维)"},
			Intro:       engine.DisplayName{"en": "OpenAI's largest embedding model offering the best retrieval performance with 3072 dimensions and MRL support.", "zh": "OpenAI 最大嵌入模型，提供 3072 维最佳检索性能，支持 MRL。"},
			DocURL:      "https://platform.openai.com/docs/guides/embeddings",
			Capability:  "embedding",
		},
		{
			Name:        "text-embedding-ada-002",
			Provider:    "embed/openai",
			DisplayName: engine.DisplayName{"en": "Text Embedding Ada 002", "zh": "Text Embedding Ada 002"},
			Description: engine.DisplayName{"en": "Legacy text embedding model (1536d)", "zh": "旧版文本嵌入模型 (1536维)"},
			Intro:       engine.DisplayName{"en": "OpenAI's previous-generation embedding model, still widely supported but superseded by text-embedding-3 series.", "zh": "OpenAI 上一代嵌入模型，仍广泛支持但已被 text-embedding-3 系列取代。"},
			DocURL:      "https://platform.openai.com/docs/guides/embeddings",
			Capability:  "embedding",
			Deprecated:  true,
		},
	}
}
