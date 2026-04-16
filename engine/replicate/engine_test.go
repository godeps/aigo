package replicate

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

func TestExecuteWithPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"pred-001","status":"starting"}`))
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"status":"processing"}`))
			return
		}
		w.Write([]byte(`{"status":"succeeded","output":["https://replicate.delivery/image.png"]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "stability-ai/sdxl:abc123",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a medieval castle"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://replicate.delivery/image.png" {
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
		w.Write([]byte(`{"id":"pred-nowait","status":"starting"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "some-model:v1",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "futuristic city"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "pred-nowait" {
		t.Fatalf("Value = %q, want pred-nowait", result.Value)
	}
}

func TestExecuteSynchronousCompletion(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Prediction already succeeded in the create response — no polling needed.
		w.Write([]byte(`{"id":"pred-sync","status":"succeeded","output":["https://replicate.delivery/sync.png"]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "fast-model:v1",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "instant result"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://replicate.delivery/sync.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"pred-fail","status":"starting"}`))
			return
		}
		w.Write([]byte(`{"status":"failed","error":"GPU out of memory"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "model:v1",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed prediction")
	}
	if !contains(err.Error(), "GPU out of memory") {
		t.Fatalf("error = %q, want to contain 'GPU out of memory'", err.Error())
	}
}

func TestPollCanceled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"pred-cancel","status":"starting"}`))
			return
		}
		w.Write([]byte(`{"status":"canceled"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "model:v1",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for canceled prediction")
	}
	if !contains(err.Error(), "canceled") {
		t.Fatalf("error = %q, want to contain 'canceled'", err.Error())
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/predictions/pred-resume" {
			w.Write([]byte(`{"status":"succeeded","output":"https://replicate.delivery/resumed.png"}`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	e := New(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		Model:        "model:v1",
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "pred-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://replicate.delivery/resumed.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("REPLICATE_API_TOKEN", "")

	e := New(Config{Model: "model:v1"})

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
		w.Write([]byte(`{"detail":"Invalid token"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "bad-key",
		BaseURL: server.URL,
		Model:   "model:v1",
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
			w.Write([]byte(`{"id":"pred-poll-err","status":"starting"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"internal server error"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "model:v1",
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
		if f.Key == "apiKey" && f.Required && f.EnvVar == "REPLICATE_API_TOKEN" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with REPLICATE_API_TOKEN env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	if _, ok := models["image"]; !ok {
		t.Fatal("ModelsByCapability() missing image models")
	}
	if _, ok := models["video"]; !ok {
		t.Fatal("ModelsByCapability() missing video models")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: "test"})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Fatalf("MediaTypes = %v, want 2 types", cap.MediaTypes)
	}
}

func TestCapabilitiesWaitForCompletion(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: "test", WaitForCompletion: true})
	cap := e.Capabilities()
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
	if cap.SupportsSync {
		t.Fatal("SupportsSync should be false when WaitForCompletion is true")
	}
}

func TestExtractOutputString(t *testing.T) {
	t.Parallel()
	result, err := extractOutput("https://example.com/img.png")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestExtractOutputArray(t *testing.T) {
	t.Parallel()
	result, err := extractOutput([]any{"https://example.com/a.png", "https://example.com/b.png"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "https://example.com/a.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExtractOutputFallbackJSON(t *testing.T) {
	t.Parallel()
	// When output is a map (unusual), should fall back to JSON.
	result, err := extractOutput(map[string]any{"key": "val"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != engine.OutputJSON {
		t.Fatalf("Kind = %v, want OutputJSON", result.Kind)
	}
}

func TestExecuteWithImageAndOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			input, _ := body["input"].(map[string]any)
			if input["image"] != "https://example.com/ref.jpg" {
				t.Fatalf("image = %v", input["image"])
			}
			w.Write([]byte(`{"id":"pred-opts","status":"starting"}`))
			return
		}
		w.Write([]byte(`{"status":"succeeded","output":["https://replicate.delivery/opt.png"]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "model:v1",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "enhance this photo"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.jpg"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
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
