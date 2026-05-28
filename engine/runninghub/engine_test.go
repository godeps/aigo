package runninghub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

// submitAndPollServer creates a test server that handles submit + poll sequences.
// submitResp is the JSON body for the POST /{endpoint} request.
// pollResps are served in order for POST /query requests.
func submitAndPollServer(t *testing.T, endpoint, submitResp string, pollResps []string) *httptest.Server {
	t.Helper()
	var pollIdx int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/"+endpoint {
			w.Write([]byte(submitResp))
			return
		}
		if r.URL.Path == "/query" {
			idx := int(atomic.AddInt32(&pollIdx, 1)) - 1
			if idx >= len(pollResps) {
				idx = len(pollResps) - 1
			}
			w.Write([]byte(pollResps[idx]))
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
}

func TestExecute_Success(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/video",
		`{"taskId":"task-001"}`,
		[]string{
			`{"status":"RUNNING"}`,
			`{"status":"SUCCESS","results":[{"url":"https://cdn.runninghub.cn/output.mp4","text":""}]}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/video",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset timelapse"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.runninghub.cn/output.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecute_NoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"taskId":"task-async-42"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/image",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a forest at dawn"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "task-async-42" {
		t.Fatalf("Value = %q, want task-async-42", result.Value)
	}
}

func TestExecute_PollFailure(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/video",
		`{"taskId":"task-fail"}`,
		[]string{
			`{"status":"FAILED","errorCode":"E001","errorMessage":"model overload"}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/video",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "some prompt"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if got := err.Error(); got == "" {
		t.Fatal("error message is empty")
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"status":"QUEUED"}`))
			return
		}
		w.Write([]byte(`{"status":"SUCCESS","results":[{"url":"https://cdn.runninghub.cn/resumed.mp4"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		Endpoint:     "generate/video",
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "task-existing-99")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.runninghub.cn/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestBuildPayload(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.Write([]byte(`{"taskId":"t1"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Endpoint: "generate/image",
		Model:    "flux-dev",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "beautiful mountain"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/input.jpg"}},
		"3": {ClassType: "NegativePrompt", Inputs: map[string]any{"text": "blurry, ugly"}},
		"4": {ClassType: "Option", Inputs: map[string]any{"size": "1024x1024", "duration": 5}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if captured["prompt"] != "beautiful mountain" {
		t.Errorf("prompt = %v", captured["prompt"])
	}
	if captured["model"] != "flux-dev" {
		t.Errorf("model = %v", captured["model"])
	}
	if captured["imageUrl"] != "https://example.com/input.jpg" {
		t.Errorf("imageUrl = %v", captured["imageUrl"])
	}
	if captured["negative_prompt"] != "blurry, ugly" {
		t.Errorf("negative_prompt = %v", captured["negative_prompt"])
	}
	if captured["size"] != "1024x1024" {
		t.Errorf("size = %v", captured["size"])
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) == 0 {
		t.Fatal("ConfigSchema() returned empty slice")
	}
	keys := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f.Key == "" {
			t.Error("ConfigField has empty Key")
		}
		keys[f.Key] = true
	}
	for _, required := range []string{"apiKey", "endpoint"} {
		if !keys[required] {
			t.Errorf("ConfigSchema missing required field %q", required)
		}
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		model        string
		waitResult   bool
		wantPoll     bool
		wantSync     bool
		wantModelLen int
	}{
		{
			name:         "with wait and model",
			model:        "flux-dev",
			waitResult:   true,
			wantPoll:     true,
			wantSync:     false,
			wantModelLen: 1,
		},
		{
			name:         "without wait",
			model:        "kling-v2.5",
			waitResult:   false,
			wantPoll:     false,
			wantSync:     true,
			wantModelLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New(Config{
				APIKey:            "k",
				Endpoint:          "generate/video",
				Model:             tt.model,
				WaitForCompletion: tt.waitResult,
			})
			caps := e.Capabilities()
			if caps.SupportsPoll != tt.wantPoll {
				t.Errorf("SupportsPoll = %v, want %v", caps.SupportsPoll, tt.wantPoll)
			}
			if caps.SupportsSync != tt.wantSync {
				t.Errorf("SupportsSync = %v, want %v", caps.SupportsSync, tt.wantSync)
			}
			if len(caps.Models) != tt.wantModelLen {
				t.Errorf("len(Models) = %d, want %d", len(caps.Models), tt.wantModelLen)
			}
			if caps.Models[0] != tt.model {
				t.Errorf("Models[0] = %q, want %q", caps.Models[0], tt.model)
			}
			if len(caps.MediaTypes) == 0 {
				t.Error("MediaTypes is empty")
			}
		})
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	if len(m) == 0 {
		t.Fatal("ModelsByCapability() returned empty map")
	}
	for _, cap := range []string{"image", "video"} {
		models, ok := m[cap]
		if !ok {
			t.Errorf("missing capability %q", cap)
			continue
		}
		if len(models) == 0 {
			t.Errorf("capability %q has no models", cap)
		}
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "runninghub" {
		t.Errorf("Name = %q, want runninghub", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("Configs is empty")
	}
	cfg := p.Configs[0]
	if cfg.Name != "runninghub" {
		t.Errorf("Config.Name = %q, want runninghub", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Error("Config.Engine is nil")
	}
	if len(cfg.EnvVars) == 0 {
		t.Error("Config.EnvVars is empty")
	}
}

func TestExecute_ValidationError(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey:   "test-key",
		Endpoint: "generate/image",
	})

	// Empty graph fails validation.
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestExecute_MissingEndpoint(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey: "test-key",
		// Endpoint intentionally omitted
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != ErrMissingEndpoint {
		t.Fatalf("error = %v, want ErrMissingEndpoint", err)
	}
}

func TestExecute_MissingAPIKey(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel.
	t.Setenv("RH_API_KEY", "")

	e := New(Config{
		Endpoint: "generate/image",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected API key error, got nil")
	}
}

func TestExecute_EmptyTaskID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"taskId":""}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Endpoint: "generate/image",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected empty taskId error, got nil")
	}
}

func TestPoll_CancelStatus(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/video",
		`{"taskId":"task-cancel"}`,
		[]string{
			`{"status":"CANCEL"}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/video",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected cancel error, got nil")
	}
}

func TestPoll_FailedWithErrorCodeOnly(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/video",
		`{"taskId":"task-errcode"}`,
		[]string{
			`{"status":"FAILED","errorCode":"ERR_QUOTA","errorMessage":""}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/video",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ERR_QUOTA") {
		t.Fatalf("expected ERR_QUOTA in error, got: %v", err)
	}
}

func TestPoll_SuccessWithTextResult(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/text",
		`{"taskId":"task-text"}`,
		[]string{
			`{"status":"SUCCESS","results":[{"url":"","text":"generated text content"}]}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/text",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "generated text content" {
		t.Errorf("Value = %q, want %q", result.Value, "generated text content")
	}
}

func TestPoll_SuccessEmptyResults(t *testing.T) {
	t.Parallel()

	server := submitAndPollServer(t, "generate/video",
		`{"taskId":"task-empty"}`,
		[]string{
			`{"status":"SUCCESS","results":[]}`,
		},
	)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "generate/video",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected empty results error, got nil")
	}
}

func TestResume_MissingAPIKey(t *testing.T) {
	t.Setenv("RH_API_KEY", "")

	e := New(Config{
		Endpoint: "generate/video",
	})

	_, err := e.Resume(context.Background(), "task-123")
	if err == nil {
		t.Fatal("expected API key error, got nil")
	}
}

func TestBuildPayload_LoadVideo(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.Write([]byte(`{"taskId":"t1"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Endpoint: "generate/video",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "enhance video"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/input.mp4"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if captured["videoUrl"] != "https://example.com/input.mp4" {
		t.Errorf("videoUrl = %v, want https://example.com/input.mp4", captured["videoUrl"])
	}
}

func TestNew_BaseURLFromEnv(t *testing.T) {
	t.Setenv("RH_BASE_URL", "https://custom.example.com/api")

	e := New(Config{
		APIKey:   "test-key",
		Endpoint: "generate/image",
	})

	if e.baseURL != "https://custom.example.com/api" {
		t.Errorf("baseURL = %q, want https://custom.example.com/api", e.baseURL)
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	t.Setenv("RH_BASE_URL", "")

	e := New(Config{
		APIKey:   "test-key",
		Endpoint: "generate/image",
	})

	if e.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos() returned empty slice")
	}
	info := infos[0]
	if info.Provider != "runninghub" {
		t.Errorf("Provider = %q, want runninghub", info.Provider)
	}
	if info.Name == "" {
		t.Error("Name is empty")
	}
}
