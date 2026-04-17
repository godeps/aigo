package alibabacloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)

func newTestServer(t *testing.T, wantModel string, wantTextType string, dims int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}

		var req apiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if wantModel != "" && req.Model != wantModel {
			t.Errorf("model = %q, want %q", req.Model, wantModel)
		}
		if wantTextType != "" && req.Parameters.TextType != wantTextType {
			t.Errorf("text_type = %q, want %q", req.Parameters.TextType, wantTextType)
		}

		n := dims
		if n <= 0 {
			n = 4
		}
		embeddings := make([]apiEmbedding, len(req.Input.Texts))
		for i := range req.Input.Texts {
			vec := make([]float32, n)
			for j := range vec {
				vec[j] = float32(i+1) * 0.1
			}
			embeddings[i] = apiEmbedding{TextIndex: i, Embedding: vec}
		}

		resp := apiResponse{
			Output: apiOutput{Embeddings: embeddings},
			Usage:  apiUsage{TotalTokens: len(req.Input.Texts) * 10},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestEmbed(t *testing.T) {
	srv := newTestServer(t, DefaultModel, "query", 4)
	defer srv.Close()

	eng, err := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := eng.Embed(context.Background(), embed.TextRequest("hello world", "RETRIEVAL_QUERY"))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Vector) != 4 {
		t.Errorf("vector length = %d, want 4", len(result.Vector))
	}
	if result.Model != DefaultModel {
		t.Errorf("model = %q, want %q", result.Model, DefaultModel)
	}
}

func TestEmbed_Document(t *testing.T) {
	srv := newTestServer(t, DefaultModel, "document", 4)
	defer srv.Close()

	eng, err := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := eng.Embed(context.Background(), embed.TextRequest("a document", "RETRIEVAL_DOCUMENT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Vector) == 0 {
		t.Error("expected non-empty vector")
	}
}

func TestEmbedBatch(t *testing.T) {
	srv := newTestServer(t, "", "", 4)
	defer srv.Close()

	eng, err := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	reqs := []embed.Request{
		embed.TextRequest("first", "RETRIEVAL_DOCUMENT"),
		embed.TextRequest("second", "RETRIEVAL_DOCUMENT"),
		embed.TextRequest("third", "RETRIEVAL_DOCUMENT"),
	}

	results, err := eng.EmbedBatch(context.Background(), reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestEmbed_RejectsNonText(t *testing.T) {
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.VideoRequest([]byte{0}, ""))
	if err == nil {
		t.Error("expected error for non-text content")
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")
	_, err := New(Config{})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestDefaultDimensions(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"text-embedding-v3", 1024},
		{"text-embedding-v2", 1536},
		{"text-embedding-v1", 1536},
		{"unknown", DefaultDimensions},
	}
	for _, tt := range tests {
		if got := defaultDimensions(tt.model); got != tt.want {
			t.Errorf("defaultDimensions(%q) = %d, want %d", tt.model, got, tt.want)
		}
	}
}

func TestEmbedCapabilities(t *testing.T) {
	eng, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	cap := eng.EmbedCapabilities()
	if len(cap.SupportedTypes) != 1 || cap.SupportedTypes[0] != embed.ContentText {
		t.Errorf("unexpected supported types: %v", cap.SupportedTypes)
	}
	if !cap.SupportsMRL {
		t.Error("text-embedding-v3 should support MRL")
	}

	eng2, _ := New(Config{APIKey: "test-key", Model: "text-embedding-v2"})
	cap2 := eng2.EmbedCapabilities()
	if cap2.SupportsMRL {
		t.Error("text-embedding-v2 should not support MRL")
	}
}
