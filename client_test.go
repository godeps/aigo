package aigo

import (
	"context"
	"errors"
	"testing"

	"github.com/godeps/aigo/workflow"
)

type stubEngine struct {
	result string
	err    error
}

func (s stubEngine) Execute(context.Context, workflow.Graph) (string, error) {
	return s.result, s.err
}

func TestClientRegisterAndExecute(t *testing.T) {
	t.Parallel()

	client := NewClient()
	err := client.RegisterEngine("stub", stubEngine{result: "ok"})
	if err != nil {
		t.Fatalf("RegisterEngine() error = %v", err)
	}

	got, err := client.Execute(context.Background(), "stub", workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got != "ok" {
		t.Fatalf("Execute() = %q, want %q", got, "ok")
	}
}

func TestClientRegisterRejectsDuplicate(t *testing.T) {
	t.Parallel()

	client := NewClient()
	if err := client.RegisterEngine("stub", stubEngine{}); err != nil {
		t.Fatalf("RegisterEngine() error = %v", err)
	}

	err := client.RegisterEngine("stub", stubEngine{})
	if !errors.Is(err, ErrEngineExists) {
		t.Fatalf("RegisterEngine() error = %v, want %v", err, ErrEngineExists)
	}
}

func TestClientExecuteUnknownEngine(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_, err := client.Execute(context.Background(), "missing", workflow.Graph{})
	if !errors.Is(err, ErrEngineNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrEngineNotFound)
	}
}

type captureEngine struct {
	graph workflow.Graph
}

func (c *captureEngine) Execute(_ context.Context, graph workflow.Graph) (string, error) {
	c.graph = graph
	return "captured", nil
}

func TestClientExecutePromptBuildsGraph(t *testing.T) {
	t.Parallel()

	client := NewClient()
	engine := &captureEngine{}
	if err := client.RegisterEngine("capture", engine); err != nil {
		t.Fatalf("RegisterEngine() error = %v", err)
	}

	got, err := client.ExecutePrompt(context.Background(), "capture", "draw a lighthouse in a storm")
	if err != nil {
		t.Fatalf("ExecutePrompt() error = %v", err)
	}
	if got != "captured" {
		t.Fatalf("ExecutePrompt() = %q", got)
	}

	node, ok := engine.graph["1"]
	if !ok || node.ClassType != "CLIPTextEncode" {
		t.Fatalf("graph = %#v", engine.graph)
	}
	if text, _ := node.StringInput("text"); text != "draw a lighthouse in a storm" {
		t.Fatalf("prompt = %q", text)
	}
}

func TestClientExecuteTaskBuildsMediaGraph(t *testing.T) {
	t.Parallel()

	client := NewClient()
	engine := &captureEngine{}
	if err := client.RegisterEngine("capture", engine); err != nil {
		t.Fatalf("RegisterEngine() error = %v", err)
	}

	request := AgentTask{
		Prompt:         "turn this scene into a short ad",
		NegativePrompt: "blur",
		Width:          1280,
		Height:         720,
		Duration:       2,
		Size:           "1280*720",
		Watermark:      boolPtr(false),
		References: []ReferenceAsset{
			{Type: ReferenceTypeImage, URL: "https://example.com/ref.png"},
			{Type: ReferenceTypeVideo, URL: "https://example.com/input.mp4"},
		},
	}

	got, err := client.ExecuteTask(context.Background(), "capture", request)
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}
	if got != "captured" {
		t.Fatalf("ExecuteTask() = %q", got)
	}

	if _, ok := engine.graph["1"]; !ok {
		t.Fatalf("graph missing prompt node: %#v", engine.graph)
	}
	if node, ok := engine.graph["2"]; !ok || node.ClassType != "EmptyLatentImage" {
		t.Fatalf("graph missing latent node: %#v", engine.graph)
	}
	if node, ok := engine.graph["3"]; !ok || node.ClassType != "NegativePrompt" {
		t.Fatalf("graph missing negative prompt node: %#v", engine.graph)
	}
	if node, ok := engine.graph["4"]; !ok || node.ClassType != "ImageOptions" {
		t.Fatalf("graph missing image options node: %#v", engine.graph)
	}
	if node, ok := engine.graph["5"]; !ok || node.ClassType != "VideoOptions" {
		t.Fatalf("graph missing video options node: %#v", engine.graph)
	}
	if node, ok := engine.graph["6"]; !ok || node.ClassType != "LoadImage" {
		t.Fatalf("graph missing image ref node: %#v", engine.graph)
	}
	if node, ok := engine.graph["7"]; !ok || node.ClassType != "LoadVideo" {
		t.Fatalf("graph missing video ref node: %#v", engine.graph)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

type stubSelector struct {
	gotTask    AgentTask
	gotEngines []string
	result     Selection
	err        error
}

func (s *stubSelector) SelectEngine(_ context.Context, task AgentTask, engines []string) (Selection, error) {
	s.gotTask = task
	s.gotEngines = append([]string(nil), engines...)
	return s.result, s.err
}

func TestClientExecuteTaskAutoRoutesWithSelector(t *testing.T) {
	t.Parallel()

	client := NewClient()
	if err := client.RegisterEngine("img", stubEngine{result: "image-url"}); err != nil {
		t.Fatalf("RegisterEngine(img) error = %v", err)
	}
	if err := client.RegisterEngine("video", stubEngine{result: "video-url"}); err != nil {
		t.Fatalf("RegisterEngine(video) error = %v", err)
	}

	selector := &stubSelector{
		result: Selection{
			Engine: "video",
			Reason: "the task asks for a short video",
		},
	}

	result, err := client.ExecuteTaskAuto(context.Background(), selector, AgentTask{
		Prompt:   "make a 2 second ad video",
		Duration: 2,
	})
	if err != nil {
		t.Fatalf("ExecuteTaskAuto() error = %v", err)
	}
	if result.Engine != "video" || result.Output != "video-url" {
		t.Fatalf("ExecuteTaskAuto() = %#v", result)
	}
	if result.Reason != "the task asks for a short video" {
		t.Fatalf("Reason = %q", result.Reason)
	}
	if selector.gotTask.Duration != 2 {
		t.Fatalf("selector task = %#v", selector.gotTask)
	}
	if len(selector.gotEngines) != 2 || selector.gotEngines[0] != "img" || selector.gotEngines[1] != "video" {
		t.Fatalf("selector engines = %#v", selector.gotEngines)
	}
}

func TestClientExecutePromptAuto(t *testing.T) {
	t.Parallel()

	client := NewClient()
	if err := client.RegisterEngine("img", stubEngine{result: "image-url"}); err != nil {
		t.Fatalf("RegisterEngine(img) error = %v", err)
	}

	selector := &stubSelector{
		result: Selection{Engine: "img"},
	}

	result, err := client.ExecutePromptAuto(context.Background(), selector, "draw a castle")
	if err != nil {
		t.Fatalf("ExecutePromptAuto() error = %v", err)
	}
	if result.Output != "image-url" || result.Engine != "img" {
		t.Fatalf("ExecutePromptAuto() = %#v", result)
	}
	if selector.gotTask.Prompt != "draw a castle" {
		t.Fatalf("selector prompt = %#v", selector.gotTask)
	}
}
