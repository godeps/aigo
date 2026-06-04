package engine_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	_ "github.com/godeps/aigo/engine/alibabacloud"
	"github.com/godeps/aigo/engine/httpx"
	_ "github.com/godeps/aigo/engine/newapi"
	_ "github.com/godeps/aigo/engine/openai"
	_ "github.com/godeps/aigo/engine/qwenvl"
	"github.com/godeps/aigo/workflow"
)

type providerHookTestHeader struct{}

func (providerHookTestHeader) BeforeRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("X-Hook-Test", "ok")
	return clone, nil
}

func TestProviderFactoriesPassHTTPHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		model    string
		baseURL  func(string) string
		handler  func(*testing.T, *bool) http.HandlerFunc
		graph    workflow.Graph
		wait     bool
	}{
		{
			name:     "openai",
			provider: "openai",
			model:    "dall-e-3",
			baseURL:  func(url string) string { return url },
			handler: func(t *testing.T, sawHook *bool) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Hook-Test") == "ok" {
						*sawHook = true
					}
					if r.URL.Path != "/images/generations" {
						t.Errorf("path = %q, want /images/generations", r.URL.Path)
						http.Error(w, "bad path", http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/image.png"}]}`))
				}
			},
			graph: promptGraph("test image"),
		},
		{
			name:     "newapi",
			provider: "newapi",
			model:    "dall-e-3",
			baseURL:  func(url string) string { return url },
			handler: func(t *testing.T, sawHook *bool) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Hook-Test") == "ok" {
						*sawHook = true
					}
					if r.URL.Path != "/v1/images/generations" {
						t.Errorf("path = %q, want /v1/images/generations", r.URL.Path)
						http.Error(w, "bad path", http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/newapi.png"}]}`))
				}
			},
			graph: promptGraph("test image"),
		},
		{
			name:     "qwenvl",
			provider: "qwenvl",
			model:    "qwen-vl-max",
			baseURL:  func(url string) string { return url },
			handler: func(t *testing.T, sawHook *bool) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Hook-Test") == "ok" {
						*sawHook = true
					}
					if r.URL.Path != "/chat/completions" {
						t.Errorf("path = %q, want /chat/completions", r.URL.Path)
						http.Error(w, "bad path", http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
				}
			},
			graph: promptGraph("describe"),
		},
		{
			name:     "alibabacloud",
			provider: "alibabacloud",
			model:    "qwen-image",
			baseURL:  func(url string) string { return url + "/api/v1" },
			handler: func(t *testing.T, sawHook *bool) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Hook-Test") == "ok" {
						*sawHook = true
					}
					switch r.URL.Path {
					case "/api/v1/services/aigc/text2image/image-synthesis":
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"output":{"task_id":"task-1","task_status":"PENDING"}}`))
					case "/api/v1/tasks/task-1":
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"output":{"task_status":"SUCCEEDED","results":[{"url":"https://cdn.example.com/ali.png"}]}}`))
					default:
						t.Errorf("unexpected path %q", r.URL.Path)
						http.Error(w, "bad path", http.StatusInternalServerError)
					}
				}
			},
			graph: promptGraph("test image"),
			wait:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sawHook bool
			srv := httptest.NewServer(tt.handler(t, &sawHook))
			defer srv.Close()

			factory, ok := engine.GetFactory(tt.provider)
			if !ok {
				t.Fatalf("factory %q not registered", tt.provider)
			}
			cfg := engine.EngineConfig{
				APIKey:          "test-key",
				BaseURL:         tt.baseURL(srv.URL),
				Model:           tt.model,
				HTTPHookOptions: []httpx.HookOption{httpx.WithRequestHooks(providerHookTestHeader{})},
				PollInterval:    time.Millisecond,
			}
			if tt.wait {
				v := true
				cfg.WaitForCompletion = &v
			}
			eng, err := factory(cfg)
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			if _, err := eng.Execute(context.Background(), tt.graph); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !sawHook {
				t.Fatal("provider request did not include hook-injected header")
			}
		})
	}
}

func promptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func TestProviderFactoriesUseHookedHTTPClient(t *testing.T) {
	t.Parallel()

	providers, err := filepath.Glob(filepath.Join("..", "engine", "*", "provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Fatal("no provider.go files found")
	}
	for _, path := range providers {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "cfg.ClientWithHooks()") {
				t.Fatalf("%s must pass cfg.ClientWithHooks() into provider engine config", path)
			}
		})
	}
}
