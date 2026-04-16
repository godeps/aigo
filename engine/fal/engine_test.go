package fal

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Key ") {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			w.Write([]byte(`{"request_id":"req-001"}`))
			return
		}
		// GET — distinguish status vs result by path suffix.
		if strings.Contains(r.URL.Path, "/status") {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"status":"IN_QUEUE"}`))
				return
			}
			w.Write([]byte(`{"status":"COMPLETED"}`))
			return
		}
		// Result fetch.
		w.Write([]byte(`{"images":[{"url":"https://fal.media/image.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "cyberpunk city"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://fal.media/image.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"request_id":"req-nowait"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/schnell",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "req-nowait" {
		t.Fatalf("Value = %q, want req-nowait", result.Value)
	}
}

func TestPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"request_id":"req-fail"}`))
			return
		}
		if strings.Contains(r.URL.Path, "/status") {
			w.Write([]byte(`{"status":"FAILED"}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed request")
	}
	if !contains(err.Error(), "failed") {
		t.Fatalf("error = %q, want to contain 'failed'", err.Error())
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/status") {
			w.Write([]byte(`{"status":"COMPLETED"}`))
			return
		}
		// Result.
		w.Write([]byte(`{"images":[{"url":"https://fal.media/resumed.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:       "test-key",
		QueueURL:     server.URL,
		Model:        "fal-ai/flux/dev",
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "req-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://fal.media/resumed.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("FAL_KEY", "")

	e := New(Config{Model: "fal-ai/flux/dev"})

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

func TestMissingModel(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if err != ErrMissingModel {
		t.Fatalf("error = %v, want ErrMissingModel", err)
	}
}

func TestExecuteHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid key"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "bad-key",
		QueueURL: server.URL,
		Model:    "fal-ai/flux/dev",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestPollHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"request_id":"req-poll-err"}`))
			return
		}
		// Status poll returns 500.
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for poll HTTP failure")
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
		if f.Key == "apiKey" && f.Required && f.EnvVar == "FAL_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with FAL_KEY env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	images, ok := models["image"]
	if !ok || len(images) == 0 {
		t.Fatal("ModelsByCapability() missing image models")
	}
	videos, ok := models["video"]
	if !ok || len(videos) == 0 {
		t.Fatal("ModelsByCapability() missing video models")
	}

	foundFlux := false
	for _, m := range images {
		if m == ModelFluxDev {
			foundFlux = true
		}
	}
	if !foundFlux {
		t.Fatalf("ModelsByCapability() missing %q in image models", ModelFluxDev)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelFluxDev})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Fatalf("MediaTypes = %v, want 2 types", cap.MediaTypes)
	}
}

func TestCapabilitiesWaitForCompletion(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelFluxDev, WaitForCompletion: true})
	cap := e.Capabilities()
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
	if cap.SupportsSync {
		t.Fatal("SupportsSync should be false when WaitForCompletion is true")
	}
}

func TestCapabilitiesNoWait(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelFluxSchnell, WaitForCompletion: false})
	cap := e.Capabilities()
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false when WaitForCompletion is false")
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true when WaitForCompletion is false")
	}
}

func TestExtractResultVideo(t *testing.T) {
	t.Parallel()
	body := []byte(`{"video":{"url":"https://fal.media/video.mp4"}}`)
	result, err := extractResult(body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "https://fal.media/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestExtractResultFallbackJSON(t *testing.T) {
	t.Parallel()
	// Response with neither images nor video — falls back to raw JSON.
	body := []byte(`{"custom_output":"some-data"}`)
	result, err := extractResult(body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != engine.OutputJSON {
		t.Fatalf("Kind = %v, want OutputJSON", result.Kind)
	}
}

func TestExecuteVideoResult(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"request_id":"req-video"}`))
			return
		}
		if strings.Contains(r.URL.Path, "/status") {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"status":"IN_PROGRESS"}`))
				return
			}
			w.Write([]byte(`{"status":"COMPLETED"}`))
			return
		}
		w.Write([]byte(`{"video":{"url":"https://fal.media/output.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             ModelKling,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a flying dragon"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://fal.media/output.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
