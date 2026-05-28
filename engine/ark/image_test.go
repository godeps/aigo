package ark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestRunImageGeneration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		graph    workflow.Graph
		respBody string
		wantURL  string
		wantErr  bool
	}{
		{
			name: "success with url",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "a cat"}},
			},
			respBody: `{"data":[{"url":"https://img.example.com/cat.png"}]}`,
			wantURL:  "https://img.example.com/cat.png",
		},
		{
			name: "success with b64_json",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"prompt":          "a dog",
					"response_format": "b64_json",
				}},
			},
			respBody: `{"data":[{"b64_json":"abc123"}]}`,
			wantURL:  "data:image/png;base64,abc123",
		},
		{
			name: "api error",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "test"}},
			},
			respBody: `{"error":{"code":"invalid_model","message":"model not found"}}`,
			wantErr:  true,
		},
		{
			name: "empty data",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "test"}},
			},
			respBody: `{"data":[]}`,
			wantErr:  true,
		},
		{
			name:    "missing prompt",
			graph:   workflow.Graph{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != imagesPath {
						t.Errorf("expected path %s, got %s", imagesPath, r.URL.Path)
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Error("missing auth header")
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{
				httpClient: http.DefaultClient,
				model:      ModelSeedream3_0,
			}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runImageGeneration(context.Background(), e, "test-key", tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantURL {
				t.Fatalf("expected %q, got %q", tt.wantURL, result)
			}
		})
	}
}

func TestRunImageGeneration_RequestFormat(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[{"url":"https://img.example.com/out.png"}]}`))
	}))
	defer srv.Close()

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient, model: ModelSeedream3_0}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt": "a sunset",
			"size":   "1024x1024",
		}},
	}

	_, err := runImageGeneration(context.Background(), e, "key", graph)
	if err != nil {
		t.Fatal(err)
	}

	if gotPayload["model"] != ModelSeedream3_0 {
		t.Fatalf("expected model %s, got %v", ModelSeedream3_0, gotPayload["model"])
	}
	if gotPayload["prompt"] != "a sunset" {
		t.Fatalf("expected prompt 'a sunset', got %v", gotPayload["prompt"])
	}
	if gotPayload["size"] != "1024x1024" {
		t.Fatalf("expected size 1024x1024, got %v", gotPayload["size"])
	}
}

func TestExecuteImageModel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != imagesPath {
			t.Errorf("expected images path, got %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[{"url":"https://img.example.com/result.png"}]}`))
	}))
	defer srv.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   ModelSeedream3_0,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a mountain"}},
	}
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "https://img.example.com/result.png" {
		t.Fatalf("expected URL, got %q", result.Value)
	}
}

func TestExecuteVideoModelStillWorks(t *testing.T) {
	t.Parallel()
	// Ensure video models still route to the async content generation path.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tasksPath:
			w.WriteHeader(200)
			w.Write([]byte(`{"id":"cgt-img-test"}`))
		case r.Method == http.MethodGet:
			calls.Add(1)
			if calls.Load() < 2 {
				w.Write([]byte(`{"id":"cgt-img-test","status":"running"}`))
			} else {
				w.Write([]byte(`{"id":"cgt-img-test","status":"succeeded","content":{"video_url":"https://v.example.com/out.mp4"}}`))
			}
		}
	}))
	defer srv.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           srv.URL,
		Model:             "doubao-seedance-2-0-260128",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "video test"}},
	}
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "https://v.example.com/out.mp4" {
		t.Fatalf("expected video URL, got %q", result.Value)
	}
}

func TestModelsByCapabilityIncludesImage(t *testing.T) {
	t.Parallel()
	caps := ModelsByCapability()
	if len(caps["image"]) == 0 {
		t.Fatal("expected image models")
	}
	found := false
	for _, m := range caps["image"] {
		if m == ModelSeedream3_0 {
			found = true
		}
	}
	if !found {
		t.Fatal("missing ModelSeedream3_0 in image cap")
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "key", Model: ModelSeedream3_0})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("expected image media type, got %v", cap.MediaTypes)
	}
}

func TestExtractImageResultB64Fallback(t *testing.T) {
	t.Parallel()

	// When format is "url" but only b64_json is available, use b64_json.
	body := []byte(`{"data":[{"b64_json":"fallback123"}]}`)
	result, err := extractImageResult(body, "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "data:image/png;base64,fallback123" {
		t.Fatalf("expected b64 fallback, got %q", result)
	}
}

func TestExtractImageResultNoURLOrB64(t *testing.T) {
	t.Parallel()

	body := []byte(`{"data":[{}]}`)
	_, err := extractImageResult(body, "url")
	if err == nil {
		t.Fatal("expected error for no url or b64_json")
	}
	if !strings.Contains(err.Error(), "no url or b64_json") {
		t.Fatalf("expected no url or b64_json error, got: %v", err)
	}
}

func TestExtractImageResultInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := extractImageResult([]byte(`not json`), "url")
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
	if !strings.Contains(err.Error(), "decode image response") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestRunImageGenerationWithOptionalParams(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[{"url":"https://img.example.com/out.png"}]}`))
	}))
	defer srv.Close()

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient, model: ModelSeedream3_0}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt":          "a mountain",
			"size":            "512x512",
			"seed":            99,
			"watermark":       true,
			"optimize_prompt": true,
			"image":           "https://example.com/ref.jpg",
			"guidance_scale":  "7.5",
		}},
	}

	_, err := runImageGeneration(context.Background(), e, "key", graph)
	if err != nil {
		t.Fatal(err)
	}

	if gotPayload["seed"] == nil {
		t.Fatal("expected seed in payload")
	}
	if gotPayload["watermark"] != true {
		t.Fatalf("expected watermark=true, got %v", gotPayload["watermark"])
	}
	if gotPayload["optimize_prompt"] != true {
		t.Fatalf("expected optimize_prompt=true, got %v", gotPayload["optimize_prompt"])
	}
	if gotPayload["image"] != "https://example.com/ref.jpg" {
		t.Fatalf("expected image URL, got %v", gotPayload["image"])
	}
	if gotPayload["guidance_scale"] != "7.5" {
		t.Fatalf("expected guidance_scale=7.5, got %v", gotPayload["guidance_scale"])
	}
}

func TestExecuteImageModelSeedream21(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != imagesPath {
			t.Errorf("expected images path, got %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[{"url":"https://img.example.com/s21.png"}]}`))
	}))
	defer srv.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   ModelSeedream2_1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a forest"}},
	}
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "https://img.example.com/s21.png" {
		t.Fatalf("expected URL, got %q", result.Value)
	}
}

func TestCapabilitiesImageSync(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "key", Model: ModelSeedream3_0})
	cap := e.Capabilities()
	if !cap.SupportsSync {
		t.Fatal("image model should support sync")
	}
	if cap.SupportsPoll {
		t.Fatal("image model should not support poll")
	}
}

func TestModelsByCapabilitySeedream21(t *testing.T) {
	t.Parallel()

	caps := ModelsByCapability()
	found := false
	for _, m := range caps["image"] {
		if m == ModelSeedream2_1 {
			found = true
		}
	}
	if !found {
		t.Fatal("missing ModelSeedream2_1 in image capabilities")
	}
}
