package luma

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

func TestExecuteVideo(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "a dancing robot" {
				t.Fatalf("prompt = %v", body["prompt"])
			}
			w.Write([]byte(`{"id":"gen-abc"}`))
			return
		}
		// GET poll
		if !strings.Contains(r.URL.Path, "gen-abc") {
			t.Fatalf("unexpected poll path: %s", r.URL.Path)
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"state":"dreaming"}`))
			return
		}
		w.Write([]byte(`{"state":"completed","assets":{"video":"https://cdn.luma.ai/video.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a dancing robot"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestExecuteImage(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"img-123"}`))
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"state":"queued"}`))
			return
		}
		w.Write([]byte(`{"state":"completed","assets":{"image":"https://cdn.luma.ai/photo.png"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhoton1,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunset"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/photo.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelRay2})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelPhoton1})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}
