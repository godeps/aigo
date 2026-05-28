package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)


func TestNew_MissingKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != DefaultModel {
		t.Fatalf("model = %q, want %q", e.model, DefaultModel)
	}
	if e.dimensions != DefaultDimensions {
		t.Fatalf("dimensions = %d, want %d", e.dimensions, DefaultDimensions)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	t.Parallel()

	e, err := New(Config{
		APIKey:     "test-key",
		Model:      "custom-model",
		Dimensions: 512,
		RPM:        100,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != "custom-model" {
		t.Fatalf("model = %q, want %q", e.model, "custom-model")
	}
	if e.dimensions != 512 {
		t.Fatalf("dimensions = %d, want 512", e.dimensions)
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key", Dimensions: 256})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.Dimensions() != 256 {
		t.Fatalf("Dimensions() = %d, want 256", e.Dimensions())
	}
}

func TestEmbedCapabilities(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cap := e.EmbedCapabilities()

	if len(cap.SupportedTypes) != 3 {
		t.Fatalf("SupportedTypes = %v, want 3 types", cap.SupportedTypes)
	}
	if cap.MaxDimensions != 3072 {
		t.Fatalf("MaxDimensions = %d, want 3072", cap.MaxDimensions)
	}
	if !cap.SupportsMRL {
		t.Fatal("SupportsMRL should be true")
	}
	if len(cap.Models) != 1 || cap.Models[0] != DefaultModel {
		t.Fatalf("Models = %v", cap.Models)
	}
}

func TestBuildContent_Text(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	content := e.buildContent(embed.TextRequest("hello world", "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].Text != "hello world" {
		t.Fatalf("text = %q, want %q", content.Parts[0].Text, "hello world")
	}
}

func TestBuildContent_Image(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data := []byte{0xFF, 0xD8, 0xFF}
	content := e.buildContent(embed.ImageRequest(data, "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].InlineData == nil {
		t.Fatal("expected InlineData for image")
	}
	if content.Parts[0].InlineData.MIMEType != "image/jpeg" {
		t.Fatalf("mime = %q, want image/jpeg", content.Parts[0].InlineData.MIMEType)
	}
}

func TestBuildContent_Video(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data := []byte{0x00, 0x00, 0x00}
	content := e.buildContent(embed.VideoRequest(data, "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].InlineData == nil {
		t.Fatal("expected InlineData for video")
	}
	if content.Parts[0].InlineData.MIMEType != "video/mp4" {
		t.Fatalf("mime = %q, want video/mp4", content.Parts[0].InlineData.MIMEType)
	}
}

func TestEmbedBatch_Empty(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	results, err := e.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Mock HTTP transport helpers
// ---------------------------------------------------------------------------

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// jsonResp constructs an *http.Response with the given status code and
// JSON-encoded body.
func jsonResp(code int, v any) *http.Response {
	data, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

// goodEmbedResp returns a valid Gemini batchEmbedContents JSON response
// with one embedding of the given dimensionality.
func goodEmbedResp(dims int) map[string]any {
	vals := make([]float64, dims)
	for i := range vals {
		vals[i] = float64(i+1) * 0.01
	}
	return map[string]any{
		"embeddings": []map[string]any{
			{"values": vals},
		},
	}
}

// newMockEngine creates an Engine backed by a mock HTTP transport.
func newMockEngine(t *testing.T, dims int, fn roundTripFunc) *Engine {
	t.Helper()
	e, err := New(Config{
		APIKey:     "test-key",
		Dimensions: dims,
		RPM:        1000,
		HTTPClient: &http.Client{Transport: fn},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return e
}

// ---------------------------------------------------------------------------
// ModelInfos
// ---------------------------------------------------------------------------

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) != 1 {
		t.Fatalf("ModelInfos() returned %d entries, want 1", len(infos))
	}
	info := infos[0]
	if info.Name != "gemini-embedding-2-preview" {
		t.Errorf("Name = %q, want %q", info.Name, "gemini-embedding-2-preview")
	}
	if info.Provider != "embed/gemini" {
		t.Errorf("Provider = %q, want %q", info.Provider, "embed/gemini")
	}
	if info.Capability != "embedding" {
		t.Errorf("Capability = %q, want %q", info.Capability, "embedding")
	}
	if info.DisplayName["en"] == "" {
		t.Error("DisplayName[en] is empty")
	}
	if info.DisplayName["zh"] == "" {
		t.Error("DisplayName[zh] is empty")
	}
	if info.Description["en"] == "" {
		t.Error("Description[en] is empty")
	}
	if info.Description["zh"] == "" {
		t.Error("Description[zh] is empty")
	}
	if info.Intro["en"] == "" {
		t.Error("Intro[en] is empty")
	}
	if info.DocURL == "" {
		t.Error("DocURL is empty")
	}
}

// ---------------------------------------------------------------------------
// buildContent – default (unknown) type
// ---------------------------------------------------------------------------

func TestBuildContent_DefaultType(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := embed.Request{Content: "fallback text", Type: 99}
	content := e.buildContent(req)
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].Text != "fallback text" {
		t.Fatalf("text = %q, want %q", content.Parts[0].Text, "fallback text")
	}
}

// ---------------------------------------------------------------------------
// Embed – success paths
// ---------------------------------------------------------------------------

func TestEmbed_TextSuccess(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, goodEmbedResp(3)), nil
	})

	result, err := e.Embed(context.Background(), embed.TextRequest("hello world", ""))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
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
}

func TestEmbed_ImageSuccess(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 4, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, goodEmbedResp(4)), nil
	})

	imgData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	result, err := e.Embed(context.Background(), embed.ImageRequest(imgData, "RETRIEVAL_DOCUMENT"))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(result.Vector) != 4 {
		t.Errorf("vector length = %d, want 4", len(result.Vector))
	}
}

