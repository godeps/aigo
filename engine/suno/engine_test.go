package suno

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

		if r.Method == http.MethodPost && r.URL.Path == "/api/generate" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "indie folk, melancholy" {
				t.Fatalf("prompt = %v", body["prompt"])
			}
			if body["model"] != ModelChirpV4 {
				t.Fatalf("model = %v", body["model"])
			}
			w.Write([]byte(`[{"id":"clip-001","status":"queued"}]`))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/api/feed/clip-001" {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`[{"id":"clip-001","status":"processing"}]`))
				return
			}
			w.Write([]byte(`[{"id":"clip-001","status":"complete","audio_url":"https://cdn.suno.ai/song.mp3"}]`))
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
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "indie folk, melancholy"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.suno.ai/song.mp3" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"clip-nowait","status":"queued"}]`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "upbeat pop"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "clip-nowait" {
		t.Fatalf("Value = %q, want clip ID", result.Value)
	}
}

func TestExecuteImmediateURL(t *testing.T) {
	t.Parallel()

	// If the POST response already contains audio_url, return it without polling.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"clip-fast","status":"complete","audio_url":"https://cdn.suno.ai/fast.mp3"}]`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "jazz"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.suno.ai/fast.mp3" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecutePollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`[{"id":"clip-fail","status":"queued"}]`))
			return
		}
		w.Write([]byte(`[{"id":"clip-fail","status":"error"}]`))
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
		t.Fatal("expected error for failed clip")
	}
	if !strings.Contains(err.Error(), "generation failed") {
		t.Fatalf("error = %q, want to contain 'generation failed'", err.Error())
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/feed/clip-resume" {
			w.Write([]byte(`[{"id":"clip-resume","status":"complete","audio_url":"https://cdn.suno.ai/resumed.mp3"}]`))
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

	result, err := e.Resume(context.Background(), "clip-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.suno.ai/resumed.mp3" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestResumeMissingBaseURL(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})
	_, err := e.Resume(context.Background(), "clip-123")
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
	if err != ErrMissingBaseURL {
		t.Fatalf("expected ErrMissingBaseURL, got: %v", err)
	}
}

func TestMissingAPIKey(t *testing.T) {
	orig := os.Getenv("SUNO_API_KEY")
	os.Unsetenv("SUNO_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("SUNO_API_KEY", orig)
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

	_, err = e.Resume(context.Background(), "clip-123")
	if err == nil {
		t.Fatal("expected error for missing API key on Resume")
	}
}

func TestMissingBaseURL(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
	if err != ErrMissingBaseURL {
		t.Fatalf("expected ErrMissingBaseURL, got: %v", err)
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
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Write([]byte(`[{"id":"clip-opts","status":"complete","audio_url":"https://cdn.suno.ai/opts.mp3"}]`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "epic orchestral"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"lyrics":          "verse one lyrics",
			"is_instrumental": true,
			"title":           "Epic Theme",
			"tags":            "orchestral,epic",
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.suno.ai/opts.mp3" {
		t.Fatalf("Value = %q", result.Value)
	}
	if gotPayload["lyrics"] != "verse one lyrics" {
		t.Fatalf("lyrics = %v", gotPayload["lyrics"])
	}
	if gotPayload["make_instrumental"] != true {
		t.Fatalf("make_instrumental = %v", gotPayload["make_instrumental"])
	}
	if gotPayload["title"] != "Epic Theme" {
		t.Fatalf("title = %v", gotPayload["title"])
	}
	if gotPayload["tags"] != "orchestral,epic" {
		t.Fatalf("tags = %v", gotPayload["tags"])
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

	foundAPIKey := false
	foundBaseURL := false
	for _, f := range schema {
		if f.Key == "apiKey" && f.Required && f.EnvVar == "SUNO_API_KEY" {
			foundAPIKey = true
		}
		if f.Key == "baseUrl" && f.Required && f.EnvVar == "SUNO_BASE_URL" {
			foundBaseURL = true
		}
	}
	if !foundAPIKey {
		t.Fatal("ConfigSchema() missing required apiKey field with SUNO_API_KEY env var")
	}
	if !foundBaseURL {
		t.Fatal("ConfigSchema() missing required baseUrl field with SUNO_BASE_URL env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	music, ok := models["music"]
	if !ok || len(music) == 0 {
		t.Fatal("ModelsByCapability() missing music models")
	}

	found := false
	for _, m := range music {
		if m == ModelChirpV4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("ModelsByCapability() missing %q", ModelChirpV4)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{
		BaseURL:           "https://example.com",
		Model:             ModelChirpV4,
		WaitForCompletion: true,
	})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "audio" {
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

	e := New(Config{
		BaseURL:           "https://example.com",
		WaitForCompletion: false,
	})
	cap := e.Capabilities()
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false when WaitForCompletion is false")
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true when WaitForCompletion is false")
	}
}
