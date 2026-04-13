# aigo

[中文说明](./README.zh-CN.md)

`aigo` is an agent-native Go SDK for multimodal media generation. Describe work as a lightweight workflow graph, route it to different execution engines, and get structured results with error classification, retry hints, and progress callbacks. Zero external dependencies — only Go stdlib.

## Architecture

```
Agent (LLM / code)
  │
  ▼
AgentTask ──► BuildGraph() ──► workflow.Graph (DAG)
                                    │
                     ┌──────────────┼──────────────┐──────────────┐
                     ▼              ▼              ▼              ▼
               engine/aliyun  engine/openai  engine/newapi  engine/comfyui
                     │              │              │              │
                     ▼              ▼              ▼              ▼
               Bailian API    DALL-E API    Multi-gateway    ComfyUI WS
```

## Engines

| Engine | Backend | Capabilities |
|--------|---------|-------------|
| `aliyun` | Alibaba Cloud Bailian / DashScope | Image, video, TTS, voice design |
| `openai` | OpenAI DALL-E | Image generation |
| `newapi` | Multi-route gateway | OpenAI-compat, Kling, Jimeng, Sora, Qwen, Gemini |
| `comfyui` | ComfyUI server | Full passthrough via WebSocket |

## Install

```bash
go get github.com/godeps/aigo
```

## Quick Start

### Simple prompt

```go
client := aigo.NewClient()

_ = client.RegisterEngine("img", aliyun.New(aliyun.Config{
    Model: aliyun.ModelQwenImage,
}))

result, err := client.ExecutePrompt(ctx, "img", "A shiba inu riding a vintage motorcycle")
fmt.Println(result.Value)   // URL or data URI
fmt.Println(result.Kind)    // aigo.OutputURL, OutputDataURI, etc.
fmt.Println(result.Engine)  // "img"
fmt.Println(result.Elapsed) // execution duration
```

### Rich Result type

Every execution method returns `aigo.Result`:

```go
type Result struct {
    Value    string         // raw output (URL, data URI, JSON, etc.)
    Kind     OutputKind     // authoritative classification
    Engine   string         // which engine produced this
    Elapsed  time.Duration  // wall-clock execution time
    Metadata map[string]any // engine-specific data (optional)
}

fmt.Println(result) // Result implements String(), prints Value
```

### Structured task

```go
result, err := client.ExecuteTask(ctx, "video", aigo.AgentTask{
    Prompt:   "Turn this product scene into a short ad",
    Duration: 2,
    Structured: &aigo.AgentTaskStructured{
        VideoSize: "1280*720",
        ImageSize: "1024*1024",
    },
    References: []aigo.ReferenceAsset{
        {Type: aigo.ReferenceTypeVideo, URL: "https://example.com/input.mp4"},
        {Type: aigo.ReferenceTypeImage, URL: "https://example.com/style.png"},
    },
})
```

### TTS (text-to-speech)

```go
result, err := client.ExecuteTask(ctx, "tts", aigo.AgentTask{
    Prompt: "Welcome to our product launch event",
    TTS: &aigo.TTSOptions{
        Voice:        "zhiyan",
        LanguageType: "zh",
    },
})
```

### Voice design

```go
result, err := client.ExecuteTask(ctx, "vd", aigo.AgentTask{
    Prompt: "design a voice",
    VoiceDesign: &aigo.VoiceDesignOptions{
        VoicePrompt:   "A warm, friendly female voice",
        PreviewText:   "Hello, welcome!",
        TargetModel:   "cosyvoice-v2",
        PreferredName: "custom-voice-01",
    },
})
```

## Agent-Native Features

### Structured errors with retry classification

Errors from all engines are classified so agents can make retry decisions:

```go
import "github.com/godeps/aigo/engine/aigoerr"

_, err := client.ExecutePrompt(ctx, "img", "...")
if aigoerr.IsRetryable(err) {
    // safe to retry (429, 5xx, timeout)
}

code, ok := aigoerr.GetCode(err)
// aigoerr.CodeRateLimit, CodeServerError, CodeInvalidInput, etc.

var ae *aigoerr.Error
if errors.As(err, &ae) {
    fmt.Println(ae.StatusCode)  // original HTTP status
    fmt.Println(ae.RetryAfter)  // parsed Retry-After header
}
```

### JSON Schema tool definitions

Register aigo tools with any agent framework (OpenAI, Anthropic, LangChain, Vercel AI SDK):

```go
import "github.com/godeps/aigo/tooldef"

tools := tooldef.AllTools() // generate_image, generate_video, text_to_speech, ...
// Register with your framework's function-calling system
```

### Engine capability discovery

Query what engines can do — for dynamic tool selection:

```go
cap, _ := client.EngineCapabilities("aliyun-img")
// cap.MediaTypes  → ["image"]
// cap.Models      → ["qwen-image"]
// cap.SupportsSync, cap.SupportsPoll

// Find all engines that handle video:
videoEngines := client.AvailableFor("video")
```

### Progress reporting

Monitor long-running tasks:

```go
result, err := client.Execute(ctx, "video", graph, aigo.WithProgress(func(e aigo.ProgressEvent) {
    fmt.Printf("[%s] elapsed=%s\n", e.Phase, e.Elapsed)
    // Phase: "submitted", "completed"
}))
```

### Middleware

Add cross-cutting concerns (logging, retry, timing):

```go
client.Use(aigo.WithLogging(os.Stderr))
client.Use(aigo.WithRetry(3)) // retry retryable errors up to 3 times
```

### Pipeline chaining

Chain multi-step workflows where each step feeds the next:

