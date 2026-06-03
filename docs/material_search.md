# 素材检索 (Material Search)

`material` 包提供统一的素材检索接口，支持从多个外部平台和本地素材库搜索图片、视频、音频和文档资源。

## 架构

```
material.NewFromURIs("pexels://K1,unsplash://K2,oss://AK:SK@bucket.region")
         │
         ▼
   MultiSearcher ─── 并发查询 ───┬── Pexels
                                  ├── Unsplash
                                  ├── Pixabay
                                  ├── OSS MetaQuery
                                  └── Local (向量搜索)
         │
         ▼
   material.Result{Items, Total, Source}
```

## 快速开始

### URI 方式（推荐）

```go
import _ "github.com/godeps/aigo/material/all"

// 逗号分割多后端聚合
searcher, err := material.NewFromURIs("pexels://YOUR_KEY,unsplash://YOUR_KEY")
if err != nil {
    log.Fatal(err)
}

result, err := searcher.Search(ctx, material.Request{
    Query:      "雪山日出",
    MediaTypes: []string{"image"},
    MaxResults: 20,
})

for _, item := range result.Items {
    fmt.Printf("[%s] %s → %s\n", item.Source, item.Caption, item.PreviewURL)
}
```

### 手动构建

```go
import (
    "github.com/godeps/aigo/material"
    "github.com/godeps/aigo/material/pexels"
    "github.com/godeps/aigo/material/ossmeta"
)

p := pexels.New(pexels.Config{APIKey: os.Getenv("PEXELS_API_KEY")})
o, _ := ossmeta.New(ossmeta.Config{
    AccessKeyID:     os.Getenv("OSS_ACCESS_KEY_ID"),
    AccessKeySecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
    Region:          "cn-hangzhou",
    Bucket:          "my-assets",
    Mode:            ossmeta.ModeSemantic,
})

multi := material.NewMultiSearcher(p, o)
result, _ := multi.Search(ctx, material.Request{Query: "森林航拍"})
```

## URI 格式

| 后端 | URI 格式 | 示例 |
|------|----------|------|
| Pexels | `pexels://<API_KEY>` | `pexels://abc123def456` |
| Unsplash | `unsplash://<ACCESS_KEY>` | `unsplash://your-access-key` |
| Pixabay | `pixabay://<API_KEY>` | `pixabay://12345678-abcdef` |
| OSS | `oss://<AK_ID>:<AK_SECRET>@<BUCKET>.<REGION>?mode=semantic` | `oss://LTAI5t:wJalr@my-bucket.cn-hangzhou?mode=semantic` |
| Local | `local://<INDEX_PATH>?embed=dashscope&embed_key=<KEY>` | `local://.aigo/index.json?embed=jina&embed_key=KEY` |

### OSS URI 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `mode` | 检索模式：`basic`（标量）或 `semantic`（向量） | `semantic` |
| `token` | STS 临时安全令牌 | 空 |

### Local URI 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `embed` | 嵌入后端：`dashscope`, `jina`, `openai`, `voyage`, `gemini` | `dashscope` |
| `embed_key` | 嵌入引擎 API Key | 空 |
| `embed_model` | 模型覆盖 | 各后端默认 |

## 接口定义

### Searcher

```go
type Searcher interface {
    Search(ctx context.Context, req Request) (Result, error)
}
```

### Request

| 字段 | 类型 | 说明 |
|------|------|------|
| `Query` | string | 搜索关键词或自然语言描述（必填） |
| `MediaTypes` | []string | 媒体类型过滤：`image`, `video`, `audio`, `document` |
| `Tags` | []string | 标签过滤 |
| `MaxResults` | int | 最大返回数量 (1-100)，默认 20 |
| `Page` | int | 分页页码（1-based） |
| `Sort` | string | 排序：`relevance`, `newest`, `popular` |
| `Order` | string | 排序方向：`asc`, `desc` |
| `NextToken` | string | 翻页游标 |
| `Locale` | string | 语言：`zh`, `en` 等 |
| `FieldQuery` | string | OSS 标量检索 JSON 条件 |
| `SimpleQuery` | string | OSS 语义检索附加过滤 |

### Result

| 字段 | 类型 | 说明 |
|------|------|------|
| `Items` | []Item | 搜索结果列表 |
| `Total` | int | 总匹配数 |
| `NextToken` | string | 下一页游标 |
| `Source` | string | 来源标识 |

### Item

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | string | 平台内唯一标识 |
| `URI` | string | 资源完整路径 |
| `Filename` | string | 文件名 |
| `PreviewURL` | string | 预览/缩略图 URL |
| `DownloadURL` | string | 下载 URL |
| `Size` | int64 | 文件大小（字节） |
| `MediaType` | string | 媒体类型 |
| `ContentType` | string | MIME 类型 |
| `Width` | int | 宽度（像素） |
| `Height` | int | 高度（像素） |
| `Duration` | float64 | 时长（秒，视频/音频） |
| `Source` | string | 来源平台 |
| `Author` | string | 作者/摄影师 |
| `License` | string | 授权方式 |
| `Tags` | []string | 标签列表 |
| `Score` | float64 | 相关度分数（向量搜索） |
| `Caption` | string | 描述/标题 |
| `Metadata` | map[string]string | 扩展元数据 |

## 各后端详细说明

### Pexels

