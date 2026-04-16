package replicate

import (
	"context"
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
		t.Fatalf("Kind = %v", result.Kind)
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

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: "test"})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}
