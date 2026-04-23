package engine

// EngineMetadata holds engine-level i18n metadata including display name,
// introduction, and documentation URL.
type EngineMetadata struct {
	DisplayName DisplayName `json:"display_name"`
	Intro       DisplayName `json:"intro,omitempty"`
	DocURL      string      `json:"doc_url,omitempty"`
}

// EngineMetadataMap maps engine package names to their full metadata.
var EngineMetadataMap = map[string]EngineMetadata{
	// Image Generation
	"alibabacloud": {
		DisplayName: DisplayName{"en": "Alibaba Cloud DashScope", "zh": "阿里云百炼"},
		Intro:       DisplayName{"en": "Alibaba Cloud's DashScope platform provides access to Qwen, Wan, and Z-Image models for image, video, TTS, and voice design generation.", "zh": "阿里云百炼平台提供通义万相、Wan、Z-Image 等模型，支持图片、视频、语音合成和声音设计生成。"},
		DocURL:      "https://help.aliyun.com/zh/model-studio/",
	},
	"openai": {
		DisplayName: DisplayName{"en": "OpenAI DALL-E", "zh": "OpenAI DALL-E"},
		Intro:       DisplayName{"en": "OpenAI's DALL-E models generate images from text prompts with high quality and creative flexibility.", "zh": "OpenAI 的 DALL-E 模型通过文本提示生成高质量、富有创意的图片。"},
		DocURL:      "https://platform.openai.com/docs/guides/images",
	},
	"google": {
		DisplayName: DisplayName{"en": "Google Imagen", "zh": "Google Imagen"},
		Intro:       DisplayName{"en": "Google's Imagen models deliver photorealistic image generation with strong text rendering and prompt adherence.", "zh": "Google Imagen 模型提供照片级真实感的图片生成，具备出色的文字渲染和提示词遵循能力。"},
		DocURL:      "https://cloud.google.com/vertex-ai/generative-ai/docs/image/overview",
	},
	"flux": {
		DisplayName: DisplayName{"en": "FLUX by Black Forest Labs", "zh": "FLUX"},
		Intro:       DisplayName{"en": "FLUX by Black Forest Labs offers state-of-the-art open-source image generation with multiple model tiers for different quality-speed tradeoffs.", "zh": "Black Forest Labs 的 FLUX 提供先进的开源图片生成，多个模型层级满足不同质量-速度需求。"},
		DocURL:      "https://docs.bfl.ml/",
	},
	"stability": {
		DisplayName: DisplayName{"en": "Stability AI", "zh": "Stability AI"},
		Intro:       DisplayName{"en": "Stability AI provides Stable Diffusion 3, Ultra, and Core models for versatile image generation with fine-grained control.", "zh": "Stability AI 提供 Stable Diffusion 3、Ultra 和 Core 模型，支持精细控制的多场景图片生成。"},
		DocURL:      "https://platform.stability.ai/docs/api-reference",
	},
	"ideogram": {
		DisplayName: DisplayName{"en": "Ideogram", "zh": "Ideogram"},
		Intro:       DisplayName{"en": "Ideogram specializes in text-accurate image generation, excelling at rendering readable text within generated images.", "zh": "Ideogram 专注于文字精准的图片生成，擅长在生成图片中渲染可读文字。"},
		DocURL:      "https://developer.ideogram.ai/api-reference",
	},
	"recraft": {
		DisplayName: DisplayName{"en": "Recraft", "zh": "Recraft"},
		Intro:       DisplayName{"en": "Recraft V3 generates production-ready vector and raster images with consistent brand style support.", "zh": "Recraft V3 生成可直接投产的矢量和位图，支持品牌风格一致性。"},
		DocURL:      "https://www.recraft.ai/docs",
	},
	"midjourney": {
		DisplayName: DisplayName{"en": "Midjourney", "zh": "Midjourney"},
		Intro:       DisplayName{"en": "Midjourney produces highly aesthetic images via the GoAPI proxy, known for artistic and cinematic quality.", "zh": "Midjourney 通过 GoAPI 代理生成高美感图片，以艺术性和电影感著称。"},
		DocURL:      "https://docs.goapi.ai/",
	},
	"jimeng": {
		DisplayName: DisplayName{"en": "Jimeng", "zh": "即梦"},
		Intro:       DisplayName{"en": "Jimeng (Volcengine) provides AI-powered image and video generation optimized for Chinese creative content.", "zh": "即梦（火山引擎）提供 AI 驱动的图片和视频生成，针对中文创意内容优化。"},
		DocURL:      "https://www.volcengine.com/docs/6791",
	},
	"liblib": {
		DisplayName: DisplayName{"en": "LibLibAI", "zh": "哩布哩布AI"},
		Intro:       DisplayName{"en": "LibLibAI is a ComfyUI-based platform offering hosted workflow execution with HMAC-SHA1 authentication.", "zh": "哩布哩布AI 是基于 ComfyUI 的平台，提供托管工作流执行，使用 HMAC-SHA1 认证。"},
		DocURL:      "https://www.liblib.art/",
	},
	"ark": {
		DisplayName: DisplayName{"en": "Volcengine Ark", "zh": "火山引擎方舟"},
		Intro:       DisplayName{"en": "Volcengine Ark provides access to Doubao and other models for image generation and multimodal tasks.", "zh": "火山引擎方舟提供豆包等模型，支持图片生成和多模态任务。"},
		DocURL:      "https://www.volcengine.com/docs/82379",
	},

	// Video Generation
	"kling": {
		DisplayName: DisplayName{"en": "Kling AI", "zh": "可灵 AI"},
		Intro:       DisplayName{"en": "Kling AI by Kuaishou delivers high-fidelity video generation with realistic physics simulation and motion coherence across multiple model versions.", "zh": "快手可灵 AI 提供高保真视频生成，具备真实物理模拟和动作一致性，支持多个模型版本。"},
		DocURL:      "https://docs.qingque.cn/d/home/eZQCm3mMOoGUqbJH0MrPnVXknYg",
	},
	"hailuo": {
		DisplayName: DisplayName{"en": "Hailuo Video", "zh": "海螺视频"},
		Intro:       DisplayName{"en": "Hailuo (MiniMax Video) generates cinematic-quality videos with strong motion dynamics and scene transitions.", "zh": "海螺视频（MiniMax）生成电影级视频，具备流畅的运动动态和场景切换。"},
		DocURL:      "https://platform.minimaxi.com/document/video-generation",
	},
	"luma": {
		DisplayName: DisplayName{"en": "Luma Dream Machine", "zh": "Luma Dream Machine"},
		Intro:       DisplayName{"en": "Luma Dream Machine generates realistic videos from text and image prompts with strong 3D spatial understanding.", "zh": "Luma Dream Machine 通过文本和图片提示生成真实感视频，具备出色的 3D 空间理解。"},
		DocURL:      "https://docs.lumalabs.ai/",
	},
	"runway": {
		DisplayName: DisplayName{"en": "Runway", "zh": "Runway"},
		Intro:       DisplayName{"en": "Runway Gen-3/Gen-4 provides professional-grade video generation with precise motion control and cinematic output.", "zh": "Runway Gen-3/Gen-4 提供专业级视频生成，具备精准运动控制和电影级输出。"},
		DocURL:      "https://docs.dev.runwayml.com/",
	},
	"pika": {
		DisplayName: DisplayName{"en": "Pika Labs", "zh": "Pika Labs"},
		Intro:       DisplayName{"en": "Pika Labs generates creative videos with expressive motion and stylistic flexibility from text and image inputs.", "zh": "Pika Labs 通过文本和图片输入生成富有表现力和风格灵活性的创意视频。"},
		DocURL:      "https://pika.art/",
	},
	"hedra": {
		DisplayName: DisplayName{"en": "Hedra", "zh": "Hedra"},
		Intro:       DisplayName{"en": "Hedra specializes in talking head video generation, creating realistic lip-synced character animations.", "zh": "Hedra 专注于数字人视频生成，创建逼真的口型同步角色动画。"},
		DocURL:      "https://www.hedra.com/",
	},

	// Audio / Music
	"elevenlabs": {
		DisplayName: DisplayName{"en": "ElevenLabs", "zh": "ElevenLabs"},
		Intro:       DisplayName{"en": "ElevenLabs provides industry-leading text-to-speech with natural-sounding voices, voice cloning, and multilingual support.", "zh": "ElevenLabs 提供业界领先的语音合成，具备自然音色、声音克隆和多语言支持。"},
		DocURL:      "https://elevenlabs.io/docs/api-reference",
	},
	"minimax": {
		DisplayName: DisplayName{"en": "MiniMax", "zh": "MiniMax"},
		Intro:       DisplayName{"en": "MiniMax offers TTS and music generation with expressive voice synthesis and diverse musical styles.", "zh": "MiniMax 提供语音合成和音乐生成，具备丰富的语音表达力和多样音乐风格。"},
		DocURL:      "https://platform.minimaxi.com/document/T2A%20V2",
	},
	"suno": {
		DisplayName: DisplayName{"en": "Suno", "zh": "Suno"},
		Intro:       DisplayName{"en": "Suno generates full songs with vocals and instrumentation from text prompts, supporting multiple genres and styles.", "zh": "Suno 通过文本提示生成完整歌曲（含人声和伴奏），支持多种风格和流派。"},
		DocURL:      "https://suno.com/",
	},
	"volcvoice": {
		DisplayName: DisplayName{"en": "Volcengine Speech", "zh": "火山引擎语音"},
		Intro:       DisplayName{"en": "Volcengine Speech provides high-quality TTS with CosyVoice models supporting voice design and custom voice creation.", "zh": "火山引擎语音提供高质量语音合成，CosyVoice 模型支持声音设计和自定义音色创建。"},
		DocURL:      "https://www.volcengine.com/docs/6561",
	},

	// 3D Generation
	"meshy": {
		DisplayName: DisplayName{"en": "Meshy", "zh": "Meshy"},
		Intro:       DisplayName{"en": "Meshy converts text descriptions and images into 3D models with textures, suitable for games and AR/VR applications.", "zh": "Meshy 将文字描述和图片转换为带纹理的 3D 模型，适用于游戏和 AR/VR 应用。"},
		DocURL:      "https://docs.meshy.ai/",
	},

	// Multi-Modal Understanding
	"gemini": {
		DisplayName: DisplayName{"en": "Google Gemini", "zh": "Google Gemini"},
		Intro:       DisplayName{"en": "Google Gemini provides multimodal understanding and generation capabilities, handling text, images, and video analysis.", "zh": "Google Gemini 提供多模态理解和生成能力，支持文本、图片和视频分析。"},
		DocURL:      "https://ai.google.dev/gemini-api/docs",
	},
	"gpt4o": {
		DisplayName: DisplayName{"en": "OpenAI GPT-4o", "zh": "OpenAI GPT-4o"},
		Intro:       DisplayName{"en": "GPT-4o provides vision understanding capabilities for image analysis, description, and multimodal reasoning.", "zh": "GPT-4o 提供视觉理解能力，支持图片分析、描述和多模态推理。"},
		DocURL:      "https://platform.openai.com/docs/guides/vision",
	},

	// Multi-Backend / Gateway
	"newapi": {
		DisplayName: DisplayName{"en": "NewAPI Gateway", "zh": "NewAPI 网关"},
		Intro:       DisplayName{"en": "NewAPI Gateway routes requests to multiple backends (OpenAI, Kling, Jimeng, Sora, Qwen, Gemini) through a unified API.", "zh": "NewAPI 网关通过统一 API 将请求路由到多个后端（OpenAI、可灵、即梦、Sora、通义、Gemini）。"},
		DocURL:      "",
	},
	"openrouter": {
		DisplayName: DisplayName{"en": "OpenRouter", "zh": "OpenRouter"},
		Intro:       DisplayName{"en": "OpenRouter provides multi-provider routing, automatically selecting the best available provider for each model.", "zh": "OpenRouter 提供多供应商路由，自动为每个模型选择最佳可用供应商。"},
		DocURL:      "https://openrouter.ai/docs",
	},
	"fal": {
		DisplayName: DisplayName{"en": "Fal.ai", "zh": "Fal.ai"},
		Intro:       DisplayName{"en": "Fal.ai is a generic model runner supporting a wide range of open-source image, video, and audio generation models.", "zh": "Fal.ai 是通用模型运行器，支持多种开源图片、视频和音频生成模型。"},
		DocURL:      "https://fal.ai/docs",
	},
	"replicate": {
		DisplayName: DisplayName{"en": "Replicate", "zh": "Replicate"},
		Intro:       DisplayName{"en": "Replicate hosts and runs open-source machine learning models in the cloud with a simple API.", "zh": "Replicate 在云端托管和运行开源机器学习模型，提供简洁 API。"},
		DocURL:      "https://replicate.com/docs",
	},
	"comfydeploy": {
		DisplayName: DisplayName{"en": "ComfyDeploy", "zh": "ComfyDeploy"},
		Intro:       DisplayName{"en": "ComfyDeploy provides hosted ComfyUI workflow execution with API access for production deployments.", "zh": "ComfyDeploy 提供托管的 ComfyUI 工作流执行，支持生产环境 API 访问。"},
		DocURL:      "https://docs.comfydeploy.com/",
	},
	"comfyui": {
		DisplayName: DisplayName{"en": "ComfyUI", "zh": "ComfyUI"},
		Intro:       DisplayName{"en": "ComfyUI server integration via WebSocket, supporting custom workflow graphs for flexible image and video generation.", "zh": "ComfyUI 服务通过 WebSocket 集成，支持自定义工作流图实现灵活的图片和视频生成。"},
		DocURL:      "https://docs.comfy.org/",
	},
	"runninghub": {
		DisplayName: DisplayName{"en": "RunningHub", "zh": "RunningHub"},
		Intro:       DisplayName{"en": "RunningHub provides cloud-hosted ComfyUI execution with managed infrastructure and API-driven workflow orchestration.", "zh": "RunningHub 提供云端 ComfyUI 执行，具备托管基础设施和 API 驱动的工作流编排。"},
		DocURL:      "https://www.runninghub.ai/",
	},

	// Embedding
	"embed/alibabacloud": {
		DisplayName: DisplayName{"en": "DashScope Embeddings", "zh": "百炼向量嵌入"},
		Intro:       DisplayName{"en": "DashScope provides text and multimodal embedding models for semantic search and retrieval applications.", "zh": "百炼提供文本和多模态向量嵌入模型，适用于语义搜索和检索应用。"},
		DocURL:      "https://help.aliyun.com/zh/model-studio/",
	},
	"embed/openai": {
		DisplayName: DisplayName{"en": "OpenAI Embeddings", "zh": "OpenAI Embeddings"},
		Intro:       DisplayName{"en": "OpenAI's text embedding models convert text into vector representations for search, clustering, and classification.", "zh": "OpenAI 文本嵌入模型将文本转换为向量表示，用于搜索、聚类和分类。"},
		DocURL:      "https://platform.openai.com/docs/guides/embeddings",
	},
	"embed/gemini": {
		DisplayName: DisplayName{"en": "Gemini Embeddings", "zh": "Gemini Embeddings"},
		Intro:       DisplayName{"en": "Google Gemini embedding models provide high-quality text embeddings with strong multilingual support.", "zh": "Google Gemini 嵌入模型提供高质量文本嵌入，具备强大的多语言支持。"},
		DocURL:      "https://ai.google.dev/gemini-api/docs/embeddings",
	},
	"embed/jina": {
		DisplayName: DisplayName{"en": "Jina Embeddings", "zh": "Jina Embeddings"},
		Intro:       DisplayName{"en": "Jina Embeddings provide multilingual text embedding models optimized for search and retrieval tasks.", "zh": "Jina Embeddings 提供多语言文本嵌入模型，针对搜索和检索任务优化。"},
		DocURL:      "https://jina.ai/embeddings/",
	},
	"embed/voyage": {
		DisplayName: DisplayName{"en": "Voyage AI Embeddings", "zh": "Voyage AI Embeddings"},
		Intro:       DisplayName{"en": "Voyage AI provides embedding models with strong performance on retrieval and semantic similarity benchmarks.", "zh": "Voyage AI 提供嵌入模型，在检索和语义相似度基准测试中表现出色。"},
		DocURL:      "https://docs.voyageai.com/",
	},
}

// EngineDisplayNames provides backward-compatible access to engine display names.
// Deprecated: Use EngineMetadataMap for full metadata access.
var EngineDisplayNames = func() map[string]DisplayName {
	m := make(map[string]DisplayName, len(EngineMetadataMap))
	for k, v := range EngineMetadataMap {
		m[k] = v.DisplayName
	}
	return m
}()

// LookupDisplayName returns the display name for the given engine key.
// If the key is not found, it returns a DisplayName with the key itself as both EN and ZH.
func LookupDisplayName(key string) DisplayName {
	if meta, ok := EngineMetadataMap[key]; ok {
		return meta.DisplayName
	}
	return DisplayName{"en": key, "zh": key}
}

// LookupEngineMetadata returns the full metadata for the given engine key.
// If the key is not found, it returns an EngineMetadata with the key as display name.
func LookupEngineMetadata(key string) EngineMetadata {
	if meta, ok := EngineMetadataMap[key]; ok {
		return meta
	}
	return EngineMetadata{
		DisplayName: DisplayName{"en": key, "zh": key},
	}
}
