package jimeng

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteImageSuccess(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.jimeng.com/img/result.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   ModelJimeng21,
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
	if result.Value != "https://cdn.jimeng.com/img/result.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if gotPayload["prompt"] != "a beautiful sunset" {
		t.Fatalf("prompt = %v", gotPayload["prompt"])
	}
	if gotPayload["model"] != ModelJimeng21 {
		t.Fatalf("model = %v", gotPayload["model"])
	}
}

func TestExecuteVideoSuccess(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/v1/video/generations" {
			_, _ = w.Write([]byte(`{"id":"task-vid-001"}`))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/video/generations/task-vid-001") {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":"https://cdn.jimeng.com/video/out.mp4"}}`))
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a rocket launch"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.jimeng.com/video/out.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteVideoNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/video/generations" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-nowait-002"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset over ocean"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "task-nowait-002" {
		t.Fatalf("expected task id, got %q", result.Value)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
}

func TestExecuteVideoPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"task-fail-003"}`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"status":"failed","error":{"code":"content_filter","message":"content blocked"}}`))
		}
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content blocked") {
		t.Fatalf("expected content blocked error, got: %v", err)
	}
}

func TestResumeSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer resume-key" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":"https://cdn.jimeng.com/video/resumed.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "resume-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	result, err := e.Resume(context.Background(), "task-resume-004")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Value != "https://cdn.jimeng.com/video/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestMissingAPIKey(t *testing.T) {
	t.Parallel()

	// Ensure env is not set.
	orig := os.Getenv("JIMENG_API_KEY")
	os.Unsetenv("JIMENG_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("JIMENG_API_KEY", orig)
		}
	}()

	e := New(Config{
		BaseURL: "https://example.com",
	})

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

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://example.com",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("expected ErrMissingPrompt, got: %v", err)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 2 {
		t.Fatalf("expected 2 config fields, got %d", len(fields))
	}

	apiKeyField := fields[0]
	if apiKeyField.Key != "apiKey" {
		t.Fatalf("first field key = %q", apiKeyField.Key)
	}
	if !apiKeyField.Required {
		t.Fatal("apiKey should be required")
	}
	if apiKeyField.EnvVar != "JIMENG_API_KEY" {
		t.Fatalf("apiKey envVar = %q", apiKeyField.EnvVar)
	}

	baseURLField := fields[1]
	if baseURLField.Key != "baseUrl" {
		t.Fatalf("second field key = %q", baseURLField.Key)
	}
	if baseURLField.Default != defaultBaseURL {
		t.Fatalf("baseUrl default = %q", baseURLField.Default)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	imageModels, ok := models["image"]
	if !ok {
		t.Fatal("expected image capability")
	}
	if len(imageModels) != 2 {
		t.Fatalf("expected 2 image models, got %d", len(imageModels))
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelJimeng21})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync for image model")
	}
}

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()

	e := New(Config{Endpoint: "/v1/video/generations", WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("expected SupportsPoll for video with WaitForCompletion")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q", e.baseURL)
	}
	if e.model != defaultModel {
		t.Fatalf("model = %q", e.model)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v", e.pollInterval)
	}
}

func TestImageB64Response(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"dGVzdA=="}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"response_format": "b64_json"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value = %q", result.Value)
	}
}
