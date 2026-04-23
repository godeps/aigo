# ModelInfo 元数据系统 — 优化路线图

> 本文档记录 ModelInfo 元数据系统的后续优化方向，供开发参考。
> 已完成的两轮优化见 git history。

---

## 短期优化（1-5）

### 1. ModelInfo 数据完整性补全

**现状**: 部分模型缺少 `Description["zh"]`，embed 和平台类模型尤为突出。没有价格/成本层级信息。

**建议**:
- 补全所有模型的 `Description["zh"]`
- 添加 `CostTier` 字段（如 `"free"`, `"standard"`, `"premium"`），让调用方无需硬编码价格即可做成本感知路由

```go
type ModelInfo struct {
    // ...existing fields...
    CostTier string `json:"cost_tier,omitempty"` // "free", "standard", "premium"
}
```

**优先级**: 高 — 直接影响用户体验和 i18n 质量

---

### 2. EngineInfo + Selector 深度整合

**现状**: `RichSelector` 做引擎级路由，但不感知模型级元数据。选择器无法按模型特征（分辨率、速度等）做细粒度路由。

**建议**:
- 实现 `ModelSelector`，支持按 ModelInfo 字段（Capability、Tags、CostTier 等）做模型级路由
- `RichSelector` 先选引擎，`ModelSelector` 再选模型

```go
type ModelSelector struct {
    Capability string
    Tags       []string
    CostTier   string
    // ...
}

func (s *ModelSelector) Select(models []engine.ModelInfo) (engine.ModelInfo, error)
```

**优先级**: 中 — 依赖 CostTier/Tags 数据先就位

---

### 3. ModelInfo 版本化

**现状**: 模型更新（如 kling-v2 → kling-v2.1）无法在元数据中体现时间线。

**建议**:
- 添加 `Version`、`ReleasedAt`、`UpdatedAt` 字段
- 配合 `Deprecated` 字段实现模型生命周期管理

```go
type ModelInfo struct {
    // ...existing fields...
    Version    string `json:"version,omitempty"`
    ReleasedAt string `json:"released_at,omitempty"` // RFC3339 date
    UpdatedAt  string `json:"updated_at,omitempty"`
}
```

**优先级**: 低 — 信息价值高但非阻塞性

---

### 4. Config 文件与 ModelInfo 校验

**现状**: 配置文件中引用模型名时没有校验，拼写错误只能在运行时发现。

**建议**:
- 在配置加载阶段，对 `engine_configs` 中的模型名调用 `LookupModelInfo()` 校验
- 不存在的模型名发出 warning（不阻塞启动）

```go
func ValidateConfigModels(cfg Config) []string {
    var warnings []string
    for _, name := range cfg.ModelNames() {
        if _, ok := engine.LookupModelInfo(name); !ok {
            warnings = append(warnings, fmt.Sprintf("unknown model: %s", name))
        }
    }
    return warnings
}
```

**优先级**: 中 — 提升开发体验，减少调试时间

---

### 5. tooldef + ModelInfo 整合

**现状**: `tooldef` 的 ToolDef 知道自己支持哪些 capability，但不知道对应有哪些具体模型。

**建议**:
- 添加 `ToolsWithModels()` 方法，返回 ToolDef + 匹配的 ModelInfo 列表
- 前端/CLI 可以展示「此工具可用的模型」

```go
type ToolDefWithModels struct {
    ToolDef
    Models []engine.ModelInfo `json:"models"`
}

func ToolsWithModels() []ToolDefWithModels {
    var result []ToolDefWithModels
    for _, td := range AllToolDefs() {
        models := engine.ModelInfosByCapability(td.Capability)
        result = append(result, ToolDefWithModels{ToolDef: td, Models: models})
    }
    return result
}
```

**优先级**: 中 — 对产品侧价值大

---

## 中长期演进（6-12）

### 6. 模型能力矩阵

**现状**: `Capability` 是粗粒度字符串（`"image"`, `"video"` 等），无法表达模型的具体参数约束。

**建议**:
- 添加结构化能力描述字段

```go
type ModelCapabilities struct {
    MaxResolution  string   `json:"max_resolution,omitempty"`  // "1920x1080"
    SupportedSizes []string `json:"supported_sizes,omitempty"` // ["512x512", "1024x1024"]
    InputTypes     []string `json:"input_types,omitempty"`     // ["text", "image", "video"]
    OutputTypes    []string `json:"output_types,omitempty"`    // ["image"]
    MaxDuration    int      `json:"max_duration,omitempty"`    // seconds, for video/audio
    MaxTokens      int      `json:"max_tokens,omitempty"`      // for text models
}

type ModelInfo struct {
    // ...existing fields...
    Capabilities *ModelCapabilities `json:"capabilities,omitempty"`
}
```

