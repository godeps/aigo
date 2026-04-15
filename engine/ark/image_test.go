package ark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestRunImageGeneration(t *testing.T) {
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
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != imagesPath {
						t.Fatalf("expected path %s, got %s", imagesPath, r.URL.Path)
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Fatal("missing auth header")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != imagesPath {
			t.Fatalf("expected images path, got %s", r.URL.Path)
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
	// Ensure video models still route to the async content generation path.
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tasksPath:
			w.WriteHeader(200)
			w.Write([]byte(`{"id":"cgt-img-test"}`))
		case r.Method == http.MethodGet:
			calls++
			if calls < 2 {
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
	e := New(Config{APIKey: "key", Model: ModelSeedream3_0})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("expected image media type, got %v", cap.MediaTypes)
	}
}
