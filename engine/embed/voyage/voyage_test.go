package voyage

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
	t.Setenv("VOYAGE_API_KEY", "")
	_, err := New(Config{})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{
		APIKey:     "custom-key",
		BaseURL:    "https://custom.voyageai.com",
		Model:      "voyage-3-lite",
		Dimensions: 512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if eng.baseURL != "https://custom.voyageai.com" {
		t.Errorf("baseURL = %q, want %q", eng.baseURL, "https://custom.voyageai.com")
	}
	if eng.model != "voyage-3-lite" {
		t.Errorf("model = %q, want %q", eng.model, "voyage-3-lite")
	}
	if eng.dimensions != 512 {
		t.Errorf("dimensions = %d, want %d", eng.dimensions, 512)
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()
	eng, err := New(Config{APIKey: "test-key", Dimensions: 768})
	if err != nil {
		t.Fatal(err)
	}
	if got := eng.Dimensions(); got != 768 {
		t.Errorf("Dimensions() = %d, want 768", got)
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
	if cap.SupportsMRL {
		t.Error("expected SupportsMRL to be false")
	}
	if cap.MaxDimensions != 1024 {
		t.Errorf("MaxDimensions = %d, want 1024", cap.MaxDimensions)
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

	// Verify input_type defaults to "document".
	if capturedReq.InputType != "document" {
		t.Errorf("input_type = %q, want %q", capturedReq.InputType, "document")
	}
	// Default dimensions should NOT send output_dimension.
	if capturedReq.OutputDimension != 0 {
		t.Errorf("output_dimension = %d, want 0 (omitted)", capturedReq.OutputDimension)
	}
}

func TestEmbed_QueryType(t *testing.T) {
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

	_, err = eng.Embed(context.Background(), embed.TextRequest("search query", "RETRIEVAL_QUERY"))
	if err != nil {
		t.Fatal(err)
	}

	if capturedReq.InputType != "query" {
		t.Errorf("input_type = %q, want %q", capturedReq.InputType, "query")
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

	var capturedReq apiRequest
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

	// Verify all texts sent in a single request.
	if len(capturedReq.Input) != 2 {
		t.Fatalf("input length = %d, want 2", len(capturedReq.Input))
	}
	if capturedReq.Input[0] != "text1" || capturedReq.Input[1] != "text2" {
		t.Errorf("input = %v, want [text1 text2]", capturedReq.Input)
	}
}

func TestEmbed_CustomDimensions(t *testing.T) {
	t.Parallel()

	var capturedReq apiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedReq)

		resp := apiResponse{
			Data:  []apiEmbeddingData{{Embedding: []float32{0.1, 0.2}, Index: 0}},
			Model: DefaultModel,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, err := New(Config{APIKey: "test-key", BaseURL: srv.URL, Dimensions: 512})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err != nil {
		t.Fatal(err)
	}

	// When dimensions != DefaultDimensions, output_dimension must be sent.
	if capturedReq.OutputDimension != 512 {
		t.Errorf("output_dimension = %d, want 512", capturedReq.OutputDimension)
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
