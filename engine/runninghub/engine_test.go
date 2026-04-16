package runninghub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		json.NewDecoder(r.Body).Decode(&captured)
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
