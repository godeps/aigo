package aigo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

var (
	ErrEngineNil       = errors.New("aigo: engine is nil")
	ErrEngineExists    = errors.New("aigo: engine already registered")
	ErrEngineNotFound  = errors.New("aigo: engine not found")
	ErrEngineNameEmpty = errors.New("aigo: engine name is empty")
)

// ReferenceType identifies the kind of remote asset to attach to an agent task.
type ReferenceType string

const (
	ReferenceTypeImage ReferenceType = "image"
	ReferenceTypeVideo ReferenceType = "video"
)

// ReferenceAsset describes an externally reachable media input.
type ReferenceAsset struct {
	Type ReferenceType
	URL  string
}

// AgentTask is a graph-free request shape for agents.
type AgentTask struct {
	Prompt         string
	NegativePrompt string
	Width          int
	Height         int
	Size           string
	Duration       int
	Watermark      *bool
	References     []ReferenceAsset

	// Qwen TTS（aliyun 引擎、qwen3-tts-* 等）：与 Prompt 一起使用。
	Voice                string
	LanguageType         string
	Instructions         string
	OptimizeInstructions *bool

	// Qwen 声音设计（aliyun 引擎、model=qwen-voice-design）：三字段均需设置。
	VoicePrompt               string
	PreviewText               string
	TargetModel               string
	VoiceDesignPreferredName  string
	VoiceDesignLanguage       string
	VoiceDesignSampleRate     int
	VoiceDesignResponseFormat string
	// OmitVoiceDesignPreview 为 true 时，Execute 返回的 JSON 不含预览音频 data URI。
	OmitVoiceDesignPreview bool
}

// Selection is the selector's routing decision.
type Selection struct {
	Engine string
	Reason string
}

// RoutedResult is the result of a selector-driven execution.
type RoutedResult struct {
	Engine string
	Reason string
	Output string
}

// Selector decides which registered engine should handle a task.
type Selector interface {
	SelectEngine(ctx context.Context, task AgentTask, engines []string) (Selection, error)
}

// Client routes a workflow graph to a registered execution engine.
type Client struct {
	mu      sync.RWMutex
	engines map[string]engine.Engine
}

// NewClient creates a new SDK client.
func NewClient() *Client {
	return &Client{
		engines: make(map[string]engine.Engine),
	}
}

// RegisterEngine registers an engine under a logical name.
func (c *Client) RegisterEngine(name string, e engine.Engine) error {
	if name == "" {
		return ErrEngineNameEmpty
	}
	if e == nil {
		return ErrEngineNil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.engines[name]; exists {
		return fmt.Errorf("%w: %s", ErrEngineExists, name)
	}

	c.engines[name] = e
	return nil
}

// Execute dispatches the graph to the named engine.
func (c *Client) Execute(ctx context.Context, engineName string, graph workflow.Graph) (string, error) {
	c.mu.RLock()
	e, ok := c.engines[engineName]
	c.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrEngineNotFound, engineName)
	}

	result, err := e.Execute(ctx, graph)
	if err != nil {
		return "", fmt.Errorf("aigo: execute with engine %q: %w", engineName, err)
	}

	return result, nil
}

// ExecutePrompt runs the simplest agent request: a single prompt.
func (c *Client) ExecutePrompt(ctx context.Context, engineName string, prompt string) (string, error) {
	return c.ExecuteTask(ctx, engineName, AgentTask{Prompt: prompt})
}

// ExecuteTask converts an agent task into a workflow graph and routes it to the target engine.
func (c *Client) ExecuteTask(ctx context.Context, engineName string, task AgentTask) (string, error) {
	return c.Execute(ctx, engineName, BuildGraph(task))
}

// ExecutePromptAuto lets a selector choose the engine for a prompt-driven request.
func (c *Client) ExecutePromptAuto(ctx context.Context, selector Selector, prompt string) (RoutedResult, error) {
	return c.ExecuteTaskAuto(ctx, selector, AgentTask{Prompt: prompt})
}

