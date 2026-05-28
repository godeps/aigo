package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)

func TestNew_Defaults(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if eng.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", eng.baseURL, defaultBaseURL)
	}
	if eng.model != DefaultModel {
		t.Errorf("model = %q, want %q", eng.model, DefaultModel)
	}
	if eng.dimensions != DefaultDimensions {
		t.Errorf("dimensions = %d, want %d", eng.dimensions, DefaultDimensions)
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := New(Config{})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{
		APIKey:     "custom-key",
		BaseURL:    "https://custom.api.com",
		Model:      "text-embedding-3-large",
		Dimensions: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if eng.baseURL != "https://custom.api.com" {
		t.Errorf("baseURL = %q, want %q", eng.baseURL, "https://custom.api.com")
	}
	if eng.model != "text-embedding-3-large" {
		t.Errorf("model = %q, want %q", eng.model, "text-embedding-3-large")
	}
	if eng.dimensions != 256 {
		t.Errorf("dimensions = %d, want %d", eng.dimensions, 256)
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key", Dimensions: 512})
	if err != nil {
		t.Fatal(err)
	}
	if got := eng.Dimensions(); got != 512 {
		t.Errorf("Dimensions() = %d, want 512", got)
	}
}

func TestEmbedCapabilities(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	cap := eng.EmbedCapabilities()
	if len(cap.SupportedTypes) != 1 || cap.SupportedTypes[0] != embed.ContentText {
		t.Errorf("unexpected supported types: %v", cap.SupportedTypes)
	}
	if !cap.SupportsMRL {
		t.Error("expected SupportsMRL to be true")
	}
	if cap.MaxDimensions != 3072 {
		t.Errorf("MaxDimensions = %d, want 3072", cap.MaxDimensions)
	}
	if len(cap.Models) != 3 {
		t.Errorf("len(Models) = %d, want 3", len(cap.Models))
	}
}

func TestEmbed_Text(t *testing.T) {
	t.Parallel()

	var capturedReq apiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedReq)

		resp := apiResponse{
			Data:  []apiEmbeddingData{{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0}},
			Model: DefaultModel,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	result, err := eng.Embed(context.Background(), embed.TextRequest("hello world", ""))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Vector) != 3 {
		t.Errorf("vector length = %d, want 3", len(result.Vector))
	}
	if result.Model != DefaultModel {
		t.Errorf("model = %q, want %q", result.Model, DefaultModel)
	}
	if result.Dimensions != 3 {
		t.Errorf("dimensions = %d, want 3", result.Dimensions)
	}

	// Verify request body fields.
	if capturedReq.Model != DefaultModel {
		t.Errorf("request model = %q, want %q", capturedReq.Model, DefaultModel)
	}
	if capturedReq.Input != "hello world" {
		t.Errorf("request input = %q, want %q", capturedReq.Input, "hello world")
	}
	if capturedReq.Dimensions != DefaultDimensions {
		t.Errorf("request dimensions = %d, want %d", capturedReq.Dimensions, DefaultDimensions)
	}
}

func TestEmbed_NonTextError(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = eng.Embed(context.Background(), embed.ImageRequest([]byte{1, 2, 3}, ""))
	if err == nil {
		t.Error("expected error for non-text content")
	}
}

func TestEmbed_EmptyTextError(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = eng.Embed(context.Background(), embed.TextRequest("", ""))
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestEmbed_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	eng, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestEmbedBatch(t *testing.T) {
	t.Parallel()

	var capturedReq apiBatchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedReq)

		resp := apiResponse{
			Data: []apiEmbeddingData{
				{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
				{Embedding: []float32{0.4, 0.5, 0.6}, Index: 1},
			},
			Model: DefaultModel,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	reqs := []embed.Request{
		embed.TextRequest("text1", ""),
		embed.TextRequest("text2", ""),
	}
	results, err := eng.EmbedBatch(context.Background(), reqs)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Verify batch request has array input.
	if len(capturedReq.Input) != 2 {
		t.Fatalf("input length = %d, want 2", len(capturedReq.Input))
	}
	if capturedReq.Input[0] != "text1" || capturedReq.Input[1] != "text2" {
		t.Errorf("input = %v, want [text1 text2]", capturedReq.Input)
	}
}

func TestEmbedBatch_NonTextError(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	reqs := []embed.Request{
		embed.TextRequest("text1", ""),
		embed.ImageRequest([]byte{1}, ""),
	}
	_, err = eng.EmbedBatch(context.Background(), reqs)
	if err == nil {
		t.Error("expected error for non-text item in batch")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("expected non-empty model infos")
	}
	for _, info := range infos {
		if info.Name == "" {
			t.Error("model name is empty")
		}
		if info.Provider == "" {
			t.Error("provider is empty")
		}
		if info.Capability != "embedding" {
			t.Errorf("capability = %q, want %q", info.Capability, "embedding")
		}
	}
}
