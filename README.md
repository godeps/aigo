# aigo

[中文说明](./README.zh-CN.md)

`aigo` is a Go SDK that lets an agent describe media generation work as a lightweight workflow graph, then route that graph to different execution engines.

Current engines:

- `comfyui`: passthrough to a real ComfyUI server
- `openai`: graph flattening for image generation
- `aliyun`: Alibaba Cloud Model Studio / Bailian image and video models

## Install

```bash
go get github.com/godeps/aigo
```

## Core Idea

The SDK separates two concerns:

- Agents generate intent as a workflow graph
- Engines compile that graph into backend-specific API calls

The low-level workflow format is:

```go
type Node struct {
	ClassType string
	Inputs    map[string]any
}

type Graph map[string]Node
```

## Agent-Friendly API

Agents do not need to build graphs manually anymore. Use `ExecutePrompt` for the simplest case, or `ExecuteTask` for structured media generation.

```go
client := aigo.NewClient()

_ = client.RegisterEngine("img", aliyun.New(aliyun.Config{
	Model: aliyun.ModelQwenImage,
}))

result, err := client.ExecutePrompt(ctx, "img", "A shiba inu riding a vintage motorcycle")
```

Structured request:

```go
result, err := client.ExecuteTask(ctx, "video", aigo.AgentTask{
	Prompt:   "Turn this product scene into a short ad",
	Size:     "1280*720",
	Duration: 2,
	References: []aigo.ReferenceAsset{
		{Type: aigo.ReferenceTypeVideo, URL: "https://example.com/input.mp4"},
		{Type: aigo.ReferenceTypeImage, URL: "https://example.com/style.png"},
	},
})
```

`ExecuteTask` compiles the request into a workflow graph internally and forwards it to the registered engine.

If you want the LLM inside your agent to choose the engine on its own, use `ExecutePromptAuto` or `ExecuteTaskAuto` with a selector:

```go
result, err := client.ExecuteTaskAuto(ctx, selector, aigo.AgentTask{
	Prompt:   "make a 2 second product video",
	Duration: 2,
	Size:     "1280*720",
})
```

The selector only decides routing. The actual generation still runs through the chosen engine.

## Low-Level API

If your agent already emits workflow graphs, call `Execute` directly:

```go
graph := workflow.Graph{
	"1": {
		ClassType: "CLIPTextEncode",
		Inputs: map[string]any{
			"text": "A cinematic lighthouse in a storm",
		},
	},
	"2": {
		ClassType: "EmptyLatentImage",
		Inputs: map[string]any{
			"width":  1536,
			"height": 1024,
		},
	},
}

result, err := client.Execute(ctx, "img", graph)
```

## Alibaba Cloud Models

`engine/aliyun` currently supports:

- `aliyun.ModelQwenImage`
- `aliyun.ModelWanImage`
- `aliyun.ModelZImageTurbo`
- `aliyun.ModelWanTextToVideo`
- `aliyun.ModelWanReferenceVideo`
- `aliyun.ModelWanVideoEdit`

Environment variable:

```bash
export DASHSCOPE_API_KEY=your_key
```

## Examples

Runnable examples:

- `go run ./examples/aliyun_qwen_image`
- `go run ./examples/aliyun_wan_image`
- `go run ./examples/aliyun_zimage`
- `go run ./examples/agent_auto_router`
- `go run ./examples/aliyun_wan_t2v`
- `go run ./examples/aliyun_wan_r2v`
- `go run ./examples/aliyun_wan_videoedit`

## Notes

- Alibaba Cloud result URLs are temporary. Persist them immediately.
- As of April 5, 2026, Alibaba Cloud public docs still expose `wan2.6-t2v` and `wan2.6-r2v` for text-to-video and reference-to-video, while `wan2.7-videoedit` is the public video editing model.
