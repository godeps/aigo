# aigo Architecture

## Overview

aigo is a Go SDK that acts as a "virtual ComfyUI engine" — a workflow compiler sitting between AI agents and multimodal backends. Agents express intent as a lightweight DAG (directed acyclic graph), and aigo compiles that graph into backend-specific API calls.

## Core Abstractions

### workflow.Graph

The universal intermediate representation. A `Graph` is `map[string]Node`, where each `Node` has a `ClassType` (e.g., `CLIPTextEncode`, `EmptyLatentImage`) and an `Inputs` map. This format is inspired by ComfyUI's node graph but kept minimal.

### engine.Engine

The single-method interface all backends implement:

```go
type Engine interface {
    Execute(ctx context.Context, graph workflow.Graph) (Result, error)
}
```

`Result` carries both the raw output string and a `Kind` (URL, DataURI, JSON, PlainText) so callers can handle results without guessing.

### Client

The top-level router. Registers named engines and dispatches graphs:

- `Execute` — direct graph dispatch
- `ExecutePrompt` / `ExecuteTask` — high-level agent API (auto-builds graph)
- `ExecuteTaskAuto` — selector-driven routing
- `ExecuteWithFallback` — ordered engine failover
- `ExecuteAsync` — non-blocking channel-based execution

## Engine Implementations

### engine/aliyun

Alibaba Cloud Bailian / DashScope. Uses a model dispatch table (`map[string]modelEntry`) to route to the correct handler (image gen, video gen, TTS, voice design). Long-running tasks use async polling via `engine/poll`.

### engine/openai

OpenAI DALL-E image generation. Extracts prompt, dimensions, and quality from the graph and calls the images/generations endpoint.

### engine/newapi

Multi-route gateway supporting 13 route families (OpenAI-compat, Kling, Jimeng, Sora, Qwen, Gemini). Uses a dispatch table (`map[Route]routeExec`) with per-route graph extractors. Supports both sync and poll-based async flows.

### engine/comfyui

Direct passthrough to a real ComfyUI server via HTTP + WebSocket. Queues the prompt and tracks execution status.

## Shared Infrastructure

### workflow/resolve

Extracted from 3 duplicate implementations across engines. Provides:

- `ResolveNodeString` — recursive node string resolution with cycle detection
- `StringOption` / `IntOption` / `BoolOption` / `Float64Option` — type-safe graph option extraction
- `NormalizeOpenAIImageSize` — width/height to standard size string

### engine/poll

Unified polling loop replacing 5 ad-hoc ticker implementations:

- Configurable interval, backoff multiplier, max interval, max attempts
- Optional `OnProgress` callback for progress monitoring
- Context-aware cancellation

### engine/httpx

HTTP client factory with sensible defaults (timeout, transport settings).

## Data Flow

```
1. Agent creates AgentTask (prompt, dimensions, references, TTS options, etc.)
2. BuildGraph() compiles AgentTask into workflow.Graph
3. Client.Execute() validates the graph, looks up the engine, dispatches
4. Engine extracts relevant fields from graph nodes
5. Engine calls backend API (sync or async with polling)
6. Result returned with Value + Kind classification
```

## Design Decisions

- **Zero external dependencies**: Only Go stdlib. Keeps the dependency tree clean for embedding in larger systems.
- **Graph as IR**: The ComfyUI-inspired graph format provides a universal language between agents and backends, even when the backend has no concept of nodes.
- **Structured output**: `Client.Execute` returns `Result` with `Value`, `Kind`, `Engine`, and `Elapsed` fields.
- **Thin wrappers over shared code**: Engine-specific packages keep their exported API but delegate to `workflow/resolve` internally.