```go
p := aigo.NewPipeline("img", aigo.AgentTask{Prompt: "a cat"}).
    Then(func(prev aigo.Result) (aigo.AgentTask, string) {
        return aigo.AgentTask{
            Prompt:     "animate this image",
            References: []aigo.ReferenceAsset{{Type: aigo.ReferenceTypeImage, URL: prev.Value}},
        }, "video"
    })

results, err := client.ExecutePipeline(ctx, p)
// results[0] = image result, results[1] = video result
```

### DryRun estimation

Check what would happen without executing:

```go
dr, err := client.DryRun("video", aigo.AgentTask{Prompt: "..."})
// dr.WillPoll       — whether the engine will poll
// dr.EstimatedTime  — human-readable time estimate
// dr.Warnings       — potential issues
```

### Auto-routing with selector

Let the LLM inside your agent choose the engine:

```go
result, err := client.ExecuteTaskAuto(ctx, selector, aigo.AgentTask{
    Prompt:   "make a 2 second product video",
    Duration: 2,
})
// result.Engine       — which engine was selected
// result.Reason       — why it was selected
// result.Output.Value — the generation result
```

### Fallback across engines

Try multiple engines in order; first success wins:

```go
result, err := client.ExecuteWithFallback(ctx, []string{"primary", "backup"}, graph)
// result.Engine       — which engine succeeded
// result.Output.Value — the result
// result.Skipped      — engines that failed (with errors)
```

### Async execution

Non-blocking execution via channel:

```go
ch := client.ExecuteAsync(ctx, "video", graph)
// ... do other work ...
ar := <-ch
if ar.Err != nil { ... }
fmt.Println(ar.Result.Value)
```

## Low-Level API

If your agent already emits workflow graphs, call `Execute` directly:

```go
graph := workflow.Graph{
    "1": {
        ClassType: "CLIPTextEncode",
        Inputs:    map[string]any{"text": "A cinematic lighthouse in a storm"},
    },
    "2": {
        ClassType: "EmptyLatentImage",
        Inputs:    map[string]any{"width": 1536, "height": 1024},
    },
}

result, err := client.Execute(ctx, "img", graph)
```

## Alibaba Cloud Models

`engine/aliyun` supports:

| Constant | Model | Type |
|----------|-------|------|
| `ModelQwenImage` | qwen-image | Image |
| `ModelWanImage` | wan2.7-image | Image |
| `ModelZImageTurbo` | z-image-turbo | Image |
| `ModelWanTextToVideo` | wan2.7-t2v | Video |
| `ModelWanImageToVideo` | wan2.7-i2v | Video |
| `ModelWanReferenceVideo` | wan2.7-r2v | Video |
| `ModelWanVideoEdit` | wan2.7-videoedit | Video |
| `ModelKlingV3Video` | kling/kling-v3-video-generation | Video |
| `ModelKlingV3OmniVideo` | kling/kling-v3-omni-video-generation | Video |
| `ModelQwenTTSFlash` | qwen3-tts-flash | TTS |
| `ModelQwenTTSInstructFlash` | qwen3-tts-instruct-flash | TTS |
| `ModelQwenVoiceDesign` | qwen-voice-design | Voice Design |

Environment variable:

```bash
export DASHSCOPE_API_KEY=your_key
```

## New API Gateway

`engine/newapi` supports multiple route families via a single gateway:

| Route | Protocol |
|-------|----------|
| `RouteOpenAIImagesGenerations` | POST /v1/images/generations |
| `RouteOpenAIImagesEdits` | POST /v1/images/edits |
| `RouteOpenAIVideoGenerations` | POST+GET /v1/video/generations |
| `RouteOpenAISpeech` | POST /v1/audio/speech |
| `RouteKlingText2Video` | Kling text-to-video |
| `RouteKlingImage2Video` | Kling image-to-video |
| `RouteJimengVideo` | Jimeng (Volcengine) video |
| `RouteSoraVideos` | OpenAI Sora video |
| `RouteQwenImagesGenerations` | Qwen image generation |
| `RouteGeminiGenerateContent` | Gemini native generateContent |

Environment variables:

```bash
export NEWAPI_BASE_URL=https://your-gateway.example.com
export NEWAPI_API_KEY=your_key
```

## Internal Packages

| Package | Purpose |
|---------|---------|
| `workflow/resolve` | Shared graph resolution (node string extraction, option helpers, link following) |
| `engine/poll` | Unified polling with exponential backoff, max attempts, and progress callbacks |
| `engine/httpx` | HTTP client defaults and helpers |
| `engine/aigoerr` | Structured error classification for agent retry logic |
| `tooldef` | JSON Schema tool definitions for agent frameworks |

## Examples

```bash
# Alibaba Cloud
go run ./examples/aliyun_qwen_image
go run ./examples/aliyun_wan_image
go run ./examples/aliyun_zimage
go run ./examples/aliyun_wan_t2v
go run ./examples/aliyun_wan_r2v
go run ./examples/aliyun_wan_videoedit
go run ./examples/aliyun_qwen_tts
go run ./examples/aliyun_qwen_voice_design

# New API gateway
go run ./examples/newapi_image
go run ./examples/newapi_speech
go run ./examples/newapi_video

# Auto-routing
go run ./examples/agent_auto_router
```

## Notes

- Alibaba Cloud result URLs are temporary OSS links. Persist them immediately.
- As of April 2026, wan2.7 series (`wan2.7-t2v`, `wan2.7-i2v`, `wan2.7-r2v`, `wan2.7-image`, `wan2.7-videoedit`) are the current video/image models.
