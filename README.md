# aigo

[中文说明](./README.zh-CN.md)

`aigo` is an agent-native Go SDK for multimodal media generation. Describe work as a lightweight workflow graph, route it to 30+ execution engines, and get structured results with error classification, retry hints, and progress callbacks.

## Architecture

```
Agent (LLM / code)
  │
  ▼
AgentTask ──► BuildGraph() ──► workflow.Graph (DAG)
                                    │
              ┌─────────┬──────────┼──────────┬──────────┐
              ▼          ▼          ▼          ▼          ▼
        engine/kling  engine/luma  engine/fal  ...   engine/comfyui
              │          │          │                     │
              ▼          ▼          ▼                     ▼
         Kling API   Luma API   Fal API             ComfyUI WS
```

## Engines

### Image Generation

| Engine | Backend | Env Var |
|--------|---------|---------|
| `aliyun` | Alibaba Cloud DashScope (Qwen, Wan, Z-Image) | `DASHSCOPE_API_KEY` |
| `openai` | OpenAI DALL-E 3 | `OPENAI_API_KEY` |
| `google` | Google Imagen | `GOOGLE_API_KEY` |
| `flux` | Black Forest Labs FLUX | `BFL_API_KEY` |
| `stability` | Stability AI (SD3, Ultra, Core) | `STABILITY_API_KEY` |
| `ideogram` | Ideogram | `IDEOGRAM_API_KEY` |
| `recraft` | Recraft V3 | `RECRAFT_API_KEY` |
| `midjourney` | Midjourney (via GoAPI) | `GOAPI_KEY` |
| `jimeng` | Jimeng (Volcengine) | `JIMENG_API_KEY` |
| `liblib` | LibLibAI (HMAC-SHA1 auth) | `LIBLIB_ACCESS_KEY` / `LIBLIB_SECRET_KEY` |
| `ark` | Volcengine Ark | `ARK_API_KEY` |

### Video Generation

| Engine | Backend | Env Var |
|--------|---------|---------|
| `kling` | Kling AI (v1/v1.5/v2/v2.1) | `KLING_API_KEY` |
| `hailuo` | Hailuo / MiniMax Video | `HAILUO_API_KEY` |
| `luma` | Luma Dream Machine | `LUMA_API_KEY` |
| `runway` | Runway Gen-3/Gen-4 | `RUNWAY_API_KEY` |
| `pika` | Pika Labs | `PIKA_API_KEY` |
| `hedra` | Hedra (talking head video) | `HEDRA_API_KEY` |

### Audio / Music

| Engine | Backend | Env Var |
|--------|---------|---------|
| `elevenlabs` | ElevenLabs TTS | `ELEVENLABS_API_KEY` |
| `minimax` | MiniMax TTS & Music | `MINIMAX_API_KEY` |
| `suno` | Suno Music Generation | `SUNO_API_KEY` |
| `volcvoice` | Volcengine Speech | `VOLC_SPEECH_ACCESS_TOKEN` |

### 3D Generation

| Engine | Backend | Env Var |
|--------|---------|---------|
| `meshy` | Meshy (text/image to 3D) | `MESHY_API_KEY` |

### Multi-Modal Understanding

| Engine | Backend | Env Var |
|--------|---------|---------|
| `gemini` | Google Gemini (vision + text) | `GEMINI_API_KEY` |
| `gpt4o` | OpenAI GPT-4o Vision | `OPENAI_API_KEY` |

### Multi-Backend / Gateway

| Engine | Backend | Env Var |
|--------|---------|---------|
| `newapi` | Multi-route gateway (OpenAI, Kling, Jimeng, Sora, Qwen, Gemini) | `NEWAPI_API_KEY` |
| `openrouter` | OpenRouter (multi-provider routing) | `OPENROUTER_API_KEY` |
| `fal` | Fal.ai (generic model runner) | `FAL_KEY` |
| `replicate` | Replicate (generic model runner) | `REPLICATE_API_TOKEN` |
| `comfydeploy` | ComfyDeploy (hosted ComfyUI) | `COMFYDEPLOY_API_TOKEN` |
| `comfyui` | ComfyUI server (WebSocket) | `COMFY_CLOUD_API_KEY` |
| `runninghub` | RunningHub (ComfyUI cloud) | `RH_API_KEY` |

### Embedding

| Engine | Backend | Env Var |
|--------|---------|---------|
| `embed/openai` | OpenAI Embeddings | `OPENAI_API_KEY` |
| `embed/gemini` | Google Gemini Embeddings | `GEMINI_API_KEY` |
| `embed/aliyun` | DashScope Embeddings (text + multimodal) | `DASHSCOPE_API_KEY` |
| `embed/jina` | Jina Embeddings | `JINA_API_KEY` |
| `embed/voyage` | Voyage AI Embeddings | `VOYAGE_API_KEY` |

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

tools := tooldef.AllTools()
// generate_image, generate_video, generate_3d, text_to_speech,
// design_voice, edit_image, edit_video, transcribe_audio, generate_music
```

### Engine registry

Centralized engine registration, lookup, and capability-based discovery:

```go
import "github.com/godeps/aigo/engine"

reg := engine.NewRegistry()
reg.Register("kling", engine.Entry{
    Name:   "kling",
    Engine: klingEngine,
    ConfigSchemaFunc:   kling.ConfigSchema,
    ModelsByCapability: kling.ModelsByCapability,
})

// Find all engines that can generate video
videoEngines := reg.FindByCapability("video")

// Get all models grouped by engine and capability
allModels := reg.AllModels()
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

### Result caching

Cache results to avoid redundant API calls:

```go
import "github.com/godeps/aigo/engine"

cached := engine.WithCache(myEngine, 10*time.Minute, 100) // TTL + max entries
// Identical workflow graphs return cached results
```

### HTTP retry & rate limiting

Built-in HTTP transports for resilient API calls:

```go
import "github.com/godeps/aigo/engine/httpx"

// Auto-retry on 429/5xx with exponential backoff
retryClient := httpx.NewRetryClient(httpx.RetryOptions{
    MaxRetries: 3,
    BaseDelay:  time.Second,
})

// Token bucket rate limiting
rateLimitedClient := httpx.NewRateLimitedClient(10, 20, 30*time.Second) // 10 RPS, burst 20
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

## Internal Packages

| Package | Purpose |
|---------|---------|
| `workflow` | Workflow graph types and validation |
| `workflow/resolve` | Graph resolution (prompt extraction, option helpers, link following) |
| `engine/poll` | Unified polling with backoff, progress callbacks, and status mapping |
| `engine/httpx` | HTTP client defaults, retry transport, rate limiting, file upload |
| `engine/aigoerr` | Structured error classification for agent retry logic |
| `engine/embed` | Embedding engine implementations (OpenAI, Gemini, Jina, Voyage, Aliyun) |
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
- All async engines support `WaitForCompletion` mode for synchronous use and `Resume()` for reconnecting to running tasks.
- All engines use unified API key resolution via `engine.ResolveKey` — configure via struct field or environment variable.
