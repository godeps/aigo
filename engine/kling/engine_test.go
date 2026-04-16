package kling

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

func TestExecuteText2Video(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			if !strings.HasSuffix(r.URL.Path, "/v1/videos/text2video") {
				t.Fatalf("unexpected POST path: %s", r.URL.Path)
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "a rocket launch" {
				t.Fatalf("prompt = %v", body["prompt"])
			}
			if body["model_name"] != ModelKlingV2 {
				t.Fatalf("model_name = %v", body["model_name"])
			}
			w.Write([]byte(`{"data":{"task_id":"task-123"}}`))
			return
		}
		// GET poll
		if !strings.Contains(r.URL.Path, "task-123") {
			t.Fatalf("unexpected poll path: %s", r.URL.Path)
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"data":{"task_status":"processing","task_result":{}}}`))
			return
		}
		w.Write([]byte(`{"data":{"task_status":"completed","task_result":{"videos":[{"url":"https://cdn.klingai.com/video.mp4"}]}}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a rocket launch"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
	if result.Value != "https://cdn.klingai.com/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteImage2Video(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if !strings.HasSuffix(r.URL.Path, "/v1/videos/image2video") {
				t.Fatalf("expected image2video endpoint, got %s", r.URL.Path)
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["image"] != "https://example.com/img.png" {
				t.Fatalf("image = %v", body["image"])
			}
			w.Write([]byte(`{"data":{"task_id":"task-i2v"}}`))
			return
		}
		w.Write([]byte(`{"data":{"task_status":"completed","task_result":{"videos":[{"url":"https://cdn.klingai.com/i2v.mp4"}]}}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "animate this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.klingai.com/i2v.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if !strings.HasSuffix(r.URL.Path, "/v1/images/generations") {
				t.Fatalf("expected images endpoint, got %s", r.URL.Path)
			}
			w.Write([]byte(`{"data":{"task_id":"img-001"}}`))
			return
		}
		if !strings.Contains(r.URL.Path, "/v1/images/") {
			t.Fatalf("expected image poll path, got %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":{"task_status":"succeed","task_result":{"images":[{"url":"https://cdn.klingai.com/photo.png"}]}}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          EndpointImage,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset over mountains"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.klingai.com/photo.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"task_id":"task-nowait"}}`))
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
		t.Fatalf("Kind = %v, want PlainText", result.Kind)
	}
	if result.Value != "task-nowait" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"data":{"task_id":"task-fail"}}`))
			return
		}
		w.Write([]byte(`{"data":{"task_status":"failed"},"message":"content policy violation"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content policy violation") {
		t.Fatalf("error = %v, want content policy violation", err)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "task-resume") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":{"task_status":"completed","task_result":{"videos":[{"url":"https://cdn.klingai.com/resumed.mp4"}]}}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Resume(context.Background(), "task-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Value != "https://cdn.klingai.com/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestMissingAPIKey(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != ErrMissingAPIKey {
		t.Fatalf("error = %v, want ErrMissingAPIKey", err)
	}

	_, err = e.Resume(context.Background(), "some-id")
	if err != ErrMissingAPIKey {
		t.Fatalf("Resume error = %v, want ErrMissingAPIKey", err)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("expected at least 2 fields, got %d", len(fields))
	}
	found := false
	for _, f := range fields {
		if f.Key == "apiKey" && f.EnvVar == "KLING_API_KEY" && f.Required {
			found = true
		}
	}
	if !found {
		t.Fatal("missing apiKey field with correct EnvVar")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	if len(m["video"]) == 0 {
		t.Fatal("expected video models")
	}
	if len(m["image"]) == 0 {
		t.Fatal("expected image models")
	}
}

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelKlingV2})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.MaxDuration != 10 {
		t.Fatalf("MaxDuration = %d", cap.MaxDuration)
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelKlingV2, Endpoint: EndpointImage})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.MaxDuration != 0 {
		t.Fatalf("MaxDuration = %d, want 0 for image", cap.MaxDuration)
	}
}
