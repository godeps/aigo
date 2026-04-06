package comfyui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestExecuteReturnsPromptID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/prompt" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			ClientID string         `json:"client_id"`
			Prompt   workflow.Graph `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode prompt: %v", err)
		}
		if payload.ClientID != "client-1" {
			t.Fatalf("client_id = %q, want client-1", payload.ClientID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prompt_id":"prompt-123"}`))
	}))
	defer server.Close()

	engine := New(Config{
		BaseURL:  server.URL,
		ClientID: "client-1",
	})

	got, err := engine.Execute(context.Background(), workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "prompt-123" {
		t.Fatalf("Execute() = %q, want prompt id", got.Value)
	}
}

func TestExecuteWaitsForHistory(t *testing.T) {
	t.Parallel()

	var historyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"prompt-456"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/prompt-456":
			call := historyCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_, _ = w.Write([]byte(`{"outputs":{"9":{"images":[{"filename":"result.png","subfolder":"outputs","type":"output"}]}}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	engine := New(Config{
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := engine.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := server.URL + "/view?filename=result.png&subfolder=outputs&type=output"
	if got.Value != want {
		t.Fatalf("Execute() = %q, want %q", got.Value, want)
	}
}
