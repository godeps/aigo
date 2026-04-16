package hedra

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
		if got := r.Header.Get("X-API-KEY"); got != "test-key" {
			t.Fatalf("X-API-KEY = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/v1/characters" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["text"] != "Hello world" {
				t.Fatalf("text = %v", body["text"])
			}
			if body["avatarImage"] != "https://example.com/face.png" {
				t.Fatalf("avatarImage = %v", body["avatarImage"])
			}
			if body["voiceUrl"] != "https://example.com/audio.mp3" {
				t.Fatalf("voiceUrl = %v", body["voiceUrl"])
			}
			if body["audioSource"] != "audio" {
				t.Fatalf("audioSource = %v", body["audioSource"])
			}
			w.Write([]byte(`{"jobId":"proj-abc"}`))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/v1/projects/proj-abc" {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"status":"processing"}`))
				return
			}
			w.Write([]byte(`{"status":"completed","videoUrl":"https://cdn.hedra.com/video.mp4"}`))
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
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Hello world"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/face.png"}},
		"3": {ClassType: "LoadAudio", Inputs: map[string]any{"url": "https://example.com/audio.mp3"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.hedra.com/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jobId":"proj-xyz"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Say hi"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/face.png"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "proj-xyz" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecutePollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			w.Write([]byte(`{"jobId":"proj-fail"}`))
			return
		}
		w.Write([]byte(`{"status":"failed","errorMessage":"generation quota exceeded"}`))
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
		t.Fatal("expected error for failed project")
	}
	if got := err.Error(); !contains(got, "generation quota exceeded") {
		t.Fatalf("error = %q, want to contain 'generation quota exceeded'", got)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/projects/proj-resume" {
			w.Write([]byte(`{"status":"completed","videoUrl":"https://cdn.hedra.com/resumed.mp4"}`))
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

	result, err := e.Resume(context.Background(), "proj-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.hedra.com/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("HEDRA_API_KEY", "")

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

	schema := ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}

	found := false
	for _, f := range schema {
		if f.Key == "apiKey" && f.Required && f.EnvVar == "HEDRA_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	videos, ok := models["video"]
	if !ok || len(videos) == 0 {
		t.Fatal("ModelsByCapability() missing video models")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
}

func TestExecuteTTS(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["voiceId"] != "voice-123" {
				t.Fatalf("voiceId = %v", body["voiceId"])
			}
			if body["audioSource"] != "tts" {
				t.Fatalf("audioSource = %v, want tts", body["audioSource"])
			}
			if body["aspectRatio"] != "16:9" {
				t.Fatalf("aspectRatio = %v", body["aspectRatio"])
			}
			w.Write([]byte(`{"jobId":"proj-tts"}`))
			return
		}

		w.Write([]byte(`{"status":"completed","videoUrl":"https://cdn.hedra.com/tts.mp4"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Hello from TTS"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/face.png"}},
		"3": {ClassType: "Options", Inputs: map[string]any{"voice_id": "voice-123", "aspect_ratio": "16:9"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.hedra.com/tts.mp4" {
		t.Fatalf("Value = %q", result.Value)
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
