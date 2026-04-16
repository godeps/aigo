package recraft

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteCallsAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["prompt"] != "a futuristic cityscape" {
			t.Fatalf("prompt = %v", body["prompt"])
		}
		if body["model"] != ModelRecraftV3 {
			t.Fatalf("model = %v", body["model"])
		}
		if body["style"] != StyleRealisticImage {
			t.Fatalf("style = %v", body["style"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://recraft.ai/image.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, Style: StyleRealisticImage})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a futuristic cityscape"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://recraft.ai/image.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestExecuteGraphStyleOverride(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["style"] != StyleVectorIllustration {
			t.Fatalf("style = %v, want %v", body["style"], StyleVectorIllustration)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://recraft.ai/vec.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, Style: StyleRealisticImage})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a logo"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"style": StyleVectorIllustration}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://recraft.ai/vec.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("RECRAFT_API_KEY", "")
	e := New(Config{})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecuteAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "bad-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelRecraftV3})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync = true")
	}
	if len(cap.Models) != 1 || cap.Models[0] != ModelRecraftV3 {
		t.Fatalf("Models = %v", cap.Models)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("expected at least 2 config fields, got %d", len(fields))
	}
	found := false
	for _, f := range fields {
		if f.Key == "apiKey" {
			found = true
			if f.EnvVar != "RECRAFT_API_KEY" {
				t.Fatalf("apiKey EnvVar = %q", f.EnvVar)
			}
			if !f.Required {
				t.Fatal("apiKey should be required")
			}
		}
	}
	if !found {
		t.Fatal("apiKey field not found in ConfigSchema")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	imgs, ok := models["image"]
	if !ok {
		t.Fatal("missing 'image' capability")
	}
	if len(imgs) != 2 {
		t.Fatalf("expected 2 image models, got %d", len(imgs))
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "k"})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q", e.baseURL)
	}
	if e.model != ModelRecraftV3 {
		t.Fatalf("model = %q", e.model)
	}
}
