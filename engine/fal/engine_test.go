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
		// GET — check if status or result request.
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
		t.Fatalf("Kind = %v", result.Kind)
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
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelFluxDev})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}
