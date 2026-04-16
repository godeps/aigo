package elevenlabs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteTTS(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/") {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q", got)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("fake-audio-bytes"))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, VoiceID: "voice-123"})
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
		t.Fatalf("Value prefix = %q", result.Value[:30])
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
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelMultilingualV2})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "audio" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}
