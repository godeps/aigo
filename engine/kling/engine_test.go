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
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			if !strings.HasSuffix(r.URL.Path, "/v1/videos/text2video") {
				t.Errorf("unexpected POST path: %s", r.URL.Path)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			if body["prompt"] != "a rocket launch" {
				t.Errorf("prompt = %v", body["prompt"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if body["model_name"] != ModelKlingV2 {
				t.Errorf("model_name = %v", body["model_name"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"data":{"task_id":"task-123"}}`))
			return
		}
		// GET poll
		if !strings.Contains(r.URL.Path, "task-123") {
			t.Errorf("unexpected poll path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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
				t.Errorf("expected image2video endpoint, got %s", r.URL.Path)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			if body["image"] != "https://example.com/img.png" {
				t.Errorf("image = %v", body["image"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
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
				t.Errorf("expected images endpoint, got %s", r.URL.Path)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"data":{"task_id":"img-001"}}`))
			return
		}
		if !strings.Contains(r.URL.Path, "/v1/images/") {
			t.Errorf("expected image poll path, got %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	_, err = e.Resume(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for missing API key on Resume")
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

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "kling" {
		t.Fatalf("Name = %q, want kling", p.Name)
	}
	if len(p.Configs) != 2 {
		t.Fatalf("len(Configs) = %d, want 2", len(p.Configs))
	}
	if p.Configs[0].Name != "kling-video" {
		t.Fatalf("Configs[0].Name = %q, want kling-video", p.Configs[0].Name)
	}
	if p.Configs[1].Name != "kling-image" {
		t.Fatalf("Configs[1].Name = %q, want kling-image", p.Configs[1].Name)
	}
	for _, cfg := range p.Configs {
		if len(cfg.EnvVars) != 1 || cfg.EnvVars[0] != "KLING_API_KEY" {
			t.Fatalf("EnvVars = %v", cfg.EnvVars)
		}
		if cfg.Engine == nil {
			t.Fatalf("Engine is nil for %s", cfg.Name)
		}
	}
}

func TestExecuteImageWithOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			if body["negative_prompt"] != "blurry" {
				t.Errorf("negative_prompt = %v, want blurry", body["negative_prompt"])
			}
			if body["aspect_ratio"] != "16:9" {
				t.Errorf("aspect_ratio = %v, want 16:9", body["aspect_ratio"])
			}
			if body["image"] != "https://example.com/ref.png" {
				t.Errorf("image = %v", body["image"])
			}
			w.Write([]byte(`{"data":{"task_id":"img-opts"}}`))
			return
		}
		w.Write([]byte(`{"data":{"task_status":"succeed","task_result":{"images":[{"url":"https://cdn.klingai.com/opts.png"}]}}}`))
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
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"negative_prompt": "blurry", "aspect_ratio": "16:9"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.png"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.klingai.com/opts.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos() returned empty")
	}
	for _, info := range infos {
		if info.Provider != "kling" {
			t.Errorf("Provider = %q, want kling", info.Provider)
		}
	}
}
