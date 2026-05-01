# 阿里云百炼（DashScope）模型接入指南

本文档汇总阿里云百炼（Model Studio / DashScope）平台的官方资料入口，
记录在 `engine/alibabacloud` 中接入新模型时的标准流程，
方便后续新增模型时快速定位文档与代码骨架。

---

## 官方文档入口

### 一级入口

| 用途 | 链接 |
|------|------|
| **模型总目录** | <https://help.aliyun.com/zh/model-studio/models> |
| 控制台 | <https://bailian.console.aliyun.com> |
| API 鉴权 / API Key | <https://help.aliyun.com/zh/model-studio/get-api-key> |
| 异步任务规范 | DashScope `X-DashScope-Async: enable` + `GET /api/v1/tasks/{task_id}` |

> **新增模型时的固定动作**：先到「模型总目录」按能力（图像 / 视频 / 语音 / 3D 等）找到目标模型，
> 点进去拿到 API Reference URL，然后按下面的接入清单落地代码。

### 已接入模型的 API Reference

| 模型常量 | 能力 | API Reference |
|----------|------|---------------|
| `ModelQwenImage` | image | <https://help.aliyun.com/zh/model-studio/text-to-image-v2-api-reference> |
| `ModelQwenImage2` | image | <https://help.aliyun.com/zh/model-studio/qwen-image-api-reference> |
| `ModelQwenImageEditPlus` | image_edit | <https://help.aliyun.com/zh/model-studio/qwen-image-edit-api-reference> |
| `ModelWanImage` | image | <https://help.aliyun.com/zh/model-studio/wanx-image-api-reference> |
| `ModelZImageTurbo` | image | <https://help.aliyun.com/zh/model-studio/z-image-api-reference> |
| `ModelWanTextToVideo` | video | <https://help.aliyun.com/zh/model-studio/wan-text-to-video-api-reference> |
| `ModelWanImageToVideo` | video | <https://help.aliyun.com/zh/model-studio/wan-image-to-video-api-reference> |
| `ModelWanReferenceVideo` | video | <https://help.aliyun.com/zh/model-studio/wan-reference-to-video-api-reference> |
| `ModelWanVideoEdit` | video_edit | <https://help.aliyun.com/zh/model-studio/wan-video-edit-api-reference> |
| `ModelKlingV3Video` | video | <https://help.aliyun.com/zh/model-studio/kling-v3-api-reference> |
| `ModelKlingV3OmniVideo` | video | <https://help.aliyun.com/zh/model-studio/kling-v3-omni-api-reference> |
| `ModelQwenTTSFlash` | tts | <https://help.aliyun.com/zh/model-studio/qwen-tts-api-reference> |
| `ModelQwenTTSInstructFlash` | tts | <https://help.aliyun.com/zh/model-studio/qwen-tts-instruct-api-reference> |
| `ModelQwenVoiceDesign` | voice_design | <https://help.aliyun.com/zh/model-studio/voice-design-api-reference> |
| `ModelQwenASRFlash` | asr | <https://help.aliyun.com/zh/model-studio/qwen-asr-flash-api-reference> |
| `ModelQwenASRFlashFiletrans` | asr | <https://help.aliyun.com/zh/model-studio/qwen-asr-flash-filetrans-api-reference> |
| `ModelTripoP1` / `ModelTripoH31` | 3d | <https://help.aliyun.com/zh/model-studio/tripo-3d-generation-api-reference> |

---

## 接入新模型的标准流程

### 第 1 步：分类与定位

1. 打开 <https://help.aliyun.com/zh/model-studio/models>，按能力筛选（图像 / 视频 / 语音 / 3D 等）。
2. 点进具体模型的 API Reference 页面，记录以下要素：
   - HTTP 方法 + 路径（例如 `POST /api/v1/services/aigc/video-generation/3d-generation`）
   - 是否同步 / 异步（异步需要 `X-DashScope-Async: enable` 头 + 任务轮询）
   - 请求体字段（`model` / `input` / `parameters`）
   - 响应体结构（成功路径里结果 URL 字段嵌套位置）

### 第 2 步：选择子包

`engine/alibabacloud/internal/` 已按能力分包：

| 子包 | 能力 |
|------|------|
| `imggen/` | 图片生成 / 编辑 |
| `vidgen/` | 视频生成 / 编辑 |
| `audiogen/` | TTS / 声音设计 / ASR |
| `threedgen/` | 3D 资产生成（如 Tripo 系列） |

新模型若属于上述能力，复用现有子包；新增能力则新建对应子包。

### 第 3 步：编写 handler

handler 签名固定：

```go
func RunXxx(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error)
```

实现要点：

