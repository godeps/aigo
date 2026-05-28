package aigo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

// ---------------------------------------------------------------------------
// Additional mock engines
// ---------------------------------------------------------------------------

// stubDryRunnerEngine implements Engine + DryRunner.
type stubDryRunnerEngine struct {
	result    string
	dryResult engine.DryRunResult
}

func (s stubDryRunnerEngine) Execute(_ context.Context, _ workflow.Graph) (engine.Result, error) {
	return engine.Result{Value: s.result}, nil
}

func (s stubDryRunnerEngine) DryRun(_ workflow.Graph) (engine.DryRunResult, error) {
	return s.dryResult, nil
}

// NOTE: stubDescriberEngine (Engine + Describer, no DryRunner) is defined in
// selector_test.go and reused here to avoid duplication.

// stubSyncEngine returns a non-PlainText result (simulating synchronous completion).
type stubSyncEngine struct {
	value string
	kind  engine.OutputKind
}

func (s stubSyncEngine) Execute(_ context.Context, _ workflow.Graph) (engine.Result, error) {
	return engine.Result{Value: s.value, Kind: s.kind}, nil
}

// ---------------------------------------------------------------------------
// Result.String (0%)
// ---------------------------------------------------------------------------

func TestResultString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    Result
		want string
	}{
		{"empty", Result{}, ""},
		{"url value", Result{Value: "https://example.com/img.png"}, "https://example.com/img.png"},
		{"plain text", Result{Value: "hello world"}, "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.r.String(); got != tt.want {
				t.Errorf("Result.String() = %q, want %q", got, tt.want)
			}
			// Verify fmt.Sprint integration.
			if got := fmt.Sprint(tt.r); got != tt.want {
				t.Errorf("fmt.Sprint(Result) = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WithDefaultStore (0%)
// ---------------------------------------------------------------------------

func TestWithDefaultStore(t *testing.T) {
	// Cannot use t.Parallel() because we change the working directory.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	opt, err := WithDefaultStore()
	if err != nil {
		t.Fatalf("WithDefaultStore() error = %v", err)
	}
	if opt == nil {
		t.Fatal("WithDefaultStore() returned nil option")
	}

	client := NewClient(opt)
	if client.store == nil {
		t.Fatal("expected store to be set after applying WithDefaultStore option")
	}

	// Verify the directory was created.
	if _, err := os.Stat(filepath.Join(tmp, ".aigo")); err != nil {
		t.Fatalf(".aigo directory not created: %v", err)
	}
}

// ---------------------------------------------------------------------------
// WithProgress (0%)
// ---------------------------------------------------------------------------

func TestWithProgress(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("stub", stubEngine{result: "ok"})

	var events []ProgressEvent
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	}

	result, err := client.Execute(context.Background(), "stub", graph, WithProgress(func(e ProgressEvent) {
		events = append(events, e)
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "ok" {
		t.Fatalf("result = %q, want %q", result.Value, "ok")
	}

	// Should get at least "submitted" and "completed".
	if len(events) < 2 {
		t.Fatalf("expected >= 2 progress events, got %d", len(events))
	}
	if events[0].Phase != "submitted" {
		t.Errorf("first event phase = %q, want %q", events[0].Phase, "submitted")
	}
	last := events[len(events)-1]
	if last.Phase != "completed" {
		t.Errorf("last event phase = %q, want %q", last.Phase, "completed")
	}
	if last.Elapsed <= 0 {
		t.Error("completed event should have positive Elapsed")
	}
}

// ---------------------------------------------------------------------------
// FallbackError.Error (0%) and FallbackError.Unwrap (0%)
// ---------------------------------------------------------------------------

func TestFallbackError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fe        FallbackError
		wantError string
	}{
		{
			name:      "basic error",
			fe:        FallbackError{Engine: "eng1", Err: errors.New("connection refused")},
			wantError: `engine "eng1": connection refused`,
		},
		{
			name:      "wrapped error",
			fe:        FallbackError{Engine: "flux", Err: fmt.Errorf("timeout: %w", context.DeadlineExceeded)},
			wantError: `engine "flux": timeout: context deadline exceeded`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test Error().
			if got := tt.fe.Error(); got != tt.wantError {
				t.Errorf("Error() = %q, want %q", got, tt.wantError)
			}

			// Test Unwrap().
			unwrapped := tt.fe.Unwrap()
			if unwrapped != tt.fe.Err {
				t.Errorf("Unwrap() = %v, want %v", unwrapped, tt.fe.Err)
			}
		})
	}

	// Test errors.Is propagation through Unwrap.
	t.Run("errors.Is via Unwrap", func(t *testing.T) {
		t.Parallel()
		fe := FallbackError{Engine: "x", Err: fmt.Errorf("wrap: %w", context.DeadlineExceeded)}
		if !errors.Is(fe, context.DeadlineExceeded) {
			t.Error("errors.Is should find DeadlineExceeded via Unwrap")
		}
	})
}

// ---------------------------------------------------------------------------
// EngineCapabilities (0%)
// ---------------------------------------------------------------------------

func TestEngineCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupEngines  func(c *Client)
		query         string
		wantErr       error
		wantPoll      bool
		wantMediaLen  int
	}{
		{
			name:         "not found",
			setupEngines: func(c *Client) {},
			query:        "missing",
			wantErr:      ErrEngineNotFound,
		},
		{
			name: "describer engine",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("img", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{MediaTypes: []string{"image"}, SupportsPoll: true},
				})
			},
			query:        "img",
			wantPoll:     true,
			wantMediaLen: 1,
		},
		{
			name: "non-describer engine returns empty capability",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("plain", stubEngine{result: "ok"})
			},
			query:        "plain",
			wantPoll:     false,
			wantMediaLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient()
			tt.setupEngines(client)

			cap, err := client.EngineCapabilities(tt.query)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cap.SupportsPoll != tt.wantPoll {
				t.Errorf("SupportsPoll = %v, want %v", cap.SupportsPoll, tt.wantPoll)
			}
			if len(cap.MediaTypes) != tt.wantMediaLen {
				t.Errorf("len(MediaTypes) = %d, want %d", len(cap.MediaTypes), tt.wantMediaLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DryRun (0%)
// ---------------------------------------------------------------------------

func TestDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupEngines func(c *Client)
		engineName   string
		task         AgentTask
		wantErr      error
		wantPoll     bool
	}{
		{
			name:         "engine not found",
			setupEngines: func(c *Client) {},
			engineName:   "missing",
			task:         AgentTask{Prompt: "test"},
			wantErr:      ErrEngineNotFound,
		},
		{
			name: "DryRunner engine",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("dr", stubDryRunnerEngine{
					result:    "ok",
					dryResult: engine.DryRunResult{WillPoll: true, EstimatedTime: "30s"},
				})
			},
			engineName: "dr",
			task:       AgentTask{Prompt: "test"},
			wantPoll:   true,
		},
		{
			name: "describer-only engine falls back to capability inference",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("desc", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{SupportsPoll: true},
				})
			},
			engineName: "desc",
			task:       AgentTask{Prompt: "test"},
			wantPoll:   true,
		},
		{
			name: "plain engine returns zero DryRunResult",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("plain", stubEngine{result: "ok"})
			},
			engineName: "plain",
			task:       AgentTask{Prompt: "test"},
			wantPoll:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient()
			tt.setupEngines(client)

			dr, err := client.DryRun(tt.engineName, tt.task)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dr.WillPoll != tt.wantPoll {
				t.Errorf("WillPoll = %v, want %v", dr.WillPoll, tt.wantPoll)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AvailableFor (50%)
// ---------------------------------------------------------------------------

func TestAvailableFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupEngines func(c *Client)
		mediaType    string
		wantNames    []string
	}{
		{
			name: "non-describer engines are included as assumed capable",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("plain", stubEngine{result: "ok"})
			},
			mediaType: "image",
			wantNames: []string{"plain"},
		},
		{
			name: "describer with matching media type is included",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("img", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{MediaTypes: []string{"image", "video"}},
				})
			},
			mediaType: "image",
			wantNames: []string{"img"},
		},
		{
			name: "describer with non-matching media type is excluded",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("vid", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{MediaTypes: []string{"video"}},
				})
			},
			mediaType: "image",
			wantNames: nil,
		},
		{
			name: "mixed engines",
			setupEngines: func(c *Client) {
				_ = c.RegisterEngine("img", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{MediaTypes: []string{"image"}},
				})
				_ = c.RegisterEngine("vid", stubDescriberEngine{
					result: "ok",
					cap:    engine.Capability{MediaTypes: []string{"video"}},
				})
				_ = c.RegisterEngine("plain", stubEngine{result: "ok"})
			},
			mediaType: "image",
			wantNames: []string{"img", "plain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient()
			tt.setupEngines(client)

			got := client.AvailableFor(tt.mediaType)
			if len(got) != len(tt.wantNames) {
				t.Fatalf("AvailableFor(%q) = %v, want %v", tt.mediaType, got, tt.wantNames)
			}
			for i, name := range tt.wantNames {
				if got[i] != name {
					t.Errorf("AvailableFor(%q)[%d] = %q, want %q", tt.mediaType, i, got[i], name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RegisterEngine edge cases (80%)
// ---------------------------------------------------------------------------

func TestRegisterEngine_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		eName   string
		engine  engine.Engine
		wantErr error
	}{
		{
			name:    "empty name",
			eName:   "",
			engine:  stubEngine{result: "ok"},
			wantErr: ErrEngineNameEmpty,
		},
		{
			name:    "nil engine",
			eName:   "test",
			engine:  nil,
			wantErr: ErrEngineNil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient()
			err := client.RegisterEngine(tt.eName, tt.engine)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("RegisterEngine() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EnableEngine not-found (untested path)
// ---------------------------------------------------------------------------

func TestEnableEngine_NotFound(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.EnableEngine("missing")
	if !errors.Is(err, ErrEngineNotFound) {
		t.Fatalf("EnableEngine() error = %v, want %v", err, ErrEngineNotFound)
	}
}

// ---------------------------------------------------------------------------
// ExecuteTaskAutoWithFallback - plain selector path (56%)
// ---------------------------------------------------------------------------

func TestExecuteTaskAutoWithFallback_PlainSelector(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("bad", stubEngine{err: errors.New("fail")})
	_ = client.RegisterEngine("good", stubEngine{result: "ok"})

	sel := &stubSelector{
		result: Selection{Engine: "bad", Reason: "preferred"},
	}

	result, err := client.ExecuteTaskAutoWithFallback(context.Background(), sel, AgentTask{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Engine != "good" {
		t.Fatalf("engine = %q, want %q", result.Engine, "good")
	}
	if result.Output.Value != "ok" {
		t.Fatalf("output = %q, want %q", result.Output.Value, "ok")
	}
}

func TestExecuteTaskAutoWithFallback_NilSelector(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_, err := client.ExecuteTaskAutoWithFallback(context.Background(), nil, AgentTask{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error for nil selector")
	}
}

func TestExecuteTaskAutoWithFallback_SelectorError(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("e1", stubEngine{result: "ok"})

	sel := &stubSelector{err: errors.New("selector failed")}
	_, err := client.ExecuteTaskAutoWithFallback(context.Background(), sel, AgentTask{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error when selector fails")
	}
}

// ---------------------------------------------------------------------------
// ExecuteTaskAuto edge cases
// ---------------------------------------------------------------------------

func TestExecuteTaskAuto_NilSelector(t *testing.T) {
	t.Parallel()
	client := NewClient()
	_, err := client.ExecuteTaskAuto(context.Background(), nil, AgentTask{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for nil selector")
	}
}

func TestExecuteTaskAuto_EmptySelection(t *testing.T) {
	t.Parallel()
	client := NewClient()
	_ = client.RegisterEngine("e1", stubEngine{result: "ok"})

	sel := &stubSelector{result: Selection{Engine: ""}}
	_, err := client.ExecuteTaskAuto(context.Background(), sel, AgentTask{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for empty engine selection")
	}
}

// ---------------------------------------------------------------------------
// Submit - synchronous completion path (66.7%)
// ---------------------------------------------------------------------------

func TestSubmit_SynchronousCompletion(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	// Engine returns a URL (non-PlainText) = synchronous completion.
	_ = client.RegisterEngine("sync", stubSyncEngine{value: "https://cdn.example.com/image.png", kind: engine.OutputURL})

	graph := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "draw cat"}}}
	taskID, err := client.Submit(context.Background(), "sync", graph)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// Synchronous completion returns empty task ID.
	if taskID != "" {
		t.Fatalf("expected empty task ID for synchronous completion, got %q", taskID)
	}

	// Verify the record was stored as completed.
	all, err := store.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 record, got %d", len(all))
	}
	if all[0].Status != TaskStatusCompleted {
		t.Errorf("status = %q, want %q", all[0].Status, TaskStatusCompleted)
	}
	if all[0].ResultVal != "https://cdn.example.com/image.png" {
		t.Errorf("result = %q", all[0].ResultVal)
	}
}

func TestSubmit_ExecuteError(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	_ = client.RegisterEngine("bad", stubEngine{err: errors.New("engine failed")})

	graph := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}}}
	_, err := client.Submit(context.Background(), "bad", graph)
	if err == nil {
		t.Fatal("expected error when engine fails")
	}
}

// ---------------------------------------------------------------------------
// Resume - failed task and engine-not-found paths (75.9%)
// ---------------------------------------------------------------------------

func TestResume_FailedTask(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	rec := TaskRecord{
		ID:         "failed-001",
		EngineName: "flux",
		Status:     TaskStatusFailed,
		ErrMsg:     "remote processing error",
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := client.Resume(context.Background(), "failed-001")
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if got := err.Error(); got == "" {
		t.Fatal("error message should not be empty")
	}
}

func TestResume_EngineNotFound(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	rec := TaskRecord{
		ID:         "orphan-001",
		EngineName: "deleted-engine",
		RemoteID:   "remote-xyz",
		Status:     TaskStatusPending,
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := client.Resume(context.Background(), "orphan-001")
	if !errors.Is(err, ErrEngineNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrEngineNotFound)
	}
}

func TestResume_ResumeFails(t *testing.T) {
	t.Parallel()
	client, store := newTestClient(t)

	eng := &stubResumerEngine{
		submitValue: "remote-1",
		resumeValue: "",
		err:         errors.New("remote poll failed"),
	}
	if err := client.RegisterEngine("flaky", eng); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}

	rec := TaskRecord{
		ID:         "flaky-001",
		EngineName: "flaky",
		RemoteID:   "remote-1",
		Status:     TaskStatusPending,
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := client.Resume(context.Background(), "flaky-001")
	if err == nil {
		t.Fatal("expected error when resume fails")
	}

	// Verify task was marked as failed in the store.
	updated, err := store.Load("flaky-001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.Status != TaskStatusFailed {
		t.Errorf("status = %q, want %q", updated.Status, TaskStatusFailed)
	}
}

// ---------------------------------------------------------------------------
// BuildGraph - uncovered branches (55.4%)
// ---------------------------------------------------------------------------

func TestBuildGraph_TTSOptions(t *testing.T) {
	t.Parallel()

	optTrue := true
	g := BuildGraph(AgentTask{
		Prompt: "say hello",
		TTS: &TTSOptions{
			Voice:                "alloy",
			LanguageType:         "en",
			Instructions:         "speak slowly",
			OptimizeInstructions: &optTrue,
		},
	})

	// Find the AudioOptions node.
	var found bool
	for _, node := range g {
		if node.ClassType == "AudioOptions" {
			found = true
			if v, _ := node.StringInput("voice"); v != "alloy" {
				t.Errorf("voice = %q, want %q", v, "alloy")
			}
			if v, _ := node.StringInput("language_type"); v != "en" {
				t.Errorf("language_type = %q", v)
			}
			if v, _ := node.StringInput("instructions"); v != "speak slowly" {
				t.Errorf("instructions = %q", v)
			}
			if v, ok := node.Inputs["optimize_instructions"]; !ok || v != true {
				t.Errorf("optimize_instructions = %v", v)
			}
		}
	}
	if !found {
		t.Fatalf("AudioOptions node not found in graph: %+v", g)
	}
}

func TestBuildGraph_MusicOptions(t *testing.T) {
	t.Parallel()

	instrumental := true
	optimizer := false
	g := BuildGraph(AgentTask{
		Prompt: "upbeat jazz",
		Music: &MusicOptions{
			Lyrics:          "la la la",
			IsInstrumental:  &instrumental,
			LyricsOptimizer: &optimizer,
			OutputFormat:    "url",
			SampleRate:      44100,
			Bitrate:         320,
			Format:          "mp3",
		},
	})

	var found bool
	for _, node := range g {
		if node.ClassType == "MusicOptions" {
			found = true
			if v, _ := node.StringInput("lyrics"); v != "la la la" {
				t.Errorf("lyrics = %q", v)
			}
			if v, ok := node.Inputs["is_instrumental"]; !ok || v != true {
				t.Errorf("is_instrumental = %v", v)
			}
			if v, ok := node.Inputs["lyrics_optimizer"]; !ok || v != false {
				t.Errorf("lyrics_optimizer = %v", v)
			}
			if v, _ := node.StringInput("output_format"); v != "url" {
				t.Errorf("output_format = %q", v)
			}
			if v, ok := node.Inputs["sample_rate"]; !ok || v != 44100 {
				t.Errorf("sample_rate = %v", v)
			}
			if v, ok := node.Inputs["bitrate"]; !ok || v != 320 {
				t.Errorf("bitrate = %v", v)
			}
			if v, _ := node.StringInput("format"); v != "mp3" {
				t.Errorf("format = %q", v)
			}
		}
	}
	if !found {
		t.Fatalf("MusicOptions node not found in graph: %+v", g)
	}
}

func TestBuildGraph_VoiceDesignOptions(t *testing.T) {
	t.Parallel()

	g := BuildGraph(AgentTask{
		Prompt: "design voice",
		VoiceDesign: &VoiceDesignOptions{
			VoicePrompt:    "warm and friendly",
			PreviewText:    "Hello, world!",
			TargetModel:    "model-v1",
			PreferredName:  "MyVoice",
			Language:       "en",
			SampleRate:     22050,
			ResponseFormat: "wav",
			OmitPreview:    true,
		},
	})

	var found bool
	for _, node := range g {
		if node.ClassType == "VoiceDesignInput" {
			found = true
			if v, _ := node.StringInput("voice_prompt"); v != "warm and friendly" {
				t.Errorf("voice_prompt = %q", v)
			}
			if v, _ := node.StringInput("preview_text"); v != "Hello, world!" {
				t.Errorf("preview_text = %q", v)
			}
			if v, _ := node.StringInput("target_model"); v != "model-v1" {
				t.Errorf("target_model = %q", v)
			}
			if v, _ := node.StringInput("preferred_name"); v != "MyVoice" {
				t.Errorf("preferred_name = %q", v)
			}
			if v, _ := node.StringInput("language"); v != "en" {
				t.Errorf("language = %q", v)
			}
			if v, ok := node.Inputs["sample_rate"]; !ok || v != 22050 {
				t.Errorf("sample_rate = %v", v)
			}
			if v, _ := node.StringInput("response_format"); v != "wav" {
				t.Errorf("response_format = %q", v)
			}
			if v, ok := node.Inputs["omit_preview"]; !ok || v != true {
				t.Errorf("omit_preview = %v", v)
			}
		}
	}
	if !found {
		t.Fatalf("VoiceDesignInput node not found in graph: %+v", g)
	}
}

func TestBuildGraph_VoiceDesignIncomplete(t *testing.T) {
	t.Parallel()

	// VoiceDesign with missing required fields should NOT produce a node.
	g := BuildGraph(AgentTask{
		Prompt: "test",
		VoiceDesign: &VoiceDesignOptions{
			VoicePrompt: "warm",
			// PreviewText and TargetModel are empty.
		},
	})

	for _, node := range g {
		if node.ClassType == "VoiceDesignInput" {
			t.Fatal("VoiceDesignInput should not be generated with missing required fields")
		}
	}
}

func TestBuildGraph_StructuredVideoExtras(t *testing.T) {
	t.Parallel()

	audioOn := true
	g := BuildGraph(AgentTask{
		Prompt: "test",
		Structured: &AgentTaskStructured{
			VideoAspectRatio: "16:9",
			VideoResolution:  "1080P",
			VideoAudio:       &audioOn,
			VideoDuration:    5,
		},
	})

	var found bool
	for _, node := range g {
		if node.ClassType == "VideoOptions" {
			found = true
			if v, _ := node.StringInput("aspect_ratio"); v != "16:9" {
				t.Errorf("aspect_ratio = %q", v)
			}
			if v, _ := node.StringInput("resolution"); v != "1080P" {
				t.Errorf("resolution = %q", v)
			}
			if v, ok := node.Inputs["audio"]; !ok || v != true {
				t.Errorf("audio = %v", v)
			}
			if v, _ := node.IntInput("duration"); v != 5 {
				t.Errorf("duration = %d, want 5", v)
			}
		}
	}
	if !found {
		t.Fatalf("VideoOptions node not found in graph: %+v", g)
	}
}

func TestBuildGraph_EmptyRefURL(t *testing.T) {
	t.Parallel()

	g := BuildGraph(AgentTask{
		Prompt: "test",
		References: []ReferenceAsset{
			{Type: ReferenceTypeImage, URL: ""}, // empty URL, should be skipped
			{Type: ReferenceTypeImage, URL: "https://example.com/img.png"},
		},
	})

	var loadCount int
	for _, node := range g {
		if node.ClassType == "LoadImage" || node.ClassType == "LoadVideo" {
			loadCount++
		}
	}
	if loadCount != 1 {
		t.Fatalf("expected 1 LoadImage node (empty URL skipped), got %d", loadCount)
	}
}

func TestBuildGraph_PromptOnly(t *testing.T) {
	t.Parallel()

	g := BuildGraph(AgentTask{Prompt: "minimal"})
	if len(g) != 1 {
		t.Fatalf("expected 1 node for prompt-only task, got %d", len(g))
	}
	node, ok := g["1"]
	if !ok || node.ClassType != "CLIPTextEncode" {
		t.Fatalf("expected CLIPTextEncode node, got %+v", g)
	}
	if v, _ := node.StringInput("text"); v != "minimal" {
		t.Errorf("text = %q, want %q", v, "minimal")
	}
}

func TestBuildGraph_TTSEmpty(t *testing.T) {
	t.Parallel()

	// TTS with all empty fields should NOT generate an AudioOptions node.
	g := BuildGraph(AgentTask{
		Prompt: "test",
		TTS:    &TTSOptions{},
	})
	for _, node := range g {
		if node.ClassType == "AudioOptions" {
			t.Fatal("AudioOptions should not be generated when all TTS fields are empty")
		}
	}
}

func TestBuildGraph_MusicEmpty(t *testing.T) {
	t.Parallel()

	// Music with all empty fields should NOT generate a MusicOptions node.
	g := BuildGraph(AgentTask{
		Prompt: "test",
		Music:  &MusicOptions{},
	})
	for _, node := range g {
		if node.ClassType == "MusicOptions" {
			t.Fatal("MusicOptions should not be generated when all Music fields are empty")
		}
	}
}

// ---------------------------------------------------------------------------
// Execute - graph validation error
// ---------------------------------------------------------------------------

func TestExecute_InvalidGraph(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("stub", stubEngine{result: "ok"})

	// Empty graph should fail validation.
	_, err := client.Execute(context.Background(), "stub", workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

// ---------------------------------------------------------------------------
// RecoverPending - no store
// ---------------------------------------------------------------------------

func TestRecoverPending_NoStore(t *testing.T) {
	t.Parallel()
	client := NewClient()
	_, err := client.RecoverPending()
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("error = %v, want %v", err, ErrStoreNotConfigured)
	}
}

// ---------------------------------------------------------------------------
// ExecuteWithFallback - context cancellation
// ---------------------------------------------------------------------------

func TestExecuteWithFallback_ContextCancelled(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("slow", stubEngine{result: "ok"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.ExecuteWithFallback(ctx, []string{"slow"}, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
