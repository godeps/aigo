package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestCompileExtractsPromptAndSize(t *testing.T) {
	t.Parallel()

	engine := New(Config{})
	graph := workflow.Graph{
		"1": {
			ClassType: "PromptSource",
			Inputs:    map[string]any{"value": "cinematic city skyline"},
		},
		"2": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": []any{"1", 0}},
		},
		"3": {
			ClassType: "EmptyLatentImage",
			Inputs:    map[string]any{"width": 1600, "height": 900},
		},
	}

	req, err := engine.Compile(graph)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if req.Prompt != "cinematic city skyline" {
		t.Fatalf("Compile().Prompt = %q, want %q", req.Prompt, "cinematic city skyline")
	}
	if req.Size != "1536x1024" {
		t.Fatalf("Compile().Size = %q, want %q", req.Size, "1536x1024")
	}
}

func TestExecuteCallsImagesAPI(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/images/generations")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/image.png"}]}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": "an astronaut reading in a tea house"},
		},
		"2": {
			ClassType: "EmptyLatentImage",
			Inputs:    map[string]any{"width": 1024, "height": 1536},
		},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got != "https://cdn.example.com/image.png" {
		t.Fatalf("Execute() = %q, want image URL", got)
	}

	if gotPayload["prompt"] != "an astronaut reading in a tea house" {
		t.Fatalf("prompt = %#v", gotPayload["prompt"])
	}
	if gotPayload["size"] != "1024x1536" {
		t.Fatalf("size = %#v", gotPayload["size"])
	}
	if gotPayload["model"] != defaultModel {
		t.Fatalf("model = %#v, want %q", gotPayload["model"], defaultModel)
	}
}
