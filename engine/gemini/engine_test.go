package gemini

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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("key") != "test-key" {
			t.Fatalf("key = %q", r.URL.Query().Get("key"))
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q", ct)
		}

		var body generateRequest
		json.NewDecoder(r.Body).Decode(&body)
		if len(body.Contents) == 0 || len(body.Contents[0].Parts) == 0 {
			t.Fatal("empty contents or parts")
		}
		if body.Contents[0].Parts[0].Text != "describe this scene" {
			t.Fatalf("prompt = %q", body.Contents[0].Parts[0].Text)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"A beautiful landscape."}]}}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this scene"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "A beautiful landscape." {
		t.Fatalf("Value = %q, want %q", result.Value, "A beautiful landscape.")
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
}

func TestExecute_WithImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body generateRequest
		json.NewDecoder(r.Body).Decode(&body)

		parts := body.Contents[0].Parts
		if len(parts) != 2 {
			t.Fatalf("parts count = %d, want 2", len(parts))
		}
		if parts[0].Text != "what is in this image" {
			t.Fatalf("text = %q", parts[0].Text)
		}
		if parts[1].FileData == nil {
			t.Fatal("expected file_data for image URL")
		}
		if parts[1].FileData.FileURI != "https://example.com/cat.jpg" {
			t.Fatalf("file_uri = %q", parts[1].FileData.FileURI)
		}
		if parts[1].FileData.MimeType != "image/jpeg" {
			t.Fatalf("mime_type = %q", parts[1].FileData.MimeType)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"A cat sitting on a couch."}]}}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "what is in this image"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/cat.jpg"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "A cat sitting on a couch." {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecute_MissingKey(t *testing.T) {
	e := New(Config{})
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecute_MissingPrompt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"width": 1024}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) < 3 {
		t.Fatalf("ConfigSchema() returned %d fields, want >= 3", len(fields))
	}

	foundKey := false
	foundURL := false
	foundModel := false
	for _, f := range fields {
		switch f.Key {
		case "apiKey":
			foundKey = true
			if !f.Required {
				t.Fatal("apiKey should be required")
			}
			if f.EnvVar != "GEMINI_API_KEY" {
				t.Fatalf("apiKey EnvVar = %q", f.EnvVar)
			}
		case "baseUrl":
			foundURL = true
			if f.Default != defaultBaseURL {
				t.Fatalf("baseUrl Default = %q", f.Default)
			}
		case "model":
			foundModel = true
			if f.Default != ModelGemini20Flash {
				t.Fatalf("model Default = %q", f.Default)
			}
		}
	}
	if !foundKey {
		t.Fatal("missing apiKey field")
	}
	if !foundURL {
		t.Fatal("missing baseUrl field")
	}
	if !foundModel {
		t.Fatal("missing model field")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelGemini20Flash})
	cap := e.Capabilities()

	if len(cap.MediaTypes) != 3 {
		t.Fatalf("MediaTypes = %v, want [text image video]", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true")
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false")
	}
	if len(cap.Models) != 1 || cap.Models[0] != ModelGemini20Flash {
		t.Fatalf("Models = %v", cap.Models)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	for _, cap := range []string{"text", "image", "video"} {
		models, ok := m[cap]
		if !ok {
			t.Fatalf("missing %q capability", cap)
		}
		if len(models) == 0 {
			t.Fatalf("%q has no models", cap)
		}
	}
	// text and image should have all 4 models
	if len(m["text"]) != 4 {
		t.Fatalf("text models = %d, want 4", len(m["text"]))
	}
	// video should have 3 models (no flash-lite)
	if len(m["video"]) != 3 {
		t.Fatalf("video models = %d, want 3", len(m["video"]))
	}
}
