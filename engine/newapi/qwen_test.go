package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestRunQwenImageGenerations(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/qwen.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-max-vl",
		Route:   RouteQwenImagesGenerations,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a landscape"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/qwen.png" {
		t.Errorf("got %q", out.Value)
	}

	// Verify qwen-specific input.messages structure
	input, ok := gotPayload["input"].(map[string]any)
	if !ok {
		t.Fatal("missing input in payload")
	}
	msgs, ok := input["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatal("missing input.messages")
	}
	msg := msgs[0].(map[string]any)
	if msg["role"] != "user" {
		t.Errorf("role = %v, want user", msg["role"])
	}
}

func TestRunQwenImageGenerationsError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-max-vl",
		Route:   RouteQwenImagesGenerations,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}