1. 用 `graphx.Prompt` / `graphx.ImageURLs` / `graphx.StringOption` 等从 workflow 抽取入参。
2. 拼装 `payload := map[string]any{"model": model, "input": ..., "parameters": ...}`。
3. 调用 `async.Submit(ctx, rt, apiKey, "<相对路径>", payload, async.URLExtractor{URLFields: ...})`。
   - `URLFields` 是「按顺序尝试的字段路径列表」，遇到 `[]any` 自动取第一个元素。
   - 例：`[][]string{{"results", "pbr_model_url"}}` 表示从 `output.results[0].pbr_model_url` 取值。
4. 同步接口（不需要轮询）直接用 `rt.HTTPClient.Do` 自行发请求即可，可参考 `imggen/multimodal_image.go`。

### 第 4 步：注册到 `engine.go`

在根 `engine/alibabacloud/engine.go` 中：

1. 加 `Model<Name>` 常量。
2. 加 `modelTable[Model<Name>] = {handler.RunXxx, engine.OutputURL/...}`。
3. 在 `Capabilities()` switch 中加分支返回 `MediaTypes: []string{"<capability>"}`。
4. 若是 edit / asr / dual 等特殊场景，更新 `editModels` / `asrModels` / `dualModels` 帮助表，
   `ModelsByCapability()` 会按这些表分组。

### 第 5 步：注册到 `resume.go`

在 `extractorForModel` 中加 case，返回与 handler 一致的 `URLExtractor`，
这样异步任务即使中断也能通过 `engine.Resumer` 续轮询。

### 第 6 步：补 i18n 元数据

在 `engine/alibabacloud/models.go` 中追加 `ModelInfo` 条目，必填：

- `Name` / `Provider`（恒为 `"alibabacloud"`）
- `DisplayName` / `Description` / `Intro`（en + zh 双语）
- `DocURL`（指向官方 API Reference）
- `Capability`（image / video / tts / asr / voice_design / 3d / image_edit / video_edit）

如果该引擎引入了**新能力维度**（例如「3d」），还要更新 `engine/names.go` 中
`EngineMetadataMap["alibabacloud"].Intro` 的能力描述。

### 第 7 步：写单元测试

放在 `engine/alibabacloud/engine_<能力>_test.go`，用 `httptest.NewServer` mock 异步流程：

- POST 路径返回 `{"output":{"task_id":"...","task_status":"PENDING"}}`
- GET `/api/v1/tasks/<id>` 返回 `{"output":{"task_status":"SUCCEEDED", ...}}`
- 断言：路径、`X-DashScope-Async` 头、payload 字段、最终 `out.Value` 是预期 URL。

`Config{WaitForCompletion: true, PollInterval: 5 * time.Millisecond}` 可让测试在毫秒级完成。

---

## 异步任务约定

### 提交

- 必须头：`X-DashScope-Async: enable`、`Authorization: Bearer <DASHSCOPE_API_KEY>`、`Content-Type: application/json`
- 响应：`{"output": {"task_id": "...", "task_status": "PENDING"}}`

### 轮询

- 路径：`GET /api/v1/tasks/{task_id}`（注意：是平台级的 `/tasks/`，不是模型自身路径）
- `task_status` 取值：`PENDING` / `RUNNING` / `SUCCEEDED` / `FAILED` / `CANCELED` / `UNKNOWN`
- 失败时通常带 `code` / `message`；本仓库的 `internal/async` 已自动拼到错误信息里。

### 结果 URL 提取

`async.URLExtractor.URLFields` 是路径列表，按顺序尝试：

```go
// 例 1：results 是数组，取第一个元素的 url 字段
URLFields: [][]string{{"results", "url"}, {"result_url"}}

// 例 2：3D 模型返回 results[0].pbr_model_url
URLFields: [][]string{{"results", "pbr_model_url"}}

// 例 3：视频返回顶层 video_url
URLFields: [][]string{{"video_url"}}
```

数组节点会自动取 `[0]`，避免每个 handler 重复处理 `[]any`。

---

## Tripo 3D 接入回顾（参考案例）

可作为新模型接入的「最小完整示例」：

- **Handler**: `engine/alibabacloud/internal/threedgen/tripo.go`
- **API**: `POST /api/v1/services/aigc/video-generation/3d-generation`（异步）
- **核心逻辑**: `prompt` / `image` / `images` 三选一互斥；`geometry_quality` 仅 `Tripo/Tripo-H3.1` 支持
- **结果字段**: `output.results[0].pbr_model_url`（`.glb` 文件）
- **测试**: `engine/alibabacloud/engine_3d_test.go`

按此案例新增任何百炼新模型，预期改动控制在 5 个文件以内：handler 一份、`engine.go` / `resume.go` / `models.go` / `<能力>_test.go` 各一处。
