package ideogram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteSuccess(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/generate" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Api-Key"); got != "test-key" {
			t.Fatalf("Api-Key = %q", got)
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://ideogram.ai/image.png","prompt":"a beautiful sunset"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   ModelV2A,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://ideogram.ai/image.png" {
		t.Fatalf("Value = %q", result.Value)
	}

	imgReq, ok := gotPayload["image_request"].(map[string]any)
	if !ok {
		t.Fatalf("image_request missing from payload")
	}
	if imgReq["prompt"] != "a beautiful sunset" {
		t.Fatalf("prompt = %v", imgReq["prompt"])
	}
	if imgReq["model"] != ModelV2A {
		t.Fatalf("model = %v", imgReq["model"])
	}
}

func TestExecuteWithOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://ideogram.ai/opt.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cyberpunk city"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"negative_prompt": "blurry, low quality",
			"aspect_ratio":    "16:9",
			"style_type":      "REALISTIC",
			"seed":            42,
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://ideogram.ai/opt.png" {
		t.Fatalf("Value = %q", result.Value)
	}

	imgReq, _ := gotPayload["image_request"].(map[string]any)
	if imgReq["negative_prompt"] != "blurry, low quality" {
		t.Fatalf("negative_prompt = %v", imgReq["negative_prompt"])
	}
	if imgReq["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio = %v", imgReq["aspect_ratio"])
	}
	if imgReq["style_type"] != "REALISTIC" {
		t.Fatalf("style_type = %v", imgReq["style_type"])
	}
}

func TestMissingAPIKey(t *testing.T) {
	orig := os.Getenv("IDEOGRAM_API_KEY")
	os.Unsetenv("IDEOGRAM_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("IDEOGRAM_API_KEY", orig)
		}
	}()

	e := New(Config{BaseURL: "https://example.com"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://example.com",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("expected ErrMissingPrompt, got: %v", err)
	}
}

func TestExecuteHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestExecuteEmptyData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty data response")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	schema := ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}

	found := false
	for _, f := range schema {
		if f.Key == "apiKey" && f.Required && f.EnvVar == "IDEOGRAM_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with IDEOGRAM_API_KEY env var")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	images, ok := models["image"]
	if !ok || len(images) == 0 {
		t.Fatal("ModelsByCapability() missing image models")
	}

	found := false
	for _, m := range images {
		if m == ModelV2A {
			found = true
		}
	}
	if !found {
		t.Fatalf("ModelsByCapability() missing %q", ModelV2A)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelV2A})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true for Ideogram (sync engine)")
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false for Ideogram (sync engine)")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.model != ModelV2A {
		t.Fatalf("model = %q, want %q", e.model, ModelV2A)
	}
}