func TestEmbed_VideoSuccess(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 5, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, goodEmbedResp(5)), nil
	})

	vidData := []byte{0x00, 0x00, 0x00, 0x1C}
	result, err := e.Embed(context.Background(), embed.VideoRequest(vidData, "RETRIEVAL_DOCUMENT"))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(result.Vector) != 5 {
		t.Errorf("vector length = %d, want 5", len(result.Vector))
	}
}

func TestEmbed_CustomTaskType(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, goodEmbedResp(3)), nil
	})

	result, err := e.Embed(context.Background(), embed.TextRequest("query text", "RETRIEVAL_QUERY"))
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if result.Dimensions != 3 {
		t.Errorf("dimensions = %d, want 3", result.Dimensions)
	}
}

// ---------------------------------------------------------------------------
// Embed – error paths
// ---------------------------------------------------------------------------

func TestEmbed_EmptyEmbeddings(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		resp := map[string]any{"embeddings": []map[string]any{}}
		return jsonResp(http.StatusOK, resp), nil
	})

	_, err := e.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err == nil {
		t.Fatal("expected error for empty embeddings")
	}
	if !strings.Contains(err.Error(), "empty embedding") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "empty embedding")
	}
}

func TestEmbed_EmptyValues(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		resp := map[string]any{
			"embeddings": []map[string]any{
				{"values": []float64{}},
			},
		}
		return jsonResp(http.StatusOK, resp), nil
	})

	_, err := e.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err == nil {
		t.Fatal("expected error for empty values")
	}
	if !strings.Contains(err.Error(), "empty embedding") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "empty embedding")
	}
}

func TestEmbed_APIError(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"code":    401,
				"message": "invalid api key",
			},
		}), nil
	})

	_, err := e.Embed(context.Background(), embed.TextRequest("hello", ""))
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestEmbed_CancelledContext(t *testing.T) {
	t.Parallel()

	e, err := New(Config{
		APIKey:     "test-key",
		Dimensions: 3,
		RPM:        1,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusOK, goodEmbedResp(3)), nil
		})},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Exhaust the single rate-limit slot.
	if werr := e.limiter.Wait(context.Background()); werr != nil {
		t.Fatalf("limiter.Wait() error = %v", werr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = e.Embed(ctx, embed.TextRequest("hello", ""))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// EmbedBatch – success and partial-error paths
// ---------------------------------------------------------------------------

func TestEmbedBatch_MultipleItems(t *testing.T) {
	t.Parallel()

	e := newMockEngine(t, 4, func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, goodEmbedResp(4)), nil
	})

	reqs := []embed.Request{
		embed.TextRequest("first", ""),
		embed.TextRequest("second", ""),
		embed.TextRequest("third", ""),
	}
	results, err := e.EmbedBatch(context.Background(), reqs)
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
		if r.Model != DefaultModel {
			t.Errorf("result[%d] model = %q, want %q", i, r.Model, DefaultModel)
		}
	}
}

func TestEmbedBatch_ErrorMidBatch(t *testing.T) {
	t.Parallel()

	callCount := 0
	e := newMockEngine(t, 3, func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 2 {
			return jsonResp(http.StatusInternalServerError, map[string]any{
				"error": map[string]any{"code": 500, "message": "server error"},
			}), nil
		}
		return jsonResp(http.StatusOK, goodEmbedResp(3)), nil
	})

	reqs := []embed.Request{
		embed.TextRequest("first", ""),
		embed.TextRequest("second", ""),
		embed.TextRequest("third", ""),
	}
	results, err := e.EmbedBatch(context.Background(), reqs)
	if err == nil {
		t.Fatal("expected error for failed batch item")
	}
	if !strings.Contains(err.Error(), "batch item 1") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "batch item 1")
	}
	// First result should be valid.
	if len(results) != 1 {
		t.Fatalf("got %d results before error, want 1", len(results))
	}
	if len(results[0].Vector) != 3 {
		t.Errorf("result[0] vector length = %d, want 3", len(results[0].Vector))
	}
}
