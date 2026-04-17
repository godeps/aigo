package engine

// EngineDisplayNames maps engine package names to their official localized display names.
// Use LookupDisplayName to query this registry.
var EngineDisplayNames = map[string]DisplayName{
	// Image Generation
	"alibabacloud": {EN: "Alibaba Cloud DashScope", ZH: "阿里云百炼"},
	"openai":       {EN: "OpenAI DALL-E", ZH: "OpenAI DALL-E"},
	"google":       {EN: "Google Imagen", ZH: "Google Imagen"},
	"flux":         {EN: "FLUX by Black Forest Labs", ZH: "FLUX"},
	"stability":    {EN: "Stability AI", ZH: "Stability AI"},
	"ideogram":     {EN: "Ideogram", ZH: "Ideogram"},
	"recraft":      {EN: "Recraft", ZH: "Recraft"},
	"midjourney":   {EN: "Midjourney", ZH: "Midjourney"},
	"jimeng":       {EN: "Jimeng", ZH: "即梦"},
	"liblib":       {EN: "LibLibAI", ZH: "哩布哩布AI"},
	"ark":          {EN: "Volcengine Ark", ZH: "火山引擎方舟"},

	// Video Generation
	"kling":  {EN: "Kling AI", ZH: "可灵 AI"},
	"hailuo": {EN: "Hailuo Video", ZH: "海螺视频"},
	"luma":   {EN: "Luma Dream Machine", ZH: "Luma Dream Machine"},
	"runway": {EN: "Runway", ZH: "Runway"},
	"pika":   {EN: "Pika Labs", ZH: "Pika Labs"},
	"hedra":  {EN: "Hedra", ZH: "Hedra"},

	// Audio / Music
	"elevenlabs": {EN: "ElevenLabs", ZH: "ElevenLabs"},
	"minimax":    {EN: "MiniMax", ZH: "MiniMax"},
	"suno":       {EN: "Suno", ZH: "Suno"},
	"volcvoice":  {EN: "Volcengine Speech", ZH: "火山引擎语音"},

	// 3D Generation
	"meshy": {EN: "Meshy", ZH: "Meshy"},

	// Multi-Modal Understanding
	"gemini": {EN: "Google Gemini", ZH: "Google Gemini"},
	"gpt4o":  {EN: "OpenAI GPT-4o", ZH: "OpenAI GPT-4o"},

	// Multi-Backend / Gateway
	"newapi":      {EN: "NewAPI Gateway", ZH: "NewAPI 网关"},
	"openrouter":  {EN: "OpenRouter", ZH: "OpenRouter"},
	"fal":         {EN: "Fal.ai", ZH: "Fal.ai"},
	"replicate":   {EN: "Replicate", ZH: "Replicate"},
	"comfydeploy": {EN: "ComfyDeploy", ZH: "ComfyDeploy"},
	"comfyui":     {EN: "ComfyUI", ZH: "ComfyUI"},
	"runninghub":  {EN: "RunningHub", ZH: "RunningHub"},

	// Embedding
	"embed/alibabacloud": {EN: "DashScope Embeddings", ZH: "百炼向量嵌入"},
	"embed/openai":       {EN: "OpenAI Embeddings", ZH: "OpenAI Embeddings"},
	"embed/gemini":       {EN: "Gemini Embeddings", ZH: "Gemini Embeddings"},
	"embed/jina":         {EN: "Jina Embeddings", ZH: "Jina Embeddings"},
	"embed/voyage":       {EN: "Voyage AI Embeddings", ZH: "Voyage AI Embeddings"},
}

// LookupDisplayName returns the display name for the given engine key.
// If the key is not found, it returns a DisplayName with the key itself as both EN and ZH.
func LookupDisplayName(key string) DisplayName {
	if dn, ok := EngineDisplayNames[key]; ok {
		return dn
	}
	return DisplayName{EN: key, ZH: key}
}
