# aigo

[English README](./README.md)

`aigo` 是一个面向 Agent 的 Golang SDK。Agent 可以输出轻量工作流图，SDK 再把这张图路由到不同的多模态执行引擎。

当前支持的执行引擎：

- `comfyui`：透传到真实 ComfyUI 服务
- `openai`：把图降维成图片 API 请求
- `aliyun`：对接阿里云百炼 / Model Studio 的图片和视频模型

## 安装

```bash
go get github.com/godeps/aigo
```

## 核心思路

SDK 把问题拆成两层：

- Agent 负责表达意图，输出工作流图
- Engine 负责把工作流图编译成具体后端 API 请求

底层工作流结构很轻：

```go
type Node struct {
	ClassType string
	Inputs    map[string]any
}

type Graph map[string]Node
```

## 面向 Agent 的高层 API

现在 Agent 不需要自己拼图了。最简单场景直接用 `ExecutePrompt`，复杂场景用 `ExecuteTask`。

```go
client := aigo.NewClient()

_ = client.RegisterEngine("img", aliyun.New(aliyun.Config{
	Model: aliyun.ModelQwenImage,
}))

result, err := client.ExecutePrompt(ctx, "img", "一只骑复古摩托的柴犬，电影感")
```

结构化任务示例：

```go
result, err := client.ExecuteTask(ctx, "video", aigo.AgentTask{
	Prompt:   "把这个产品场景生成成一条 2 秒广告视频",
	Size:     "1280*720",
	Duration: 2,
	References: []aigo.ReferenceAsset{
		{Type: aigo.ReferenceTypeVideo, URL: "https://example.com/input.mp4"},
		{Type: aigo.ReferenceTypeImage, URL: "https://example.com/style.png"},
	},
})
```

`ExecuteTask` 会先把高层任务编译成 `workflow.Graph`，再路由到指定引擎。

## 低层 API

如果你的 Agent 已经能直接生成工作流图，就继续调用 `Execute`：

```go
graph := workflow.Graph{
	"1": {
		ClassType: "CLIPTextEncode",
		Inputs: map[string]any{
			"text": "一座暴风雨中的灯塔，电影构图",
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

## 阿里云百炼模型

`pkg/engine/aliyun` 当前支持：

- `aliyun.ModelQwenImage`
- `aliyun.ModelWanImage`
- `aliyun.ModelZImageTurbo`
- `aliyun.ModelWanTextToVideo`
- `aliyun.ModelWanReferenceVideo`
- `aliyun.ModelWanVideoEdit`

环境变量：

```bash
export DASHSCOPE_API_KEY=your_key
```

## 示例

可直接运行的示例：

- `go run ./examples/aliyun_qwen_image`
- `go run ./examples/aliyun_wan_image`
- `go run ./examples/aliyun_zimage`
- `go run ./examples/aliyun_wan_t2v`
- `go run ./examples/aliyun_wan_r2v`
- `go run ./examples/aliyun_wan_videoedit`

## 说明

- 阿里云返回的结果 URL 是临时 OSS 链接，拿到后应立即保存。
- 截至 `2026-04-05`，阿里云公开文档里文生视频和参考生视频的模型名仍为 `wan2.6-t2v`、`wan2.6-r2v`，公开的 `wan2.7` 视频模型是 `wan2.7-videoedit`。
