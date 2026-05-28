package fal

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

func TestExecuteWithPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Key ") {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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
	if !strings.Contains(err.Error(), "failed") {
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

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "fal" {
		t.Fatalf("Name = %q, want fal", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("DefaultProvider() returned no configs")
	}
	cfg := p.Configs[0]
	if cfg.Name != "fal" {
		t.Fatalf("Configs[0].Name = %q, want fal", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Fatal("Configs[0].Engine is nil")
	}
	foundEnv := false
	for _, ev := range cfg.EnvVars {
		if ev == "FAL_KEY" {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Fatal("Configs[0].EnvVars missing FAL_KEY")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) != 6 {
		t.Fatalf("ModelInfos() returned %d entries, want 6", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "fal" {
			t.Fatalf("Provider = %q, want fal", info.Provider)
		}
		if info.Name == "" {
			t.Fatal("ModelInfo has empty Name")
		}
		if info.Capability != "image" && info.Capability != "video" {
			t.Fatalf("Capability = %q, want image or video", info.Capability)
		}
	}
}

func TestExecuteWithAllOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			// Verify the request body contains all expected fields.
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
				w.WriteHeader(500)
				return
			}
			if body["prompt"] != "test prompt" {
				t.Errorf("prompt = %v", body["prompt"])
			}
			if body["negative_prompt"] != "ugly" {
				t.Errorf("negative_prompt = %v", body["negative_prompt"])
			}
			if body["seed"] == nil {
				t.Error("missing seed")
			}
			if body["num_inference_steps"] == nil {
				t.Error("missing num_inference_steps")
			}
			imgSize, ok := body["image_size"].(map[string]any)
			if !ok {
				t.Error("missing image_size")
			} else {
				if imgSize["width"] == nil {
					t.Error("missing width in image_size")
				}
				if imgSize["height"] == nil {
					t.Error("missing height in image_size")
				}
			}
			w.Write([]byte(`{"request_id":"req-opts"}`))
			return
		}
		w.Write([]byte(`{"request_id":"req-opts"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test prompt"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"negative_prompt":     "ugly",
			"width":              1024,
			"height":             768,
			"seed":               42,
			"num_inference_steps": 30,
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "req-opts" {
		t.Fatalf("Value = %q, want req-opts", result.Value)
	}
}

func TestExecuteHeightOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
				w.WriteHeader(500)
				return
			}
			imgSize, ok := body["image_size"].(map[string]any)
			if !ok {
				t.Error("missing image_size")
			} else if imgSize["height"] == nil {
				t.Error("missing height in image_size")
			}
			w.Write([]byte(`{"request_id":"req-h"}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: false,
	})

	// Height without width — exercises the else branch at line 135.
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"height": 512}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestExecuteWithLoadImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
				w.WriteHeader(500)
				return
			}
			if body["image_url"] != "https://example.com/ref.png" {
				t.Errorf("image_url = %v, want https://example.com/ref.png", body["image_url"])
			}
			w.Write([]byte(`{"request_id":"req-img"}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "stylize this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.png"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "req-img" {
		t.Fatalf("Value = %q, want req-img", result.Value)
	}
}

func TestExecuteEmptyRequestID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"request_id":""}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty request_id")
	}
	if !strings.Contains(err.Error(), "empty request_id") {
		t.Fatalf("error = %q, want to contain 'empty request_id'", err.Error())
	}
}

func TestExecuteMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey: "test-key",
		Model:  "fal-ai/flux/dev",
	})

	// Graph with no CLIPTextEncode node — no prompt extractable.
	graph := workflow.Graph{
		"1": {ClassType: "SomeOtherNode", Inputs: map[string]any{"foo": "bar"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestExecuteContextCanceled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"request_id":"req-ctx"}`))
			return
		}
		// Status always IN_QUEUE — forces polling until context canceled.
		w.Write([]byte(`{"status":"IN_QUEUE"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		QueueURL:          server.URL,
		Model:             "fal-ai/flux/dev",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(ctx, graph)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestExtractResultInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := extractResult([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode result") {
		t.Fatalf("error = %q, want to contain 'decode result'", err.Error())
	}
}

func TestDoRequestHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"bad gateway"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "test-key",
		QueueURL: server.URL,
		Model:    "fal-ai/flux/dev",
	})

	_, err := e.doRequest(context.Background(), http.MethodGet, server.URL+"/test", "test-key", nil)
	if err == nil {
		t.Fatal("expected error for 502 response")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey: "  my-key  ",
		Model:  "  fal-ai/flux/dev  ",
	})

	if e.apiKey != "my-key" {
		t.Fatalf("apiKey = %q, want trimmed", e.apiKey)
	}
	if e.model != "fal-ai/flux/dev" {
		t.Fatalf("model = %q, want trimmed", e.model)
	}
	if e.queueURL != defaultQueueURL {
		t.Fatalf("queueURL = %q, want default", e.queueURL)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v, want default", e.pollInterval)
	}
}

