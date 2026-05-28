package jina

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.model != DefaultModel {
		t.Errorf("model = %q, want %q", e.model, DefaultModel)
	}
	if e.dimensions != DefaultDimensions {
		t.Errorf("dimensions = %d, want %d", e.dimensions, DefaultDimensions)
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	t.Setenv("JINA_API_KEY", "")

	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	t.Parallel()

	e, err := New(Config{
		APIKey:     "custom-key",
		BaseURL:    "https://custom.example.com",
		Model:      "jina-embeddings-v3",
		Dimensions: 512,
		RPM:        100,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.baseURL != "https://custom.example.com" {
		t.Errorf("baseURL = %q, want %q", e.baseURL, "https://custom.example.com")
	}
	if e.model != "jina-embeddings-v3" {
		t.Errorf("model = %q, want %q", e.model, "jina-embeddings-v3")
	}
	if e.dimensions != 512 {
		t.Errorf("dimensions = %d, want 512", e.dimensions)
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key", Dimensions: 768})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.Dimensions() != 768 {
		t.Errorf("Dimensions() = %d, want 768", e.Dimensions())
	}
}

func TestEmbedCapabilities(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cap := e.EmbedCapabilities()
	if len(cap.SupportedTypes) != 2 {
		t.Fatalf("SupportedTypes length = %d, want 2", len(cap.SupportedTypes))
	}
	hasText, hasImage := false, false
	for _, st := range cap.SupportedTypes {
		if st == embed.ContentText {
			hasText = true
		}
		if st == embed.ContentImage {
			hasImage = true
		}
	}
	if !hasText {
		t.Error("expected ContentText in SupportedTypes")
	}
	if !hasImage {
		t.Error("expected ContentImage in SupportedTypes")
	}
	if cap.MaxDimensions != 1024 {
		t.Errorf("MaxDimensions = %d, want 1024", cap.MaxDimensions)
	}
	if !cap.SupportsMRL {
		t.Error("SupportsMRL should be true")
	}
	if len(cap.Models) != 2 {
		t.Errorf("Models length = %d, want 2", len(cap.Models))
	}
}

func newEmbedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req apiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		embeddings := make([]apiEmbeddingData, len(req.Input))
		for i := range req.Input {
			vec := make([]float32, req.Dimensions)
			for j := range vec {
				vec[j] = float32(i+1) * 0.1
			}
			embeddings[i] = apiEmbeddingData{Embedding: vec, Index: i}
		}

		resp := apiResponse{
			Data:  embeddings,
			Model: req.Model,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestEmbed_Text(t *testing.T) {
	t.Parallel()

	srv := newEmbedTestServer(t)
	defer srv.Close()

	eng, err := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := eng.Embed(context.Background(), embed.TextRequest("hello world", ""))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(result.Vector) != DefaultDimensions {
		t.Errorf("vector length = %d, want %d", len(result.Vector), DefaultDimensions)
	}
	if result.Model != DefaultModel {
		t.Errorf("model = %q, want %q", result.Model, DefaultModel)
	}
	if result.Dimensions != DefaultDimensions {
		t.Errorf("dimensions = %d, want %d", result.Dimensions, DefaultDimensions)
	}
}

func TestEmbed_TextRequestBody(t *testing.T) {
	t.Parallel()

	var capturedReq apiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		resp := apiResponse{
			Data:  []apiEmbeddingData{{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0}},
			Model: "jina-clip-v2",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, _ := New(Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		Dimensions: 3,
	})

	_, err := eng.Embed(context.Background(), embed.TextRequest("test input", ""))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if capturedReq.Model != DefaultModel {
		t.Errorf("request model = %q, want %q", capturedReq.Model, DefaultModel)
	}
	if capturedReq.Dimensions != 3 {
		t.Errorf("request dimensions = %d, want 3", capturedReq.Dimensions)
	}
	if len(capturedReq.Input) != 1 {
		t.Fatalf("request input length = %d, want 1", len(capturedReq.Input))
	}
	if capturedReq.Input[0].Text != "test input" {
		t.Errorf("request input text = %q, want %q", capturedReq.Input[0].Text, "test input")
	}
	if capturedReq.Input[0].Image != "" {
		t.Errorf("request input image should be empty, got %q", capturedReq.Input[0].Image)
	}
}

func TestEmbed_Image(t *testing.T) {
	t.Parallel()

	var capturedReq apiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		resp := apiResponse{
			Data:  []apiEmbeddingData{{Embedding: []float32{0.4, 0.5, 0.6}, Index: 0}},
			Model: "jina-clip-v2",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, _ := New(Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		Dimensions: 3,
	})

	imgData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	result, err := eng.Embed(context.Background(), embed.ImageRequest(imgData, ""))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(result.Vector) != 3 {
		t.Errorf("vector length = %d, want 3", len(result.Vector))
	}

	if capturedReq.Input[0].Text != "" {
		t.Errorf("text should be empty for image request, got %q", capturedReq.Input[0].Text)
	}
	if !strings.HasPrefix(capturedReq.Input[0].Image, "data:image/jpeg;base64,") {
		t.Errorf("image should be base64 data URI, got %q", capturedReq.Input[0].Image)
	}
}

func TestEmbed_VideoError(t *testing.T) {
	t.Parallel()

	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = eng.Embed(context.Background(), embed.VideoRequest([]byte{0x00}, ""))
	if err == nil {
		t.Fatal("expected error for video content")
	}
	if !strings.Contains(err.Error(), "video content not supported") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "video content not supported")
	}
}

func TestEmbed_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	eng, _ := New(Config{
		APIKey:  "bad-key",
		BaseURL: srv.URL,
	})

	_, err := eng.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestEmbedBatch(t *testing.T) {
	t.Parallel()

	srv := newEmbedTestServer(t)
	defer srv.Close()

	eng, err := New(Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		Dimensions: 4,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	reqs := []embed.Request{
		embed.TextRequest("first", ""),
		embed.TextRequest("second", ""),
		embed.TextRequest("third", ""),
	}

	results, err := eng.EmbedBatch(context.Background(), reqs)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for i, r := range results {
		if len(r.Vector) != 4 {
			t.Errorf("result[%d] vector length = %d, want 4", i, len(r.Vector))
		}
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos() returned empty slice")
	}

	for i, info := range infos {
		if info.Name == "" {
			t.Errorf("infos[%d].Name is empty", i)
		}
		if info.Provider == "" {
			t.Errorf("infos[%d].Provider is empty", i)
		}
		if info.Capability == "" {
			t.Errorf("infos[%d].Capability is empty", i)
		}
		if info.DisplayName["en"] == "" {
			t.Errorf("infos[%d].DisplayName[en] is empty", i)
		}
		if info.Description["en"] == "" {
			t.Errorf("infos[%d].Description[en] is empty", i)
		}
	}
}
