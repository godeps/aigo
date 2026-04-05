package aigo

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/godeps/aigo/pkg/engine"
	"github.com/godeps/aigo/pkg/workflow"
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

	return graph
}

func nodeID(v int) string {
	return fmt.Sprintf("%d", v)
}
