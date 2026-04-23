package voyage

import "github.com/godeps/aigo/engine"

func init() {
	engine.RegisterModelInfos(ModelInfos())
}

// ModelInfos returns i18n metadata for Voyage AI embedding models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        "voyage-3",
			Provider:    "embed/voyage",
			DisplayName: engine.DisplayName{"en": "Voyage 3", "zh": "Voyage 3"},
			Description: engine.DisplayName{"en": "High-performance text embedding for retrieval", "zh": "高性能检索文本嵌入"},
			Intro:       engine.DisplayName{"en": "Voyage 3 is Voyage AI's latest embedding model with top-tier performance on retrieval and semantic similarity benchmarks.", "zh": "Voyage 3 是 Voyage AI 最新嵌入模型，在检索和语义相似度基准测试中表现顶尖。"},
			DocURL:      "https://docs.voyageai.com/",
			Capability:  "embedding",
		},
	}
}
