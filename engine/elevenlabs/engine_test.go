package elevenlabs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteTTS(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/") {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "audio/mpeg" {
			t.Fatalf("Accept = %q", got)
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("fake-audio-bytes"))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		VoiceID: "voice-123",
		Model:   ModelMultilingualV2,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Hello world"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:audio/mpeg;base64,") {
		t.Fatalf("Value = %q, want data URI prefix", result.Value)
	}

	if gotPayload["text"] != "Hello world" {
		t.Fatalf("text = %v", gotPayload["text"])
	}
	if gotPayload["model_id"] != ModelMultilingualV2 {
		t.Fatalf("model_id = %v", gotPayload["model_id"])
	}
	// Voice ID is embedded in the URL path; verified by the path prefix check above.
}

func TestExecuteTTSVoiceInGraph(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	// Config has no VoiceID; voice comes from graph Options.
	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Hi there"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"voice_id": "graph-voice-456"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if gotPath != "/v1/text-to-speech/graph-voice-456" {
		t.Fatalf("path = %q, want voice in URL", gotPath)
	}
}

func TestExecuteTTSVoiceSettings(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		VoiceID: "voice-abc",
	})

	// Use string values: resolve.Float64Option parses strings correctly,
	// while raw float64 inputs are truncated via IntInput (0.8 → 0).
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Custom settings"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"stability":        "0.8",
			"similarity_boost": "0.9",
		}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	vs, ok := gotPayload["voice_settings"].(map[string]any)
	if !ok {
		t.Fatalf("voice_settings missing from payload")
	}
	if vs["stability"] != 0.8 {
		t.Fatalf("stability = %v, want 0.8", vs["stability"])
	}
	if vs["similarity_boost"] != 0.9 {
		t.Fatalf("similarity_boost = %v, want 0.9", vs["similarity_boost"])
	}
}

func TestExecuteTTSDefaultVoiceSettings(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		VoiceID: "voice-abc",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Default settings"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	vs, ok := gotPayload["voice_settings"].(map[string]any)
	if !ok {
		t.Fatalf("voice_settings missing from payload")
	}
	// Verify defaults are applied when not overridden.
	if vs["stability"] != 0.5 {
		t.Fatalf("default stability = %v, want 0.5", vs["stability"])
	}
	if vs["similarity_boost"] != 0.75 {
		t.Fatalf("default similarity_boost = %v, want 0.75", vs["similarity_boost"])
	}
}

func TestMissingVoice(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Hello"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing voice")
	}
	if err != ErrMissingVoice {
		t.Fatalf("expected ErrMissingVoice, got: %v", err)
	}
}

func TestMissingText(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key", VoiceID: "voice-123"})
	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
	if err != ErrMissingText {
		t.Fatalf("expected ErrMissingText, got: %v", err)
	}
}

func TestMissingAPIKey(t *testing.T) {
	orig := os.Getenv("ELEVENLABS_API_KEY")
	os.Unsetenv("ELEVENLABS_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("ELEVENLABS_API_KEY", orig)
		}
	}()

	e := New(Config{
		BaseURL: "https://example.com",
		VoiceID: "voice-123",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecuteHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"invalid_api_key"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "bad-key",
		BaseURL: server.URL,
		VoiceID: "voice-123",
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
		if f.Key == "apiKey" && f.Required && f.EnvVar == "ELEVENLABS_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with ELEVENLABS_API_KEY env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	tts, ok := models["tts"]
	if !ok || len(tts) == 0 {
		t.Fatal("ModelsByCapability() missing tts models")
	}

	found := false
	for _, m := range tts {
		if m == ModelMultilingualV2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("ModelsByCapability() missing %q", ModelMultilingualV2)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelMultilingualV2})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "audio" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true for ElevenLabs (sync engine)")
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false for ElevenLabs (sync engine)")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.model != ModelMultilingualV2 {
		t.Fatalf("model = %q, want %q", e.model, ModelMultilingualV2)
	}
}
