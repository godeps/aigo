# aigo

[English README](./README.md)

`aigo` 是一个面向 Agent 的 Go SDK，用于多模态媒体生成。Agent 输出轻量工作流图，SDK 把图路由到 30+ 执行引擎，并返回结构化结果（含错误分类、重试提示和进度回调）。

## 架构

```
Agent (LLM / 代码)
  │
  ▼
AgentTask ──► BuildGraph() ──► workflow.Graph (DAG)
                                    │
              ┌─────────┬──────────┼──────────┬──────────┐
              ▼          ▼          ▼          ▼          ▼
        engine/kling  engine/luma  engine/fal  ...   engine/comfyui
              │          │          │                     │
              ▼          ▼          ▼                     ▼
          可灵 API    Luma API   Fal API            ComfyUI WS
```

## 引擎

### 图片生成

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `alibabacloud` | 阿里云百炼 DashScope（通义万相、Wan、Z-Image） | `DASHSCOPE_API_KEY` |
| `openai` | OpenAI DALL-E 3 | `OPENAI_API_KEY` |
| `google` | Google Imagen | `GOOGLE_API_KEY` |
| `flux` | Black Forest Labs FLUX | `BFL_API_KEY` |
| `stability` | Stability AI（SD3、Ultra、Core） | `STABILITY_API_KEY` |
| `ideogram` | Ideogram | `IDEOGRAM_API_KEY` |
| `recraft` | Recraft V3 | `RECRAFT_API_KEY` |
| `midjourney` | Midjourney（GoAPI 代理） | `GOAPI_KEY` |
| `jimeng` | 即梦（火山引擎） | `JIMENG_API_KEY` |
| `liblib` | LibLibAI（HMAC-SHA1 签名） | `LIBLIB_ACCESS_KEY` / `LIBLIB_SECRET_KEY` |
| `ark` | 火山引擎 Ark | `ARK_API_KEY` |

### 视频生成

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `kling` | 可灵 AI（v1/v1.5/v2/v2.1） | `KLING_API_KEY` |
| `hailuo` | 海螺 / MiniMax 视频 | `HAILUO_API_KEY` |
| `luma` | Luma Dream Machine | `LUMA_API_KEY` |
| `runway` | Runway Gen-3/Gen-4 | `RUNWAY_API_KEY` |
| `pika` | Pika Labs | `PIKA_API_KEY` |
| `hedra` | Hedra（数字人视频） | `HEDRA_API_KEY` |

### 音频 / 音乐

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `elevenlabs` | ElevenLabs TTS | `ELEVENLABS_API_KEY` |
| `minimax` | MiniMax TTS 与音乐 | `MINIMAX_API_KEY` |
| `suno` | Suno 音乐生成 | `SUNO_API_KEY` |
| `volcvoice` | 火山引擎语音 | `VOLC_SPEECH_ACCESS_TOKEN` |

### 3D 生成

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `meshy` | Meshy（文本/图片转 3D） | `MESHY_API_KEY` |

### 多模态理解

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `gemini` | Google Gemini（视觉+文本） | `GEMINI_API_KEY` |
| `gpt4o` | OpenAI GPT-4o 视觉理解 | `OPENAI_API_KEY` |

### 多后端 / 网关

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `newapi` | 多路网关（OpenAI、可灵、即梦、Sora、通义、Gemini） | `NEWAPI_API_KEY` |
| `openrouter` | OpenRouter（多供应商路由） | `OPENROUTER_API_KEY` |
| `fal` | Fal.ai（通用模型运行器） | `FAL_KEY` |
| `replicate` | Replicate（通用模型运行器） | `REPLICATE_API_TOKEN` |
| `comfydeploy` | ComfyDeploy（托管 ComfyUI） | `COMFYDEPLOY_API_TOKEN` |
| `comfyui` | ComfyUI 服务（WebSocket） | `COMFY_CLOUD_API_KEY` |
| `runninghub` | RunningHub（ComfyUI 云端） | `RH_API_KEY` |

### 向量嵌入

