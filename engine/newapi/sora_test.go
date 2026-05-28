package newapi

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestRunSoraVideoFullFlow(t *testing.T) {
	t.Parallel()

	var pollCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-t1","status":"queued"}`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/videos/sora-t1/content"):
			w.Header().Set("Content-Type", "video/mp4")
			w.Write([]byte{0x00, 0x01, 0x02, 0x03})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/videos/sora-t1"):
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount < 2 {
				w.Write([]byte(`{"id":"sora-t1","status":"in_progress"}`))
			} else {
				w.Write([]byte(`{"id":"sora-t1","status":"completed"}`))
			}

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset timelapse"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	// Result should be a data URI with base64 encoded video
	wantPrefix := "data:video/mp4;base64,"
	if !strings.HasPrefix(out.Value, wantPrefix) {
		t.Errorf("got %q, want prefix %q", out.Value, wantPrefix)
	}
	// Decode and verify content
	b64Part := strings.TrimPrefix(out.Value, wantPrefix)
	decoded, err := base64.StdEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if len(decoded) != 4 {
		t.Errorf("decoded len = %d, want 4", len(decoded))
	}
}

func TestRunSoraVideoNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"sora-id-123","status":"queued"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "sora-id-123" {
		t.Errorf("got %q, want sora-id-123", out.Value)
	}
}

func TestRunSoraVideoFailed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-fail","status":"queued"}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/sora-fail":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-fail","status":"failed","error":{"message":"content policy violation"}}`))

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content policy violation") {
		t.Errorf("error = %q, want to contain 'content policy violation'", err.Error())
	}
}

func TestRunSoraVideoMissingID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: true,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}
