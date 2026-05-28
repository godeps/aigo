package imggen

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// --- helpers ---

func promptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func promptWithImageGraph(prompt, imageURL string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": imageURL}},
	}
}

func noPromptGraph() workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{}},
	}
}

// --- IsMultimodalImageModel ---

func TestIsMultimodalImageModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"wan image model", "wan2.7-image", true},
		{"z-image prefix", "z-image-turbo", true},
		{"generic image model", "some-image-model", true},
		{"qwen-image matches", "qwen-image", true},
		{"video model excluded", "wan2.7-t2v", false},
		{"image-video excluded", "image-video-mix", false},
		{"unrelated model", "some-other-model", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsMultimodalImageModel(tt.model); got != tt.want {
				t.Fatalf("IsMultimodalImageModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// --- IsQwenImageModel ---

func TestIsQwenImageModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"exact match", "qwen-image", true},
		{"versioned", "qwen-image-v2", true},
		{"unrelated", "other-model", false},
		{"not prefix", "xqwen-image", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsQwenImageModel(tt.model); got != tt.want {
				t.Fatalf("IsQwenImageModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// --- RunMultimodalImageMulti ---

func TestRunMultimodalImageMulti_Success(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/services/aigc/multimodal-generation/generation" {
			t.Errorf("path = %s, want /services/aigc/multimodal-generation/generation", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong Authorization header")
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": []any{
								map[string]any{"type": "image", "image": "https://example.com/gen.png"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	url, items, err := RunMultimodalImageMulti(context.Background(), rt, "test-key", "wan2.7-image", promptGraph("a cat"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/gen.png" {
		t.Fatalf("url = %q, want https://example.com/gen.png", url)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Value != "https://example.com/gen.png" {
		t.Fatalf("items[0].Value = %q, want URL", items[0].Value)
	}
	if gotPayload["model"] != "wan2.7-image" {
		t.Fatalf("model = %v, want wan2.7-image", gotPayload["model"])
	}
}

func TestRunMultimodalImageMulti_WithImageInput(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": []any{
								map[string]any{"type": "image", "image": "https://example.com/result.png"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	g := promptWithImageGraph("edit this", "https://example.com/source.png")
	url, _, err := RunMultimodalImageMulti(context.Background(), rt, "key", "wan2.7-image", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/result.png" {
		t.Fatalf("url = %q, want result URL", url)
	}

	// Verify image URL was included in request content.
	input, _ := gotPayload["input"].(map[string]any)
	messages, _ := input["messages"].([]any)
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("len(content) = %d, want 2 (text + image)", len(content))
	}
}

func TestRunMultimodalImageMulti_WithParameters(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": []any{
								map[string]any{"type": "image", "image": "https://x.com/img.png"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a dog"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{
			"size":          "1024x1024",
			"watermark":     true,
			"prompt_extend": true,
			"thinking_mode": true,
			"n":             2,
		}},
	}
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := RunMultimodalImageMulti(context.Background(), rt, "key", "model", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params, _ := gotPayload["parameters"].(map[string]any)
	if params == nil {
		t.Fatal("expected parameters in payload")
	}
	// NormalizeSize converts "x" to "*".
	if params["size"] != "1024*1024" {
		t.Fatalf("size = %v, want 1024*1024", params["size"])
	}
	if params["watermark"] != true {
		t.Fatalf("watermark = %v, want true", params["watermark"])
	}
	if params["prompt_extend"] != true {
		t.Fatalf("prompt_extend = %v, want true", params["prompt_extend"])
	}
	if params["thinking_mode"] != true {
		t.Fatalf("thinking_mode = %v, want true", params["thinking_mode"])
	}
}

func TestRunMultimodalImageMulti_MultipleImages(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": []any{
								map[string]any{"type": "image", "image": "https://x.com/1.png"},
								map[string]any{"type": "image", "image": "https://x.com/2.png"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	url, items, err := RunMultimodalImageMulti(context.Background(), rt, "key", "model", promptGraph("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://x.com/1.png" {
		t.Fatalf("first url = %q, want https://x.com/1.png", url)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Metadata["index"] != "0" || items[1].Metadata["index"] != "1" {
		t.Fatalf("item indices = %v, %v; want 0, 1", items[0].Metadata["index"], items[1].Metadata["index"])
	}
}

func TestRunMultimodalImageMulti_MissingPrompt(t *testing.T) {
	t.Parallel()
	_, _, err := RunMultimodalImageMulti(context.Background(), nil, "", "", noPromptGraph())
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunMultimodalImageMulti_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"code":"InternalError","message":"server error"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := RunMultimodalImageMulti(context.Background(), rt, "key", "model", promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestRunMultimodalImageMulti_InvalidSizeError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"code":"InvalidParameter","message":"invalid size value"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := RunMultimodalImageMulti(context.Background(), rt, "key", "model", promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for invalid size")
	}
	if !strings.Contains(err.Error(), "supported image sizes") {
		t.Fatalf("expected error to contain supported sizes hint, got: %v", err)
	}
}

func TestRunMultimodalImageMulti_NoImageInResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"choices": []any{}},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, _, err := RunMultimodalImageMulti(context.Background(), rt, "key", "model", promptGraph("test"))
	if err == nil {
		t.Fatal("expected error when response has no image")
	}
	if !strings.Contains(err.Error(), "did not contain an image URL") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// --- RunMultimodalImage ---

func TestRunMultimodalImage_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": []any{
								map[string]any{"type": "image", "image": "https://example.com/first.png"},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	url, err := RunMultimodalImage(context.Background(), rt, "key", "model", promptGraph("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/first.png" {
		t.Fatalf("url = %q, want first image URL", url)
	}
}

// --- RunQwenImage ---

func TestRunQwenImage_NoWait(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	var gotAsync string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAsync = r.Header.Get("X-DashScope-Async")
		if r.URL.Path != "/services/aigc/text2image/image-synthesis" {
			t.Errorf("path = %s, want /services/aigc/text2image/image-synthesis", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"task_id": "task-abc-123"},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        http.DefaultClient,
		WaitForCompletion: false,
	}
	result, err := RunQwenImage(context.Background(), rt, "test-key", "qwen-image", promptGraph("a landscape"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "task-abc-123" {
		t.Fatalf("result = %q, want task-abc-123", result)
	}
	if gotAsync != "enable" {
		t.Fatalf("X-DashScope-Async = %q, want enable", gotAsync)
	}
	if gotPayload["model"] != "qwen-image" {
		t.Fatalf("model = %v, want qwen-image", gotPayload["model"])
	}
	input, _ := gotPayload["input"].(map[string]any)
	if input["prompt"] != "a landscape" {
		t.Fatalf("input.prompt = %v, want %q", input["prompt"], "a landscape")
	}
}

func TestRunQwenImage_WithOptions(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"task_id": "task-456"},
		})
	}))
	defer srv.Close()

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{
			"negative_prompt": "blurry",
			"size":            "512x512",
			"n":               4,
			"watermark":       false,
			"prompt_extend":   true,
			"seed":            42,
		}},
	}
	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        http.DefaultClient,
		WaitForCompletion: false,
	}
	_, err := RunQwenImage(context.Background(), rt, "key", "qwen-image", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["negative_prompt"] != "blurry" {
		t.Fatalf("input.negative_prompt = %v, want blurry", input["negative_prompt"])
	}

	params, _ := gotPayload["parameters"].(map[string]any)
	// NormalizeSize converts "x" to "*".
	if params["size"] != "512*512" {
		t.Fatalf("params.size = %v, want 512*512", params["size"])
	}
}

func TestRunQwenImage_MissingPrompt(t *testing.T) {
	t.Parallel()
	_, err := RunQwenImage(context.Background(), nil, "", "", noPromptGraph())
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunQwenImage_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"code":"InternalError","message":"server error"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        http.DefaultClient,
		WaitForCompletion: false,
	}
	_, err := RunQwenImage(context.Background(), rt, "key", "qwen-image", promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