| 引擎 | 后端 | 环境变量 |
|------|------|---------|
| `embed/openai` | OpenAI Embeddings | `OPENAI_API_KEY` |
| `embed/gemini` | Google Gemini Embeddings | `GEMINI_API_KEY` |
| `embed/alibabacloud` | 百炼向量嵌入（文本+多模态） | `DASHSCOPE_API_KEY` |
| `embed/jina` | Jina Embeddings | `JINA_API_KEY` |
| `embed/voyage` | Voyage AI Embeddings | `VOYAGE_API_KEY` |

## 安装

```bash
go get github.com/godeps/aigo
```

## 快速开始

### 简单提示词

```go
client := aigo.NewClient()

_ = client.RegisterEngine("img", alibabacloud.New(alibabacloud.Config{
    Model: alibabacloud.ModelQwenImage,
}))

result, err := client.ExecutePrompt(ctx, "img", "一只骑复古摩托的柴犬，电影感")
fmt.Println(result.Value)   // URL 或 data URI
fmt.Println(result.Kind)    // aigo.OutputURL, OutputDataURI 等
fmt.Println(result.Engine)  // "img"
fmt.Println(result.Elapsed) // 执行耗时
```

### 富 Result 类型

所有执行方法统一返回 `aigo.Result`：

```go
type Result struct {
    Value    string         // 原始输出（URL、data URI、JSON 等）
    Kind     OutputKind     // 引擎权威分类
    Engine   string         // 产出引擎名
    Elapsed  time.Duration  // 执行耗时
    Metadata map[string]any // 引擎特有数据（可选）
}

fmt.Println(result) // Result 实现了 String()，打印 Value
```

### 结构化任务

```go
result, err := client.ExecuteTask(ctx, "video", aigo.AgentTask{
    Prompt:   "把这个产品场景生成成一条 2 秒广告视频",
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

### TTS（文字转语音）

```go
result, err := client.ExecuteTask(ctx, "tts", aigo.AgentTask{
    Prompt: "欢迎来到我们的产品发布会",
    TTS: &aigo.TTSOptions{
        Voice:        "zhiyan",
        LanguageType: "zh",
    },
})
```

### 声音设计

```go
result, err := client.ExecuteTask(ctx, "vd", aigo.AgentTask{
    Prompt: "设计一个声音",
    VoiceDesign: &aigo.VoiceDesignOptions{
        VoicePrompt:   "温暖友好的女性声音",
        PreviewText:   "你好，欢迎！",
        TargetModel:   "cosyvoice-v2",
        PreferredName: "custom-voice-01",
    },
})
```

## 批量注册

### 一次注册多个引擎

```go
client.RegisterAll(map[string]engine.Engine{
    "img":   alibabacloud.New(alibabacloud.Config{Model: alibabacloud.ModelQwenImage}),
    "video": kling.New(kling.Config{Model: kling.ModelKlingV2Master}),
})
```

### 条件注册（环境变量缺失时跳过）

```go
client.RegisterAllIfKey([]aigo.EngineEntry{
    {Name: "kling-video", Engine: klingEngine, EnvVars: []string{"KLING_API_KEY"}},
    {Name: "luma-video",  Engine: lumaEngine,  EnvVars: []string{"LUMA_API_KEY"}},
    {Name: "local",       Engine: localEngine}, // 始终注册
})
```

### Provider 分组

一次调用注册某个供应商的所有引擎。缺少所需环境变量的引擎会被自动跳过：

```go
import "github.com/godeps/aigo/engine/alibabacloud"

