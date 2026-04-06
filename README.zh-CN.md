# aigo

[English README](./README.md)

`aigo` 是一个面向 Agent 的 Go SDK，用于多模态媒体生成。Agent 输出轻量工作流图，SDK 把图路由到不同的执行引擎，并返回结构化结果（含错误分类、重试提示和进度回调）。零外部依赖，仅使用 Go 标准库。

## 架构

```
Agent (LLM / 代码)
  │
  ▼
AgentTask ──► BuildGraph() ──► workflow.Graph (DAG)
                                    │
                     ┌──────────────┼──────────────┐──────────────┐
                     ▼              ▼              ▼              ▼
               engine/aliyun  engine/openai  engine/newapi  engine/comfyui
                     │              │              │              │
                     ▼              ▼              ▼              ▼
                百炼 API       DALL-E API     多路网关        ComfyUI WS
```

## 引擎

| 引擎 | 后端 | 能力 |
|------|------|-----|
| `aliyun` | 阿里云百炼 / DashScope | 图片、视频、TTS、声音设计 |
| `openai` | OpenAI DALL-E | 图片生成 |
| `newapi` | 多路网关 | OpenAI 兼容、可灵、即梦、Sora、通义千问、Gemini |
| `comfyui` | ComfyUI 服务 | WebSocket 全透传 |

## 安装

```bash
go get github.com/godeps/aigo
```

## 快速开始

### 简单提示词

```go
client := aigo.NewClient()

_ = client.RegisterEngine("img", aliyun.New(aliyun.Config{
    Model: aliyun.ModelQwenImage,
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

tools := tooldef.AllTools() // generate_image, generate_video, text_to_speech, ...
// 注册到你的框架的 function-calling 系统
```

### 引擎能力发现

查询引擎的能力 — 用于动态工具选择：

```go
cap, _ := client.EngineCapabilities("aliyun-img")
// cap.MediaTypes  → ["image"]
// cap.Models      → ["qwen-image"]
// cap.SupportsSync, cap.SupportsPoll

// 查找所有支持视频的引擎：
videoEngines := client.AvailableFor("video")
```

### 进度上报

监控长时间运行的任务：

```go
result, err := client.Execute(ctx, "video", graph, aigo.WithProgress(func(e aigo.ProgressEvent) {
    fmt.Printf("[%s] elapsed=%s\n", e.Phase, e.Elapsed)
    // Phase: "submitted", "completed"
}))
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

### 引擎故障转移

按顺序尝试多个引擎，首个成功即返回：

```go
result, err := client.ExecuteWithFallback(ctx, []string{"primary", "backup"}, graph)
// result.Engine       — 成功的引擎
// result.Output.Value — 结果
// result.Skipped      — 失败的引擎列表（含错误信息）
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

## 阿里云百炼模型

`engine/aliyun` 支持：

| 常量 | 模型 | 类型 |
|------|------|------|
| `ModelQwenImage` | qwen-image | 图片 |
| `ModelWanImage` | wan2.7-image | 图片 |
| `ModelZImageTurbo` | z-image-turbo | 图片 |
| `ModelWanTextToVideo` | wan2.6-t2v | 视频 |
| `ModelWanReferenceVideo` | wan2.6-r2v | 视频 |
| `ModelWanVideoEdit` | wan2.7-videoedit | 视频 |
| `ModelQwenTTSFlash` | qwen3-tts-flash | TTS |
| `ModelQwenTTSInstructFlash` | qwen3-tts-instruct-flash | TTS |
| `ModelQwenVoiceDesign` | qwen-voice-design | 声音设计 |

环境变量：

```bash
export DASHSCOPE_API_KEY=your_key
```

## New API 多路网关

`engine/newapi` 通过单一网关支持多个路由族：

| 路由 | 协议 |
|------|------|
| `RouteOpenAIImagesGenerations` | POST /v1/images/generations |
| `RouteOpenAIImagesEdits` | POST /v1/images/edits |
| `RouteOpenAIVideoGenerations` | POST+GET /v1/video/generations |
| `RouteOpenAISpeech` | POST /v1/audio/speech |
| `RouteKlingText2Video` | 可灵文生视频 |
| `RouteKlingImage2Video` | 可灵图生视频 |
| `RouteJimengVideo` | 即梦（火山引擎）视频 |
| `RouteSoraVideos` | OpenAI Sora 视频 |
| `RouteQwenImagesGenerations` | 通义千问图片生成 |
| `RouteGeminiGenerateContent` | Gemini 原生 generateContent |

环境变量：

```bash
export NEWAPI_BASE_URL=https://your-gateway.example.com
export NEWAPI_API_KEY=your_key
```

## 内部包

| 包 | 用途 |
|----|------|
| `workflow/resolve` | 共享图解析（节点字符串提取、选项辅助函数、链接跟随） |
| `engine/poll` | 统一轮询（指数退避、最大重试次数、进度回调） |
| `engine/httpx` | HTTP 客户端默认值与辅助函数 |
| `engine/aigoerr` | 结构化错误分类，用于 Agent 重试逻辑 |
| `tooldef` | JSON Schema 工具定义，适配各类 Agent 框架 |

## 示例

```bash
# 阿里云百炼
go run ./examples/aliyun_qwen_image
go run ./examples/aliyun_wan_image
go run ./examples/aliyun_zimage
go run ./examples/aliyun_wan_t2v
go run ./examples/aliyun_wan_r2v
go run ./examples/aliyun_wan_videoedit
go run ./examples/aliyun_qwen_tts
go run ./examples/aliyun_qwen_voice_design

# New API 网关
go run ./examples/newapi_image
go run ./examples/newapi_speech
go run ./examples/newapi_video

# 自动路由
go run ./examples/agent_auto_router
```

## 说明

- 阿里云返回的结果 URL 是临时 OSS 链接，拿到后应立即保存。
- 截至 2026 年 4 月，阿里云公开文档里文生视频和参考生视频的模型名仍为 `wan2.6-t2v`、`wan2.6-r2v`，公开的 `wan2.7` 视频模型是 `wan2.7-videoedit`。
