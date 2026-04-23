package gemini

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Gemini models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelGemini20Flash,
			Provider:    "gemini",
			DisplayName: engine.DisplayName{"en": "Gemini 2.0 Flash", "zh": "Gemini 2.0 Flash"},
			Description: engine.DisplayName{"en": "Fast multimodal understanding", "zh": "快速多模态理解"},
			Intro:       engine.DisplayName{"en": "Gemini 2.0 Flash is Google's next-generation workhorse model with native multimodal output including image generation, optimized for high-throughput agentic tasks at low latency.", "zh": "Gemini 2.0 Flash 是 Google 下一代主力模型，原生支持包括图像生成在内的多模态输出，专为低延迟高吞吐量的智能体任务优化。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs",
			Capability:  "text",
		},
		{
			Name:        ModelGemini15Pro,
			Provider:    "gemini",
			DisplayName: engine.DisplayName{"en": "Gemini 1.5 Pro", "zh": "Gemini 1.5 Pro"},
			Description: engine.DisplayName{"en": "Advanced multimodal understanding", "zh": "高级多模态理解"},
			Intro:       engine.DisplayName{"en": "Gemini 1.5 Pro features a 1M token context window with advanced reasoning across text, images, audio, and video, excelling at long-document analysis and complex multimodal tasks.", "zh": "Gemini 1.5 Pro 具备 100 万 token 上下文窗口，在文本、图像、音频和视频上实现高级推理，擅长长文档分析和复杂多模态任务。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs",
			Capability:  "text",
		},
		{
			Name:        ModelGemini20FlashLite,
			Provider:    "gemini",
			DisplayName: engine.DisplayName{"en": "Gemini 2.0 Flash Lite", "zh": "Gemini 2.0 Flash 轻量版"},
			Description: engine.DisplayName{"en": "Lightweight fast multimodal model", "zh": "轻量快速多模态模型"},
			Intro:       engine.DisplayName{"en": "Gemini 2.0 Flash Lite is the most cost-efficient Gemini 2.0 model, providing fast multimodal responses at minimal cost for classification, summarization, and high-volume applications.", "zh": "Gemini 2.0 Flash Lite 是最具成本效益的 Gemini 2.0 模型，以最低成本提供快速多模态响应，适用于分类、摘要和高并发应用。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs",
			Capability:  "text",
		},
		{
			Name:        ModelGemini15Flash,
			Provider:    "gemini",
			DisplayName: engine.DisplayName{"en": "Gemini 1.5 Flash", "zh": "Gemini 1.5 Flash"},
			Description: engine.DisplayName{"en": "Fast multimodal understanding", "zh": "快速多模态理解"},
			Intro:       engine.DisplayName{"en": "Gemini 1.5 Flash delivers speed-optimized multimodal inference with a 1M token context window, balancing quality and cost for production-scale applications.", "zh": "Gemini 1.5 Flash 提供速度优化的多模态推理，具备 100 万 token 上下文窗口，在生产规模应用中平衡质量与成本。"},
			DocURL:      "https://ai.google.dev/gemini-api/docs",
			Capability:  "text",
		},
	}
}