registered, _ := client.RegisterProvider(alibabacloud.DefaultProvider())
// registered: ["alibabacloud-image", "alibabacloud-video", "alibabacloud-tts"]
```

每个引擎包都导出 `DefaultProvider()`，包含合理的默认预设。

### 配置文件驱动

通过 JSON 文件声明引擎，无需在代码中硬编码：

```json
{
  "engines": [
    {"name": "img",   "provider": "alibabacloud", "model": "qwen-image"},
    {"name": "video", "provider": "kling",         "model": "kling-v2-master"},
    {"name": "tts",   "provider": "elevenlabs"},
    {"name": "backup","provider": "runway",         "enabled": false}
  ]
}
```

```go
cfg, _ := aigo.LoadConfig("engines.json")
registered, _ := client.ApplyConfig(cfg)
```

每个引擎包通过 `init()` 注册工厂函数，只需 import 即可使用：

```go
import (
    _ "github.com/godeps/aigo/engine/alibabacloud"
    _ "github.com/godeps/aigo/engine/kling"
    _ "github.com/godeps/aigo/engine/elevenlabs"
    _ "github.com/godeps/aigo/engine/runway"
)
```

## 模型 i18n 元数据

每个通过引擎包注册的模型都携带中英文显示名称和功能描述：

```go
import _ "github.com/godeps/aigo/engine/kling"

info, ok := client.ModelInfo("kling-v2-master")
fmt.Println(info.DisplayName["en"]) // "Kling V2 Master"
fmt.Println(info.DisplayName["zh"]) // "可灵 V2 大师版"
fmt.Println(info.Description["zh"]) // "最高画质视频和图片生成"
fmt.Println(info.Intro["zh"])       // "可灵旗舰视频生成模型，支持文生视频和图生视频..."
fmt.Println(info.DocURL)            // "https://docs.qingque.cn/..."
fmt.Println(info.Capability)        // "video"
```

列出所有已注册模型：

```go
for _, m := range client.AllModelInfos() {
    fmt.Printf("%-25s %-8s %s\n", m.Name, m.Capability, m.DisplayName["zh"])
}
```

也可以直接使用 `engine.LookupModelInfo(name)` 和 `engine.AllModelInfos()`。

### 按能力过滤模型

```go
videoModels := client.ModelInfosByCapability("video")
for _, m := range videoModels {
    fmt.Printf("%s — %s\n", m.Name, m.DisplayName["zh"])
}
```

### 按引擎查询模型

```go
klingModels := client.ModelInfosByProvider("kling")
// 返回 kling 引擎包注册的所有 ModelInfo
```

### 引擎级别元数据

```go
meta := engine.LookupEngineMetadata("kling")
fmt.Println(meta.DisplayName["zh"]) // "可灵 AI"
fmt.Println(meta.Intro["zh"])       // 详细介绍
fmt.Println(meta.DocURL)            // 官方文档地址
```

## Agent 原生特性

### 结构化错误与重试分类

所有引擎的错误都经过分类，Agent 可据此决定重试策略：

```go
import "github.com/godeps/aigo/engine/aigoerr"

_, err := client.ExecutePrompt(ctx, "img", "...")
if aigoerr.IsRetryable(err) {
    // 可安全重试（429、5xx、超时）
}

code, ok := aigoerr.GetCode(err)
// aigoerr.CodeRateLimit, CodeServerError, CodeInvalidInput 等

var ae *aigoerr.Error
if errors.As(err, &ae) {
    fmt.Println(ae.StatusCode)  // 原始 HTTP 状态码
    fmt.Println(ae.RetryAfter)  // 解析后的 Retry-After
}
```

### JSON Schema 工具定义

将 aigo 工具注册到任意 Agent 框架（OpenAI、Anthropic、LangChain、Vercel AI SDK）：

```go
import "github.com/godeps/aigo/tooldef"

tools := tooldef.AllTools()
// generate_image, generate_video, generate_3d, text_to_speech,
// design_voice, edit_image, edit_video, transcribe_audio, generate_music
```

### 引擎注册表

集中式引擎注册、查找和基于能力的发现：

```go
import "github.com/godeps/aigo/engine"

reg := engine.NewRegistry()
reg.Register("kling", engine.Entry{
    Name:   "kling",
    Engine: klingEngine,
    ConfigSchemaFunc:   kling.ConfigSchema,
    ModelsByCapability: kling.ModelsByCapability,
})

// 查找所有能生成视频的引擎
videoEngines := reg.FindByCapability("video")

// 获取所有模型，按引擎和能力分组
allModels := reg.AllModels()
```

### 引擎能力发现

查询引擎的能力 — 用于动态工具选择：

```go
cap, _ := client.EngineCapabilities("alibabacloud-img")
// cap.MediaTypes  → ["image"]
// cap.Models      → ["qwen-image"]
// cap.SupportsSync, cap.SupportsPoll

