package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		// Verify no API key in URL
		if r.URL.Query().Get("key") != "" {
			t.Fatal("API key should not be in URL query params")
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type = %q", ct)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"predictions":[{"bytesBase64Encoded":"aW1hZ2VkYXRh","mimeType":"image/png"}]}`))
	}))
	defer server.Close()

	e, err := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat on a rainbow"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value prefix wrong: %q", result.Value[:40])
	}
}

func TestExecuteWithOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"predictions":[{"bytesBase64Encoded":"dGVzdA==","mimeType":"image/jpeg"}]}`))
	}))
	defer server.Close()

	e, err := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   ModelImagen3Generate001,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "landscape"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"aspect_ratio": "16:9"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(result.Value, "data:image/jpeg;base64,") {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecuteMissingPrompt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"width": 1024}},
	}

	_, execErr := e.Execute(context.Background(), graph)
	if execErr == nil {
		t.Fatal("expected error for missing prompt")
	}
	if execErr != ErrMissingPrompt {
		t.Fatalf("error = %v, want ErrMissingPrompt", execErr)
	}
}

func TestExecuteAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"invalid request","code":400}}`))
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, execErr := e.Execute(context.Background(), graph)
	if execErr == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", Model: ModelImagen3Generate002, BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cap := e.Capabilities()

	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true")
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false")
	}
	if len(cap.Models) != 1 || cap.Models[0] != ModelImagen3Generate002 {
		t.Fatalf("Models = %v", cap.Models)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("ConfigSchema() returned %d fields, want >= 2", len(fields))
	}

	foundKey := false
	foundURL := false
	for _, f := range fields {
		switch f.Key {
		case "apiKey":
			foundKey = true
			if !f.Required {
				t.Fatal("apiKey should be required")
			}
			if f.EnvVar != "GOOGLE_API_KEY" {
				t.Fatalf("apiKey EnvVar = %q", f.EnvVar)
			}
		case "baseUrl":
			foundURL = true
			if f.Default != defaultBaseURL {
				t.Fatalf("baseUrl Default = %q", f.Default)
			}
		}
	}
	if !foundKey {
		t.Fatal("missing apiKey field")
	}
	if !foundURL {
		t.Fatal("missing baseUrl field")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	images, ok := m["image"]
	if !ok {
		t.Fatal("missing 'image' capability")
	}
	if len(images) < 2 {
		t.Fatalf("expected at least 2 image models, got %d", len(images))
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	e, err := New(Config{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != ModelImagen3Generate002 {
		t.Fatalf("model = %q, want %q", e.model, ModelImagen3Generate002)
	}
}
