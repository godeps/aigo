// Package newapi 对接 New API 文档中的大模型 HTTP 接口（图像 / 视频 / 语音等）。
//
// BaseURL 可为网关 origin（https://host）或常见写法 https://host/v1；内部会规范为 origin，
// 再拼接绝对路径，以便同时支持 /v1/...、/kling/...、/jimeng/... 等。
//
// 通过 Config.Route 选择路径族（可灵、即梦、Sora、通义千问图、Gemini 等）；未指定时按 MediaKind 使用 OpenAI 兼容默认路由。
// Config.DisableRemoteMediaFetch 为 true 时，图中 image_url/audio_url 不会发起 HTTP GET（降低 SSRF 风险）。
//
// 文档片段见仓库：other/new-api-docs-v1/openapi/generated/ai-model/ 下各子目录。
package newapi