- 支持图片和视频搜索
- 授权方式：HTTP Header `Authorization: <API_KEY>`
- 免费 200 请求/月，申请：https://www.pexels.com/api/
- License：Pexels License（免费商用，无需署名）

### Unsplash

- 仅支持图片搜索
- 授权方式：HTTP Header `Authorization: Client-ID <ACCESS_KEY>`
- 免费 50 请求/小时，申请：https://unsplash.com/developers
- License：Unsplash License（免费商用，无需署名）

### Pixabay

- 支持图片和视频搜索
- 授权方式：URL 参数 `key=<API_KEY>`
- 免费无限制（合理使用），申请：https://pixabay.com/api/docs/
- License：Pixabay License（免费商用，无需署名）

### OSS MetaQuery

- 支持标量检索（basic）和语义检索（semantic）
- 标量检索：按字段条件（文件名、大小、标签等）精确查询
- 语义检索：自然语言描述搜索多媒体内容（图片、视频、音频、文档）
- 依赖：`github.com/aliyun/alibabacloud-oss-go-sdk-v2`
- 前置条件：Bucket 需开启数据索引功能
- 返回丰富元数据：Insights（AI 描述）、地理位置、音视频流信息

#### 标量检索 Query 示例

```json
{"Field": "Size", "Value": "1048576", "Operation": "gt"}
```

嵌套查询：

```json
{
  "SubQueries": [
    {"Field": "Filename", "Value": "photos/", "Operation": "prefix"},
    {"Field": "Size", "Value": "1048576", "Operation": "gt"}
  ],
  "Operation": "and"
}
```

### Local (本地向量搜索)

- 基于已有 `embed.EmbedEngine` 进行向量化
- 余弦相似度排序
- 索引持久化为 JSON 文件
- 使用前需建立索引：

```go
localSearcher, _ := local.New(local.Config{
    EmbedEngine: embedEngine,
    IndexPath:   ".aigo/search_index.json",
})

// 索引目录
count, _ := localSearcher.IndexDir(ctx, "/path/to/assets")
fmt.Printf("indexed %d files\n", count)

// 搜索
result, _ := localSearcher.Search(ctx, material.Request{
    Query:      "蓝天白云",
    MediaTypes: []string{"image"},
})
```

## Agent 工具集成

`tooldef.SearchMaterial()` 为 AI Agent 提供素材搜索工具定义：

```go
tools := tooldef.AllTools() // 包含 search_material
```

工具参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 搜索关键词或自然语言描述 |
| `media_type` | string | 否 | `image`, `video`, `audio`, `document` |
| `source` | string | 否 | `all`, `pexels`, `unsplash`, `pixabay`, `oss`, `local` |
| `max_results` | integer | 否 | 返回数量（1-100），默认 20 |
| `tags` | string | 否 | 逗号分隔的标签 |
| `sort` | string | 否 | `relevance`, `newest`, `popular` |

## 环境变量配置

支持三种配置方式，按优先级从高到低：

### 方式一：聚合 URI（推荐，一个变量搞定所有后端）

```bash
export MATERIAL_URIS="pexels://KEY1,unsplash://KEY2,pixabay://KEY3,oss://AK:SK@bucket.cn-hangzhou?mode=semantic"
```

```go
import _ "github.com/godeps/aigo/material/all"

searcher, err := material.NewFromEnv()
```

### 方式二：独立 URI（按后端分别配置）

| 环境变量 | URI 格式 | 示例 |
|----------|----------|------|
| `PEXELS_URI` | `pexels://<API_KEY>` | `pexels://abc123` |
| `UNSPLASH_URI` | `unsplash://<ACCESS_KEY>` | `unsplash://def456` |
| `PIXABAY_URI` | `pixabay://<API_KEY>` | `pixabay://ghi789` |
| `OSS_META_URI` | `oss://<AK>:<SK>@<BUCKET>.<REGION>?mode=semantic` | `oss://LTAI:wJal@assets.cn-hangzhou` |
| `LOCAL_MATERIAL_URI` | `local://<PATH>?embed=<BACKEND>&embed_key=<KEY>` | `local://.aigo/index.json?embed=jina&embed_key=KEY` |

```go
searcher, err := material.NewFromEnv()  // 自动聚合所有已设置的 *_URI 变量
```

### 方式三：传统 Key 变量（兼容零配置构造）

| 变量 | 后端 | 说明 |
|------|------|------|
| `PEXELS_API_KEY` | Pexels | API Key（Config 为空时自动读取） |
| `UNSPLASH_ACCESS_KEY` | Unsplash | Access Key（Config 为空时自动读取） |
| `PIXABAY_API_KEY` | Pixabay | API Key（Config 为空时自动读取） |
| `OSS_ACCESS_KEY_ID` | OSS | AccessKey ID |
| `OSS_ACCESS_KEY_SECRET` | OSS | AccessKey Secret |
| `OSS_BUCKET` | OSS | Bucket 名称 |
| `OSS_REGION` | OSS | 地域（如 cn-hangzhou） |

```go
// 全零配置 — 从环境变量自动读取
searcher := pexels.New(pexels.Config{})
ossSearcher, _ := ossmeta.New(ossmeta.Config{})
```

### 优先级

```
MATERIAL_URIS (方式一) > 各 *_URI (方式二) > 各 *_API_KEY / OSS_* (方式三)
```

`NewFromEnv()` 使用方式一和方式二；各后端的 `New(Config{})` 使用方式三。
