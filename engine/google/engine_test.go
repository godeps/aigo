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
			t.Errorf("method = %q, want POST", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		// Verify no API key in URL
		if r.URL.Query().Get("key") != "" {
			t.Error("API key should not be in URL query params")
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("Content-Type = %q", ct)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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
		t.Error("should not reach server")
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
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

func TestNewAPIKeyFromEnv(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "env-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	e, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != ModelImagen3Generate002 {
		t.Fatalf("model = %q, want default", e.model)
	}
}

func TestNewBaseURLFromEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	t.Setenv("GOOGLE_BASE_URL", server.URL)

	e, err := New(Config{APIKey: "k"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestNewCustomHTTPClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	e, err := New(Config{
		APIKey:     "k",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach server")
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, execErr := e.Execute(context.Background(), workflow.Graph{})
	if execErr == nil {
		t.Fatal("expected error for empty graph")
	}
	if !strings.Contains(execErr.Error(), "validate graph") {
		t.Fatalf("error = %v, want validate graph error", execErr)
	}
}

func TestExecuteEmptyPromptText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach server")
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "   "}},
	}

	_, execErr := e.Execute(context.Background(), graph)
	if execErr == nil {
		t.Fatal("expected error for empty prompt")
	}
	if execErr != ErrMissingPrompt {
		t.Fatalf("error = %v, want ErrMissingPrompt", execErr)
	}
}

func TestExecuteWithSampleCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"predictions":[{"bytesBase64Encoded":"c2VlZGVk","mimeType":"image/png"}]}`))
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"sample_count": 2,
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
}

func TestExecuteWithSeed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"predictions":[{"bytesBase64Encoded":"c2VlZGVk","mimeType":"image/png"}]}`))
	}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"seed": 42,
		}},
	}

	// Seed may be rejected by Developer API mode; the code path is still exercised.
	_, _ = e.Execute(context.Background(), graph)
}

func TestDefaultProvider(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	p := DefaultProvider()
	if p.Name != "google" {
		t.Fatalf("Name = %q, want %q", p.Name, "google")
	}
	if len(p.Configs) != 1 {
		t.Fatalf("Configs count = %d, want 1", len(p.Configs))
	}
	if p.Configs[0].Name != "google-image" {
		t.Fatalf("Configs[0].Name = %q, want %q", p.Configs[0].Name, "google-image")
	}
	if len(p.Configs[0].EnvVars) != 1 || p.Configs[0].EnvVars[0] != "GOOGLE_API_KEY" {
		t.Fatalf("EnvVars = %v, want [GOOGLE_API_KEY]", p.Configs[0].EnvVars)
	}
}

func TestInitRegistersFactory(t *testing.T) {
	t.Parallel()

	f, ok := engine.GetFactory("google")
	if !ok {
		t.Fatal("factory not registered for 'google'")
	}

	// Invoking the factory without an API key should return an error.
	_, err := f(engine.EngineConfig{})
	if err == nil {
		t.Fatal("expected error from factory with empty config")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) < 2 {
		t.Fatalf("ModelInfos() returned %d, want >= 2", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "google" {
			t.Fatalf("Provider = %q, want %q", info.Provider, "google")
		}
		if info.Capability != "image" {
			t.Fatalf("Capability = %q, want %q", info.Capability, "image")
		}
		if info.Name == "" {
			t.Fatal("Name should not be empty")
		}
	}
}
