package alibabacloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)

func newMultimodalTestServer(t *testing.T, wantModel string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}

		var req mmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if wantModel != "" && req.Model != wantModel {
			t.Errorf("model = %q, want %q", req.Model, wantModel)
		}

		if len(req.Input.Contents) == 0 {
			http.Error(w, "empty contents", http.StatusBadRequest)
			return
		}

		content := req.Input.Contents[0]
		embType := "text"
		if content.Image != "" {
			embType = "image"
		}

		vec := make([]float32, 4)
		for i := range vec {
			vec[i] = 0.1 * float32(i+1)
		}

		resp := mmResponse{
			Output: mmOutput{
				Embeddings: []mmEmbedding{
					{Index: 0, Embedding: vec, Type: embType},
				},
			},
			Usage: apiUsage{TotalTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestMultimodalEmbed_Text(t *testing.T) {
	srv := newMultimodalTestServer(t, MultimodalDefaultModel)
	defer srv.Close()

	eng, err := NewMultimodal(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := eng.Embed(context.Background(), embed.TextRequest("hello world", ""))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Vector) != 4 {
		t.Errorf("vector length = %d, want 4", len(result.Vector))
	}
	if result.Model != MultimodalDefaultModel {
		t.Errorf("model = %q, want %q", result.Model, MultimodalDefaultModel)
	}
}

func TestMultimodalEmbed_Image(t *testing.T) {
	srv := newMultimodalTestServer(t, MultimodalDefaultModel)
	defer srv.Close()

	eng, err := NewMultimodal(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	imgData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake JPEG header
	result, err := eng.Embed(context.Background(), embed.ImageRequest(imgData, ""))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Vector) != 4 {
		t.Errorf("vector length = %d, want 4", len(result.Vector))
	}
}

func TestMultimodalEmbed_ImageBase64(t *testing.T) {
	var captured mmRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		vec := make([]float32, 4)
		resp := mmResponse{
			Output: mmOutput{Embeddings: []mmEmbedding{{Embedding: vec, Type: "image"}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, _ := NewMultimodal(Config{APIKey: "test-key", BaseURL: srv.URL})
	eng.Embed(context.Background(), embed.ImageRequest([]byte{0x01, 0x02}, ""))

	if len(captured.Input.Contents) != 1 {
		t.Fatal("expected 1 content item")
	}
	if !strings.HasPrefix(captured.Input.Contents[0].Image, "data:image/jpeg;base64,") {
		t.Errorf("image should be base64 data URI, got %q", captured.Input.Contents[0].Image)
	}
}

func TestMultimodalEmbedBatch(t *testing.T) {
	srv := newMultimodalTestServer(t, "")
	defer srv.Close()

	eng, err := NewMultimodal(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	reqs := []embed.Request{
		embed.TextRequest("first", ""),
		embed.ImageRequest([]byte{0xFF}, ""),
		embed.TextRequest("third", ""),
	}

	results, err := eng.EmbedBatch(context.Background(), reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestMultimodalEmbed_RejectsVideo(t *testing.T) {
	eng, err := NewMultimodal(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.VideoRequest([]byte{0}, ""))
	if err == nil {
		t.Error("expected error for video content")
	}
}

func TestMultimodalEmbed_RejectsEmptyText(t *testing.T) {
	eng, err := NewMultimodal(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.TextRequest("", ""))
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestMultimodalEmbed_RejectsEmptyImage(t *testing.T) {
	eng, err := NewMultimodal(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Embed(context.Background(), embed.ImageRequest([]byte{}, ""))
	if err == nil {
		t.Error("expected error for empty image")
	}
}

func TestNewMultimodal_MissingAPIKey(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")
	_, err := NewMultimodal(Config{})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestMultimodalCapabilities(t *testing.T) {
	eng, err := NewMultimodal(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	cap := eng.EmbedCapabilities()
	if len(cap.SupportedTypes) != 2 {
		t.Fatalf("expected 2 supported types, got %d", len(cap.SupportedTypes))
	}
	hasText, hasImage := false, false
	for _, ct := range cap.SupportedTypes {
		if ct == embed.ContentText {
			hasText = true
		}
		if ct == embed.ContentImage {
			hasImage = true
		}
	}
	if !hasText || !hasImage {
		t.Errorf("expected text and image support, got %v", cap.SupportedTypes)
	}
	if cap.SupportsMRL {
		t.Error("multimodal-embedding-one-peace-v1 should not support MRL")
	}
}
