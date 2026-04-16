package pika

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteWithPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/v1/generate" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["promptText"] != "a flying car" {
				t.Fatalf("promptText = %v", body["promptText"])
			}
			if body["model"] != ModelPika22 {
				t.Fatalf("model = %v", body["model"])
			}
			w.Write([]byte(`{"id":"pika-001"}`))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/v1/generate/pika-001" {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"status":"processing"}`))
				return
			}
			w.Write([]byte(`{"status":"completed","video":{"url":"https://cdn.pika.art/video.mp4"}}`))
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a flying car"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.pika.art/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"pika-nowait"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset over ocean"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "pika-nowait" {
		t.Fatalf("Value = %q, want task ID", result.Value)
	}
}

func TestExecutePollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"pika-fail"}`))
			return
		}
		w.Write([]byte(`{"status":"failed","error":"quota exceeded"}`))
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
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("error = %q, want to contain 'quota exceeded'", err.Error())
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/generate/pika-resume" {
			w.Write([]byte(`{"status":"completed","video":{"url":"https://cdn.pika.art/resumed.mp4"}}`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	e := New(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "pika-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.pika.art/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	orig := os.Getenv("PIKA_API_KEY")
	os.Unsetenv("PIKA_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("PIKA_API_KEY", orig)
		}
	}()

	e := New(Config{BaseURL: "https://example.com"})

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

func TestMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://example.com",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("expected ErrMissingPrompt, got: %v", err)
	}
}

func TestExecuteWithOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Write([]byte(`{"id":"pika-opts"}`))
			return
		}
		w.Write([]byte(`{"status":"completed","video":{"url":"https://cdn.pika.art/opts.mp4"}}`))
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
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.jpg"}},
		"3": {ClassType: "Options", Inputs: map[string]any{
			"duration":     5,
			"aspect_ratio": "16:9",
			"resolution":   "1080p",
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.pika.art/opts.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if img, ok := gotPayload["image"].(map[string]any); !ok || img["url"] != "https://example.com/ref.jpg" {
		t.Fatalf("image = %v", gotPayload["image"])
	}
	if gotPayload["duration"] == nil {
		t.Fatal("expected duration in payload")
	}
	if gotPayload["aspectRatio"] != "16:9" {
		t.Fatalf("aspectRatio = %v", gotPayload["aspectRatio"])
	}
	if gotPayload["resolution"] != "1080p" {
		t.Fatalf("resolution = %v", gotPayload["resolution"])
	}
}

func TestExecuteHTTPError(t *testing.T) {
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
		t.Fatal("expected error for 401 response")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	schema := ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}

	found := false
	for _, f := range schema {
		if f.Key == "apiKey" && f.Required && f.EnvVar == "PIKA_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with PIKA_API_KEY env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	videos, ok := models["video"]
	if !ok || len(videos) == 0 {
		t.Fatal("ModelsByCapability() missing video models")
	}

	found22 := false
	for _, m := range videos {
		if m == ModelPika22 {
			found22 = true
		}
	}
	if !found22 {
		t.Fatalf("ModelsByCapability() missing %q", ModelPika22)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelPika22, WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
	if cap.SupportsSync {
		t.Fatal("SupportsSync should be false when WaitForCompletion is true")
	}
}

func TestCapabilitiesNoWait(t *testing.T) {
	t.Parallel()

	e := New(Config{WaitForCompletion: false})
	cap := e.Capabilities()
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false when WaitForCompletion is false")
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true when WaitForCompletion is false")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.model != ModelPika22 {
		t.Fatalf("model = %q, want %q", e.model, ModelPika22)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v, want %v", e.pollInterval, defaultPollInterval)
	}
}
