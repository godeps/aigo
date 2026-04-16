package stability

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteCallsAPI(t *testing.T) {
	t.Parallel()

	fakeImage := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v2beta/stable-image/generate/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("Content-Type = %q, want multipart", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat in space"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value = %q, want data URI prefix", result.Value[:40])
	}
}

func TestExecuteWithNegativePrompt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(1 << 20)
		negPrompt := r.FormValue("negative_prompt")
		if negPrompt != "blurry, low quality" {
			t.Fatalf("negative_prompt = %q, want 'blurry, low quality'", negPrompt)
		}
		w.Header().Set("Content-Type", "application/json")
		fakeImage := base64.StdEncoding.EncodeToString([]byte("img"))
		w.Write([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a bright sunny day"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"negative_prompt": "blurry, low quality"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
}

func TestExecuteURLResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Some endpoints return a URL directly in the image field.
		w.Write([]byte(`{"image":"https://cdn.stability.ai/result.png","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a mountain lake"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.stability.ai/result.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteModelCoreEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2beta/stable-image/generate/core" {
			t.Fatalf("path = %q, want /v2beta/stable-image/generate/core", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fakeImage := base64.StdEncoding.EncodeToString([]byte("core-img"))
		w.Write([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: ModelImageCore})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a lighthouse"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
}

func TestExecuteModelUltraEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2beta/stable-image/generate/ultra" {
			t.Fatalf("path = %q, want /v2beta/stable-image/generate/ultra", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fakeImage := base64.StdEncoding.EncodeToString([]byte("ultra-img"))
		w.Write([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: ModelImageUltra})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "ultra quality photo"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
}

func TestMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("STABILITY_API_KEY", "")

	e := New(Config{})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecuteHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"name":"unauthorized","message":"invalid api key"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "bad-key", BaseURL: server.URL})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestExecuteHTTP500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestExecuteMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})

	// Graph with no CLIPTextEncode node.
	graph := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelSD35Large})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true")
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false for stability (sync only)")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	images, ok := m["image"]
	if !ok || len(images) == 0 {
		t.Fatal("expected image models")
	}

	found := false
	for _, name := range images {
		if name == ModelSD35Large {
			found = true
		}
	}
	if !found {
		t.Fatalf("ModelsByCapability() missing %q", ModelSD35Large)
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
		if f.Key == "apiKey" && f.Required && f.EnvVar == "STABILITY_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("ConfigSchema() missing required apiKey field with STABILITY_API_KEY env var")
	}
}

func TestExtractResultNoImage(t *testing.T) {
	t.Parallel()
	_, err := extractResult([]byte(`{"finish_reason":"SUCCESS"}`))
	if err == nil {
		t.Fatal("expected error when image field is absent")
	}
}

func TestExtractResultBase64(t *testing.T) {
	t.Parallel()
	fakeImage := base64.StdEncoding.EncodeToString([]byte("raw-bytes"))
	result, err := extractResult([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	if err != nil {
		t.Fatalf("extractResult() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value = %q, want data URI prefix", result.Value)
	}
}