**优先级**: 中 — 结构化后可驱动前端表单自动生成

---

### 7. 模型性能基准

**现状**: 没有质量/速度/成功率等运行时指标的元数据。

**建议**:
- 添加静态基准评分（由人工或自动化测试填充）

```go
type ModelBenchmark struct {
    QualityScore float64 `json:"quality_score,omitempty"` // 0-100
    SpeedTier    string  `json:"speed_tier,omitempty"`    // "fast", "medium", "slow"
    SuccessRate  float64 `json:"success_rate,omitempty"`  // 0-1
}

type ModelInfo struct {
    // ...existing fields...
    Benchmark *ModelBenchmark `json:"benchmark,omitempty"`
}
```

**优先级**: 低 — 数据采集成本高，但对智能路由价值大

---

### 8. 动态模型发现

**现状**: 所有模型通过 `init()` 静态注册。新增模型需要改代码、重编译。

**建议**:
- 定义 `ModelDiscoverer` 接口，支持运行时从 API 发现模型

```go
type ModelDiscoverer interface {
    DiscoverModels(ctx context.Context) ([]ModelInfo, error)
}
```

- 在 `Client` 启动时可选执行 discovery，合并到注册表
- 适用于 openai/openrouter 等有 `/models` 端点的引擎

**优先级**: 低 — 架构变化大，需谨慎设计缓存和刷新策略

---

### 9. 模型别名与映射

**现状**: 用户必须使用精确的模型 API 名称，无法用简称或别名。

**建议**:
- 在 ModelInfo 中添加 `Aliases` 字段
- `LookupModelInfo()` 支持别名查找

```go
type ModelInfo struct {
    // ...existing fields...
    Aliases []string `json:"aliases,omitempty"` // e.g., ["kling", "kling-v2"]
}

// LookupModelInfo enhanced: check name first, then scan aliases
```

**优先级**: 中 — 用户体验提升明显，实现简单

---

### 10. 多租户模型权限

**现状**: 所有注册模型对所有用户可见，无访问控制。

**建议**:
- 添加 `Access` 字段标记模型可见性
- 在 `AllModelInfos()` / `SearchModelInfos()` 等查询方法中支持按 Access 过滤

```go
type ModelInfo struct {
    // ...existing fields...
    Access string `json:"access,omitempty"` // "public", "internal", "restricted"
}

func AllModelInfosFiltered(access string) []ModelInfo
```

**优先级**: 低 — 仅多租户场景需要

---

### 11. 模型 A/B 测试框架

**现状**: 无法在同 Capability 的模型间做流量分配实验。

**建议**:
- 基于 `ModelSelector` 扩展，支持按权重分流
- 结合 observability 收集对比数据

```go
type ABTestConfig struct {
    Name    string            `json:"name"`
    Weights map[string]int    `json:"weights"` // model_name -> weight
    Metrics []string          `json:"metrics"` // what to track
}
```

**优先级**: 低 — 需要 observability 基础设施先就位

---

### 12. Observability 增强

**现状**: 模型调用没有统一的指标采集和追踪。

**建议**:
- 在 ModelInfo 中添加运行时统计挂载点
- 引擎层面记录调用次数、延迟、错误率

```go
type UsageStats struct {
    TotalCalls   int64         `json:"total_calls"`
    SuccessCalls int64         `json:"success_calls"`
    AvgLatency   time.Duration `json:"avg_latency"`
    LastUsed     time.Time     `json:"last_used"`
}

// Per-model stats, stored in registry alongside ModelInfo
```

**优先级**: 中 — 对生产运维价值高，是 A/B 测试和智能路由的前提

---

## 实施建议

| 阶段 | 内容 | 依赖 |
|------|------|------|
| Phase 1 | #1 数据补全, #3 版本化, #9 别名 | 无 |
| Phase 2 | #4 Config 校验, #5 tooldef 整合 | Phase 1 |
| Phase 3 | #6 能力矩阵, #2 Selector 整合 | Phase 1 |
| Phase 4 | #12 Observability, #7 基准 | Phase 3 |
| Phase 5 | #8 动态发现, #10 权限, #11 A/B | Phase 4 |
