package midjourney

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

func TestExecuteImagineWithPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("X-API-Key = %q, want %q", got, "test-key")
		}
		w.Header().Set("Content-Type", "application/json")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		if r.URL.Path == "/mj/v2/imagine" {
			if body["prompt"] != "a beautiful sunset" {
				t.Fatalf("prompt = %v", body["prompt"])
			}
			if body["process_mode"] != "fast" {
				t.Fatalf("process_mode = %v", body["process_mode"])
			}
			w.Write([]byte(`{"task_id":"task-abc-123"}`))
			return
		}

		if r.URL.Path == "/mj/v2/fetch" {
			if body["task_id"] != "task-abc-123" {
				t.Fatalf("task_id = %v", body["task_id"])
			}
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"status":"processing","task_result":{}}`))
				return
			}
			w.Write([]byte(`{"status":"finished","task_result":{"image_url":"https://cdn.example.com/image.png"}}`))
			return
		}

		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.example.com/image.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if got := atomic.LoadInt32(&pollCount); got < 2 {
		t.Fatalf("pollCount = %d, want >= 2", got)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task_id":"task-nowait-456"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "task-nowait-456" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecutePollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/mj/v2/imagine" {
			w.Write([]byte(`{"task_id":"task-fail-789"}`))
			return
		}
		w.Write([]byte(`{"status":"failed","task_result":{}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test prompt"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error for failed task")
	}
	if got := err.Error(); got == "" {
		t.Fatal("error message should not be empty")
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"finished","task_result":{"image_url":"https://cdn.example.com/resumed.png"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Resume(context.Background(), "task-resume-999")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.example.com/resumed.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv is needed.
	t.Setenv("MIDJOURNEY_API_KEY", "")

	e := New(Config{})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestMissingPrompt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "SomeNode", Inputs: map[string]any{"other": "value"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != ErrMissingPrompt {
		t.Fatalf("Execute() error = %v, want ErrMissingPrompt", err)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 3 {
		t.Fatalf("ConfigSchema() returned %d fields, want 3", len(fields))
	}

	apiKeyField := fields[0]
	if apiKeyField.Key != "apiKey" {
		t.Fatalf("fields[0].Key = %q", apiKeyField.Key)
	}
	if !apiKeyField.Required {
		t.Fatal("apiKey field should be required")
	}
	if apiKeyField.EnvVar != "MIDJOURNEY_API_KEY" {
		t.Fatalf("apiKey.EnvVar = %q", apiKeyField.EnvVar)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	imgs, ok := models["image"]
	if !ok || len(imgs) == 0 {
		t.Fatal("ModelsByCapability() should have image models")
	}
}

func TestHTTPErrorResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error for 401 response")
	}
}
