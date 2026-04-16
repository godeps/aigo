package aigo

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

type stubEngine struct {
	result string
	err    error
}

func (s stubEngine) Execute(context.Context, workflow.Graph) (engine.Result, error) {
	return engine.Result{Value: s.result}, s.err
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

	if got.Value != "ok" {
		t.Fatalf("Execute() = %q, want %q", got.Value, "ok")
	}
	if got.Engine != "stub" {
		t.Fatalf("Execute().Engine = %q, want %q", got.Engine, "stub")
	}
	if got.Elapsed <= 0 {
		t.Fatal("Execute().Elapsed should be positive")
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

func (c *captureEngine) Execute(_ context.Context, graph workflow.Graph) (engine.Result, error) {
	c.graph = graph
	return engine.Result{Value: "captured"}, nil
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
	if got.Value != "captured" {
		t.Fatalf("ExecutePrompt() = %q", got.Value)
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
	if got.Value != "captured" {
		t.Fatalf("ExecuteTask() = %q", got.Value)
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
	if result.Engine != "video" || result.Output.Value != "video-url" {
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

func TestBuildGraphStructuredOverrides(t *testing.T) {
	t.Parallel()
	wmTrue := true
	wmFalse := false
	g := BuildGraph(AgentTask{
		Prompt:    "p",
		Size:      "1024x1024",
		Duration:  1,
		Watermark: &wmTrue,
		Structured: &AgentTaskStructured{
			ImageSize:       "512x512",
			ImageWatermark:  &wmFalse,
			VideoDuration:   8,
			VideoSize:       "1920x1080",
			VideoWatermark:  &wmFalse,
		},
	})
	img, ok := g["2"]
	if !ok || img.ClassType != "ImageOptions" {
		t.Fatalf("image options node: %#v", g)
	}
	if s, _ := img.StringInput("size"); s != "512x512" {
		t.Fatalf("image size %q", s)
	}
	vid, ok := g["3"]
	if !ok || vid.ClassType != "VideoOptions" {
		t.Fatalf("video options: %#v", g)
	}
	if s, _ := vid.StringInput("size"); s != "1920x1080" {
		t.Fatalf("video size %q", s)
	}
	if d, _ := vid.IntInput("duration"); d != 8 {
		t.Fatalf("video duration %d", d)
	}
}

func TestInterpretOutputKind(t *testing.T) {
	t.Parallel()
	if InterpretOutputKind("https://x/a.png") != OutputURL {
		t.Fatal("url")
	}
	if InterpretOutputKind("data:image/png;base64,QQ==") != OutputDataURI {
		t.Fatal("data uri")
	}
	if InterpretOutputKind(`{"a":1}`) != OutputJSON {
		t.Fatal("json")
	}
	if InterpretOutputKind("hello") != OutputPlainText {
		t.Fatal("plain")
	}
}

func TestExecuteWithFallback_FirstSuccess(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("a", stubEngine{result: "from-a"})
	_ = client.RegisterEngine("b", stubEngine{result: "from-b"})

	r, err := client.ExecuteWithFallback(context.Background(), []string{"a", "b"}, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Engine != "a" || r.Output.Value != "from-a" {
		t.Fatalf("got engine=%q output=%q", r.Engine, r.Output.Value)
	}
	if len(r.Skipped) != 0 {
		t.Fatalf("expected 0 skipped, got %d", len(r.Skipped))
	}
}

func TestExecuteWithFallback_SkipsFailures(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("bad", stubEngine{err: errors.New("boom")})
	_ = client.RegisterEngine("good", stubEngine{result: "ok"})

	r, err := client.ExecuteWithFallback(context.Background(), []string{"bad", "good"}, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Engine != "good" || r.Output.Value != "ok" {
		t.Fatalf("got engine=%q output=%q", r.Engine, r.Output.Value)
	}
	if len(r.Skipped) != 1 || r.Skipped[0].Engine != "bad" {
		t.Fatalf("skipped = %+v", r.Skipped)
	}
}

func TestExecuteWithFallback_AllFail(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("a", stubEngine{err: errors.New("fail-a")})
	_ = client.RegisterEngine("b", stubEngine{err: errors.New("fail-b")})

	_, err := client.ExecuteWithFallback(context.Background(), []string{"a", "b"}, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecuteWithFallback_EmptyList(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_, err := client.ExecuteWithFallback(context.Background(), nil, workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestExecuteTaskWithFallback(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("e1", stubEngine{err: errors.New("nope")})
	_ = client.RegisterEngine("e2", stubEngine{result: "yes"})

	r, err := client.ExecuteTaskWithFallback(context.Background(), []string{"e1", "e2"}, AgentTask{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Engine != "e2" {
		t.Fatalf("engine = %q", r.Engine)
	}
}

func TestExecuteAsync(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("s", stubEngine{result: "async-ok"})

	ch := client.ExecuteAsync(context.Background(), "s", workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	ar := <-ch
	if ar.Err != nil {
		t.Fatalf("unexpected error: %v", ar.Err)
	}
	if ar.Result.Value != "async-ok" {
		t.Fatalf("got %q", ar.Result.Value)
	}
}

func TestExecuteAsync_Error(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("bad", stubEngine{err: errors.New("fail")})

	ch := client.ExecuteAsync(context.Background(), "bad", workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	ar := <-ch
	if ar.Err == nil {
		t.Fatal("expected error")
	}
}

func TestExecuteAsync_ContextCancel(t *testing.T) {
	t.Parallel()

	client := NewClient()
	// no engine registered — Execute will fail with ErrEngineNotFound
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := client.ExecuteAsync(ctx, "missing", workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	ar := <-ch
	if ar.Err == nil {
		t.Fatal("expected error")
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
	if result.Output.Value != "image-url" || result.Engine != "img" {
		t.Fatalf("ExecutePromptAuto() = %#v", result)
	}
	if selector.gotTask.Prompt != "draw a castle" {
		t.Fatalf("selector prompt = %#v", selector.gotTask)
	}
}

// --- Submit / Resume tests ---

// stubResumerEngine returns a task ID on Execute, and a result URL on Resume.
type stubResumerEngine struct {
	submitValue string
	resumeValue string
	resumeKind  engine.OutputKind
	err         error
}

func (s *stubResumerEngine) Execute(_ context.Context, _ workflow.Graph) (engine.Result, error) {
	return engine.Result{Value: s.submitValue, Kind: engine.OutputPlainText}, s.err
}

func (s *stubResumerEngine) Resume(_ context.Context, _ string) (engine.Result, error) {
	return engine.Result{Value: s.resumeValue, Kind: s.resumeKind}, s.err
}

func newTestClient(t *testing.T) (*Client, *FileTaskStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewFileTaskStore(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Fatalf("NewFileTaskStore: %v", err)
	}
	client := NewClient(WithStore(store))
	return client, store
}

func TestClientSubmitAndResume(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	eng := &stubResumerEngine{
		submitValue: "remote-task-123",
		resumeValue: "https://cdn.example.com/video.mp4",
		resumeKind:  engine.OutputURL,
	}
	if err := client.RegisterEngine("runway", eng); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}

	graph := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}}}
	taskID, err := client.Submit(context.Background(), "runway", graph)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if taskID == "" {
		t.Fatal("expected non-empty task ID for async submit")
	}

	result, err := client.Resume(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result.Value != "https://cdn.example.com/video.mp4" || result.Kind != OutputURL {
		t.Errorf("Resume result = %+v", result)
	}
}

func TestClientRecoverPending(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	eng := &stubResumerEngine{submitValue: "remote-1"}
	if err := client.RegisterEngine("flux", eng); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}

	graph := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}}}
	_, err := client.Submit(context.Background(), "flux", graph)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	eng.submitValue = "remote-2"
	_, err = client.Submit(context.Background(), "flux", graph)
	if err != nil {
		t.Fatalf("Submit 2: %v", err)
	}

	pending, err := client.RecoverPending()
	if err != nil {
		t.Fatalf("RecoverPending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestClientResume_AlreadyCompleted(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	rec := TaskRecord{
		ID:         "completed-001",
		EngineName: "flux",
		Status:     TaskStatusCompleted,
		ResultVal:  "https://example.com/img.png",
		ResultKind: OutputURL,
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	result, err := client.Resume(context.Background(), "completed-001")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result.Value != "https://example.com/img.png" {
		t.Errorf("expected cached result, got: %+v", result)
	}
}

func TestClientResume_NoStore(t *testing.T) {
	t.Parallel()
	client := NewClient() // no store
	_, err := client.Resume(context.Background(), "any-id")
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Errorf("expected ErrStoreNotConfigured, got: %v", err)
	}
}

func TestClientSubmit_NoStore(t *testing.T) {
	t.Parallel()
	client := NewClient()
	graph := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}}}
	_, err := client.Submit(context.Background(), "any", graph)
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Errorf("expected ErrStoreNotConfigured, got: %v", err)
	}
}

func TestClientResume_EngineNotResumer(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	// Register a plain stubEngine (no Resumer interface).
	if err := client.RegisterEngine("plain", stubEngine{result: "ok"}); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}

	rec := TaskRecord{
		ID:         "plain-001",
		EngineName: "plain",
		RemoteID:   "remote-abc",
		Status:     TaskStatusPending,
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := client.Resume(context.Background(), "plain-001")
	if !errors.Is(err, ErrResumeNotSupported) {
		t.Errorf("expected ErrResumeNotSupported, got: %v", err)
	}
}