// 查找所有支持视频的引擎：
videoEngines := client.AvailableFor("video")
```

### 引擎控制

动态启用、禁用或条件注册引擎：

```go
// 禁用引擎（不移除）
client.DisableEngine("runway")

// 稍后重新启用
client.EnableEngine("runway")

// 完全移除引擎
client.UnregisterEngine("old-engine")

// 仅在 API Key 存在时注册
client.RegisterEngineIfKey("kling", klingEngine, "KLING_API_KEY")

// 检查引擎是否启用
if client.IsEnabled("kling") { ... }
```

### 工具定义过滤

按媒体类型筛选工具定义，注册到 Agent 框架：

```go
import "github.com/godeps/aigo/tooldef"

// 所有工具
tools := tooldef.AllTools() // 9 个工具

// 仅图片工具（generate_image, edit_image）
imageTools := tooldef.ToolsFor("image")

// 多个类别
mediaTools := tooldef.ToolsFor("video", "audio", "music")
```

### 进度上报

监控长时间运行的任务：

```go
result, err := client.Execute(ctx, "video", graph, aigo.WithProgress(func(e aigo.ProgressEvent) {
    fmt.Printf("[%s] elapsed=%s\n", e.Phase, e.Elapsed)
    // Phase: "submitted", "completed"
}))
```

### 结果缓存

缓存结果以避免重复 API 调用：

```go
import "github.com/godeps/aigo/engine"

cached := engine.WithCache(myEngine, 10*time.Minute, 100) // TTL + 最大条目
// 相同的工作流图返回缓存结果
```

### HTTP 重试与限流

内置 HTTP 传输层，实现弹性 API 调用：

```go
import "github.com/godeps/aigo/engine/httpx"

// 429/5xx 自动重试，指数退避
retryClient := httpx.NewRetryClient(httpx.RetryOptions{
    MaxRetries: 3,
    BaseDelay:  time.Second,
})

// 令牌桶限流
rateLimitedClient := httpx.NewRateLimitedClient(10, 20, 30*time.Second) // 10 RPS，突发 20
```

### 中间件

添加横切关注点（日志、重试、计时）：

```go
client.Use(aigo.WithLogging(os.Stderr))
client.Use(aigo.WithRetry(3)) // 可重试错误最多重试 3 次
```

### Pipeline 链式组合

链式执行多步工作流，每一步的输入依赖上一步的输出：

```go
p := aigo.NewPipeline("img", aigo.AgentTask{Prompt: "一只猫"}).
    Then(func(prev aigo.Result) (aigo.AgentTask, string) {
        return aigo.AgentTask{
            Prompt:     "让这张图动起来",
            References: []aigo.ReferenceAsset{{Type: aigo.ReferenceTypeImage, URL: prev.Value}},
        }, "video"
    })

results, err := client.ExecutePipeline(ctx, p)
// results[0] = 图片结果, results[1] = 视频结果
```

### DryRun 预估

不执行，仅检查会发生什么：

```go
dr, err := client.DryRun("video", aigo.AgentTask{Prompt: "..."})
// dr.WillPoll       — 是否需要轮询
// dr.EstimatedTime  — 可读的时间估计
// dr.Warnings       — 潜在问题
```

### 自动路由（Selector）

让 Agent 内部的 LLM 自行选择引擎：

```go
result, err := client.ExecuteTaskAuto(ctx, selector, aigo.AgentTask{
    Prompt:   "生成一条 2 秒产品广告视频",
    Duration: 2,
})
// result.Engine       — 被选中的引擎
// result.Reason       — 选择原因
// result.Output.Value — 生成结果
```

### 能力感知路由（RichSelector）

`RichSelector` 接收引擎能力元数据，让 LLM（或规则系统）做出更精准的选择：

```go
// 查询所有引擎的能力
infos := client.EngineInfos()
// []EngineInfo{{Name: "kling", Capability: {MediaTypes: ["video"], MaxDuration: 10, ...}}, ...}

