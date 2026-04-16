package gpt4o

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecute_TextOnly(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"This is a description."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe something"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "This is a description." {
		t.Fatalf("Value = %q, want %q", result.Value, "This is a description.")
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}

	// Verify the payload sent to the API.
	msgs, ok := gotPayload["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %v", gotPayload["messages"])
	}
	msg := msgs[0].(map[string]any)
	// Text-only: content should be a plain string.
	if content, ok := msg["content"].(string); !ok || content != "describe something" {
		t.Fatalf("content = %v, want plain string", msg["content"])
	}
	if gotPayload["model"] != defaultModel {
		t.Fatalf("model = %v, want %q", gotPayload["model"], defaultModel)
	}
}

func TestExecute_WithImage(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"The image shows a cat."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this image"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/cat.jpg"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "The image shows a cat." {
		t.Fatalf("Value = %q", result.Value)
	}

	// Verify multi-part content was sent.
	msgs := gotPayload["messages"].([]any)
	msg := msgs[0].(map[string]any)
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("expected array content for image request, got %T", msg["content"])
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(parts))
	}
	textPart := parts[0].(map[string]any)
	if textPart["type"] != "text" {
		t.Fatalf("first part type = %v, want text", textPart["type"])
	}
	imgPart := parts[1].(map[string]any)
	if imgPart["type"] != "image_url" {
		t.Fatalf("second part type = %v, want image_url", imgPart["type"])
	}
}

func TestExecute_MissingKey(t *testing.T) {
	t.Parallel()

	eng := New(Config{})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecute_MissingPrompt(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key"})
	g := workflow.Graph{
		"1": {ClassType: "Something", Inputs: map[string]any{}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 4 {
		t.Errorf("expected 4 config fields, got %d", len(fields))
	}
	// First field should be API key.
	if fields[0].Key != "apiKey" {
		t.Errorf("first field key = %q, want apiKey", fields[0].Key)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	eng := New(Config{})
	cap := eng.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Fatalf("expected 2 media types, got %v", cap.MediaTypes)
	}
	want := map[string]bool{"text": true, "image": true}
	for _, mt := range cap.MediaTypes {
		if !want[mt] {
			t.Errorf("unexpected media type %q", mt)
		}
	}
	if !cap.SupportsSync {
		t.Error("expected SupportsSync = true")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	if len(m["text"]) != 3 {
		t.Errorf("expected 3 text models, got %d", len(m["text"]))
	}
	if len(m["image"]) != 3 {
		t.Errorf("expected 3 image models, got %d", len(m["image"]))
	}
}
