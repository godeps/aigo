package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Fatalf("path = %q, want contains generateContent", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type = %q", ct)
		}
		// Verify no API key in URL
		if r.URL.Query().Get("key") != "" {
			t.Fatal("API key should not be in URL query params")
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		contents, _ := body["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("empty contents")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"A beautiful landscape."}]}}]}`))
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
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		contents, _ := body["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("empty contents")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"A cat sitting on a couch."}]}}]}`))
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
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	_, err := New(Config{})
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	e, err := New(Config{APIKey: "test-key", Model: ModelGemini20Flash, BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
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
	if len(m["text"]) != 4 {
		t.Fatalf("text models = %d, want 4", len(m["text"]))
	}
	if len(m["video"]) != 3 {
		t.Fatalf("video models = %d, want 3", len(m["video"]))
	}
}

func TestBuildSDKPart_URL(t *testing.T) {
	t.Parallel()

	ref := workflow.NodeRef{
		ID:   "1",
		Node: workflow.Node{ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.jpg"}},
	}
	p := buildSDKPart(ref, "image/jpeg")
	if p == nil {
		t.Fatal("expected non-nil part")
	}
	if p.FileData == nil {
		t.Fatal("expected FileData")
	}
	if p.FileData.FileURI != "https://example.com/img.jpg" {
		t.Fatalf("FileURI = %q", p.FileData.FileURI)
	}
	if p.FileData.MIMEType != "image/jpeg" {
		t.Fatalf("MIMEType = %q", p.FileData.MIMEType)
	}
}

func TestBuildSDKPart_InlineData(t *testing.T) {
	t.Parallel()

	rawData := []byte("test-image-data")
	b64 := base64.StdEncoding.EncodeToString(rawData)

	ref := workflow.NodeRef{
		ID:   "1",
		Node: workflow.Node{ClassType: "LoadImage", Inputs: map[string]any{"data": b64, "mime_type": "image/png"}},
	}
	p := buildSDKPart(ref, "image/jpeg")
	if p == nil {
		t.Fatal("expected non-nil part")
	}
	if p.InlineData == nil {
		t.Fatal("expected InlineData")
	}
	if p.InlineData.MIMEType != "image/png" {
		t.Fatalf("MIMEType = %q, want image/png", p.InlineData.MIMEType)
	}
	if string(p.InlineData.Data) != "test-image-data" {
		t.Fatalf("Data = %q", string(p.InlineData.Data))
	}
}

func TestBuildSDKPart_InvalidBase64(t *testing.T) {
	t.Parallel()

	ref := workflow.NodeRef{
		ID:   "1",
		Node: workflow.Node{ClassType: "LoadImage", Inputs: map[string]any{"data": "not-valid-base64!!!"}},
	}
	p := buildSDKPart(ref, "image/jpeg")
	if p != nil {
		t.Fatal("expected nil for invalid base64")
	}
}

func TestBuildSDKPart_Empty(t *testing.T) {
	t.Parallel()

	ref := workflow.NodeRef{
		ID:   "1",
		Node: workflow.Node{ClassType: "LoadImage", Inputs: map[string]any{}},
	}
	p := buildSDKPart(ref, "image/jpeg")
	if p != nil {
		t.Fatal("expected nil for empty inputs")
	}
}

func TestBuildSDKPart_CustomMimeURL(t *testing.T) {
	t.Parallel()

	ref := workflow.NodeRef{
		ID:   "1",
		Node: workflow.Node{ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/vid.webm", "mime_type": "video/webm"}},
	}
	p := buildSDKPart(ref, "video/mp4")
	if p == nil {
		t.Fatal("expected non-nil part")
	}
	if p.FileData.MIMEType != "video/webm" {
		t.Fatalf("MIMEType = %q, want video/webm", p.FileData.MIMEType)
	}
}