// RichSelector 自动接收能力信息，无需额外代码
result, err := client.ExecuteTaskAuto(ctx, myRichSelector, task)
```

### 规则预过滤

在 LLM 选择前，按媒体类型、尺寸、时长、音色过滤不兼容的引擎：

```go
filter := &aigo.RuleFilter{}
candidates := filter.Filter(task, client.EngineInfos())
// 仅保留满足任务约束的引擎
```

### 优先级选择器（无需 LLM）

按优先级列表选择第一个兼容的引擎：

```go
selector := &aigo.PrioritySelector{
    Priority: []string{"kling", "luma", "runway"},
    Filter:   &aigo.RuleFilter{}, // 可选的约束过滤
}
result, err := client.ExecuteTaskAuto(ctx, selector, task)
```

### 自动推断媒体类型

从任务字段自动检测所需的媒体类型：

```go
mediaType := aigo.InferMediaType(task)
// Duration > 0 → "video"，TTS → "audio"，Music → "music"，默认 → "image"
```

### 引擎故障转移

按顺序尝试多个引擎，首个成功即返回：

```go
result, err := client.ExecuteWithFallback(ctx, []string{"primary", "backup"}, graph)
// result.Engine       — 成功的引擎
// result.Output.Value — 结果
// result.Skipped      — 失败的引擎列表（含错误信息）
```

### 自动路由 + 故障转移

选择器路由与自动降级组合使用：

```go
result, err := client.ExecuteTaskAutoWithFallback(ctx, selector, task)
// 选择器选出最佳引擎；若失败，自动尝试其余候选引擎
```

### 异步执行

通过 channel 实现非阻塞执行：

```go
ch := client.ExecuteAsync(ctx, "video", graph)
// ... 做其他事情 ...
ar := <-ch
if ar.Err != nil { ... }
fmt.Println(ar.Result.Value)
```

## 低层 API

如果你的 Agent 已经能直接生成工作流图，直接调用 `Execute`：

```go
graph := workflow.Graph{
    "1": {
        ClassType: "CLIPTextEncode",
        Inputs:    map[string]any{"text": "一座暴风雨中的灯塔，电影构图"},
    },
    "2": {
        ClassType: "EmptyLatentImage",
        Inputs:    map[string]any{"width": 1536, "height": 1024},
    },
}

result, err := client.Execute(ctx, "img", graph)
```

## 内部包

| 包 | 用途 |
|----|------|
| `workflow` | 工作流图类型与校验 |
| `workflow/resolve` | 图解析（提示词提取、选项辅助函数、链接跟随） |
| `engine/poll` | 统一轮询（退避、进度回调、状态映射） |
| `engine/httpx` | HTTP 客户端默认值、重试传输层、限流、文件上传 |
| `engine/aigoerr` | 结构化错误分类，用于 Agent 重试逻辑 |
| `engine/embed` | 向量嵌入引擎（OpenAI、Gemini、Jina、Voyage、Aliyun） |
| `tooldef` | JSON Schema 工具定义，适配各类 Agent 框架 |

## 示例

```bash
# 阿里云百炼
go run ./examples/alibabacloud_qwen_image
go run ./examples/alibabacloud_wan_image
go run ./examples/alibabacloud_zimage
go run ./examples/alibabacloud_wan_t2v
go run ./examples/alibabacloud_wan_r2v
go run ./examples/alibabacloud_wan_videoedit
go run ./examples/alibabacloud_qwen_tts
go run ./examples/alibabacloud_qwen_voice_design

# New API 网关
go run ./examples/newapi_image
go run ./examples/newapi_speech
go run ./examples/newapi_video

# 自动路由
go run ./examples/agent_auto_router
```

## 说明

- 阿里云返回的结果 URL 是临时 OSS 链接，拿到后应立即保存。
- 所有异步引擎支持 `WaitForCompletion` 模式用于同步调用，以及 `Resume()` 用于重连运行中的任务。
- 所有引擎通过 `engine.ResolveKey` 统一 API Key 解析 — 支持结构体字段或环境变量配置。
