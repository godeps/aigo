package newapi

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all known NewAPI gateway models.
// Model names are prefixed with "newapi/" to avoid conflicts with native engine registrations.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		// ── Image ──
		{Name: "newapi/gpt-image-2", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GPT Image 2", "zh": "GPT Image 2"}, Capability: "image"},
		{Name: "newapi/gpt-image-1", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GPT Image 1", "zh": "GPT Image 1"}, Capability: "image"},
		{Name: "newapi/dall-e-3", Provider: "newapi", DisplayName: engine.DisplayName{"en": "DALL-E 3", "zh": "DALL-E 3"}, Capability: "image"},
		{Name: "newapi/dall-e-2", Provider: "newapi", DisplayName: engine.DisplayName{"en": "DALL-E 2", "zh": "DALL-E 2"}, Capability: "image"},
		{Name: "newapi/qwen-max-vl", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen Max VL", "zh": "通义千问 VL"}, Capability: "image"},
		{Name: "newapi/qwen-image", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen Image", "zh": "通义万相文生图"}, Capability: "image"},
		{Name: "newapi/z-image", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Z-Image", "zh": "Z-Image"}, Capability: "image"},
		{Name: "newapi/qwen-image-edit", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen Image Edit", "zh": "通义万相图像编辑"}, Capability: "image_edit"},
		{Name: "newapi/qwen-image-edit-max", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen Image Edit Max", "zh": "通义万相图像编辑 Max"}, Capability: "image_edit"},
		{Name: "newapi/gemini-2.0-flash", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Gemini 2.0 Flash", "zh": "Gemini 2.0 Flash"}, Capability: "image"},

		// ── Video ──
		{Name: "newapi/kling-v2-master", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Kling V2 Master", "zh": "可灵 V2 大师版"}, Capability: "video"},
		{Name: "newapi/kling-v1-6-pro", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Kling V1.6 Pro", "zh": "可灵 V1.6 专业版"}, Capability: "video"},
		{Name: "newapi/kling-v1", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Kling V1", "zh": "可灵 V1"}, Capability: "video"},
		{Name: "newapi/jimeng-2.1-pro", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Jimeng 2.1 Pro", "zh": "即梦 2.1 专业版"}, Capability: "video"},
		{Name: "newapi/sora", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Sora", "zh": "Sora"}, Capability: "video"},
		{Name: "newapi/sora-2", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Sora 2", "zh": "Sora 2"}, Capability: "video"},
		{Name: "newapi/sora-2-pro", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Sora 2 Pro", "zh": "Sora 2 Pro"}, Capability: "video"},
		{Name: "newapi/doubao-seedance-2-0-260128", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Seedance 2.0", "zh": "Seedance 2.0"}, Capability: "video"},
		{Name: "newapi/doubao-seedance-2-0-fast-260128", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Seedance 2.0 Fast", "zh": "Seedance 2.0 快速版"}, Capability: "video"},
		{Name: "newapi/wan2.5-i2v-preview", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Wan 2.5 I2V Preview", "zh": "万相 2.5 图生视频 Preview"}, Capability: "video"},
		{Name: "newapi/wan2.2-i2v-flash", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Wan 2.2 I2V Flash", "zh": "万相 2.2 图生视频极速版"}, Capability: "video"},
		{Name: "newapi/viduq2", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Vidu Q2", "zh": "Vidu Q2"}, Capability: "video"},
		{Name: "newapi/vidu2.0", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Vidu 2.0", "zh": "Vidu 2.0"}, Capability: "video"},
		{Name: "newapi/MiniMax-Hailuo-2.3", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Hailuo 2.3", "zh": "海螺 2.3"}, Capability: "video"},
		{Name: "newapi/veo-3.0-generate-001", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Veo 3.0", "zh": "Veo 3.0"}, Capability: "video"},

		// ── Vision Understanding ──
		{Name: "newapi/qwen3.6-plus", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen 3.6 Plus", "zh": "通义千问 3.6 Plus"}, Capability: "video_understanding"},
		{Name: "newapi/qwen-vl-max", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen-VL Max", "zh": "通义千问 VL Max"}, Capability: "video_understanding"},
		{Name: "newapi/qwen-vl-plus", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen-VL Plus", "zh": "通义千问 VL Plus"}, Capability: "video_understanding"},
		{Name: "newapi/qwen-vl-max-latest", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen-VL Max Latest", "zh": "通义千问 VL Max 最新版"}, Capability: "video_understanding"},
		{Name: "newapi/qwen-vl-plus-latest", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Qwen-VL Plus Latest", "zh": "通义千问 VL Plus 最新版"}, Capability: "video_understanding"},
		{Name: "newapi/glm-4v", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GLM-4V", "zh": "智谱 GLM-4V"}, Capability: "video_understanding"},
		{Name: "newapi/glm-4v-plus", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GLM-4V Plus", "zh": "智谱 GLM-4V Plus"}, Capability: "video_understanding"},
		{Name: "newapi/glm-4.6v", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GLM-4.6V", "zh": "智谱 GLM-4.6V"}, Capability: "video_understanding"},
		{Name: "newapi/yi-vision", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Yi Vision", "zh": "零一万物 Yi Vision"}, Capability: "video_understanding"},
		{Name: "newapi/yi-vl-plus", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Yi VL Plus", "zh": "零一万物 Yi VL Plus"}, Capability: "video_understanding"},
		{Name: "newapi/grok-2-vision-1212", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Grok 2 Vision", "zh": "Grok 2 Vision"}, Capability: "video_understanding"},
		{Name: "newapi/grok-2-vision", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Grok 2 Vision", "zh": "Grok 2 Vision"}, Capability: "video_understanding"},
		{Name: "newapi/grok-vision-beta", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Grok Vision Beta", "zh": "Grok Vision Beta"}, Capability: "video_understanding"},
		{Name: "newapi/Doubao-vision-lite-32k", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Doubao Vision Lite", "zh": "豆包视觉理解 Lite"}, Capability: "video_understanding"},
		{Name: "newapi/Doubao-vision-pro-32k", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Doubao Vision Pro", "zh": "豆包视觉理解 Pro"}, Capability: "video_understanding"},
		{Name: "newapi/Doubao-1.5-pro-vision-32k", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Doubao 1.5 Pro Vision", "zh": "豆包 1.5 Pro 视觉理解"}, Capability: "video_understanding"},
		{Name: "newapi/step-1v-8k", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Step 1V", "zh": "阶跃星辰 Step 1V"}, Capability: "video_understanding"},
		{Name: "newapi/step-1.5v-mini", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Step 1.5V Mini", "zh": "阶跃星辰 Step 1.5V Mini"}, Capability: "video_understanding"},
		{Name: "newapi/gpt-4-vision-preview", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GPT-4 Vision", "zh": "GPT-4 Vision"}, Capability: "video_understanding"},
		{Name: "newapi/gpt-4-1106-vision-preview", Provider: "newapi", DisplayName: engine.DisplayName{"en": "GPT-4 Vision 1106", "zh": "GPT-4 Vision 1106"}, Capability: "video_understanding"},

		// ── TTS ──
		{Name: "newapi/tts-1", Provider: "newapi", DisplayName: engine.DisplayName{"en": "TTS-1", "zh": "TTS-1"}, Capability: "tts"},
		{Name: "newapi/tts-1-hd", Provider: "newapi", DisplayName: engine.DisplayName{"en": "TTS-1 HD", "zh": "TTS-1 HD"}, Capability: "tts"},

		// ── ASR ──
		{Name: "newapi/whisper-1", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Whisper 1", "zh": "Whisper 1"}, Capability: "asr"},
		{Name: "newapi/whisper-large-v3", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Whisper Large V3", "zh": "Whisper Large V3"}, Capability: "asr"},

		// ── Music ──
		{Name: "newapi/suno_music", Provider: "newapi", DisplayName: engine.DisplayName{"en": "Suno Music", "zh": "Suno 音乐生成"}, Capability: "music"},
	}
}