// ExecuteTaskAuto lets a selector choose the engine for a structured agent task.
func (c *Client) ExecuteTaskAuto(ctx context.Context, selector Selector, task AgentTask) (RoutedResult, error) {
	if selector == nil {
		return RoutedResult{}, errors.New("aigo: selector is nil")
	}

	engines := c.EngineNames()
	selection, err := selector.SelectEngine(ctx, task, engines)
	if err != nil {
		return RoutedResult{}, fmt.Errorf("aigo: select engine: %w", err)
	}
	if selection.Engine == "" {
		return RoutedResult{}, errors.New("aigo: selector returned empty engine")
	}

	output, err := c.ExecuteTask(ctx, selection.Engine, task)
	if err != nil {
		return RoutedResult{}, err
	}

	return RoutedResult{
		Engine: selection.Engine,
		Reason: selection.Reason,
		Output: output,
	}, nil
}

// EngineNames returns registered engine names in deterministic order.
func (c *Client) EngineNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.engines))
	for name := range c.engines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildGraph compiles a high-level agent task into the SDK's workflow graph format.
func BuildGraph(task AgentTask) workflow.Graph {
	graph := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs: map[string]any{
				"text": task.Prompt,
			},
		},
	}

	nextID := 2

	if task.Width > 0 || task.Height > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "EmptyLatentImage",
			Inputs: map[string]any{
				"width":  task.Width,
				"height": task.Height,
			},
		}
		nextID++
	}

	if task.NegativePrompt != "" {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "NegativePrompt",
			Inputs: map[string]any{
				"negative_prompt": task.NegativePrompt,
			},
		}
		nextID++
	}

	imageOptions := map[string]any{}
	videoOptions := map[string]any{}

	if task.Size != "" {
		imageOptions["size"] = task.Size
		videoOptions["size"] = task.Size
	}
	if task.Duration > 0 {
		videoOptions["duration"] = task.Duration
	}
	if task.Watermark != nil {
		imageOptions["watermark"] = *task.Watermark
		videoOptions["watermark"] = *task.Watermark
	}

	if len(imageOptions) > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "ImageOptions",
			Inputs:    imageOptions,
		}
		nextID++
	}

	if len(videoOptions) > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "VideoOptions",
			Inputs:    videoOptions,
		}
		nextID++
	}

	for _, ref := range task.References {
		if ref.URL == "" {
			continue
		}

		classType := "LoadImage"
		if ref.Type == ReferenceTypeVideo {
			classType = "LoadVideo"
		}

		graph[nodeID(nextID)] = workflow.Node{
			ClassType: classType,
			Inputs: map[string]any{
				"url": ref.URL,
			},
		}
		nextID++
	}

	if task.Voice != "" || task.LanguageType != "" || task.Instructions != "" || task.OptimizeInstructions != nil {
		audioOpts := map[string]any{}
		if task.Voice != "" {
			audioOpts["voice"] = task.Voice
		}
		if task.LanguageType != "" {
			audioOpts["language_type"] = task.LanguageType
		}
		if task.Instructions != "" {
			audioOpts["instructions"] = task.Instructions
		}
		if task.OptimizeInstructions != nil {
			audioOpts["optimize_instructions"] = *task.OptimizeInstructions
		}
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "AudioOptions",
			Inputs:    audioOpts,
		}
		nextID++
	}

	if task.VoicePrompt != "" && task.PreviewText != "" && task.TargetModel != "" {
		vd := map[string]any{
			"voice_prompt": task.VoicePrompt,
			"preview_text": task.PreviewText,
			"target_model": task.TargetModel,
		}
		if task.VoiceDesignPreferredName != "" {
			vd["preferred_name"] = task.VoiceDesignPreferredName
		}
		if task.VoiceDesignLanguage != "" {
			vd["language"] = task.VoiceDesignLanguage
		}
		if task.VoiceDesignSampleRate > 0 {
			vd["sample_rate"] = task.VoiceDesignSampleRate
		}
		if task.VoiceDesignResponseFormat != "" {
			vd["response_format"] = task.VoiceDesignResponseFormat
		}
		if task.OmitVoiceDesignPreview {
			vd["omit_preview"] = true
		}
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "VoiceDesignInput",
			Inputs:    vd,
		}
		nextID++
	}

	return graph
}

func nodeID(v int) string {
	return fmt.Sprintf("%d", v)
}
