package comfyui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

// ---------------------------------------------------------------------------
// Capabilities (0% → 100%)
// ---------------------------------------------------------------------------

func TestCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		wait         bool
		wantSync     bool
		wantPoll     bool
		wantMediaLen int
	}{
		{name: "sync mode", wait: false, wantSync: true, wantPoll: false, wantMediaLen: 2},
		{name: "poll mode", wait: true, wantSync: false, wantPoll: true, wantMediaLen: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New(Config{WaitForCompletion: tt.wait})
			cap := e.Capabilities()

			if cap.SupportsSync != tt.wantSync {
				t.Fatalf("SupportsSync = %v, want %v", cap.SupportsSync, tt.wantSync)
			}
			if cap.SupportsPoll != tt.wantPoll {
				t.Fatalf("SupportsPoll = %v, want %v", cap.SupportsPoll, tt.wantPoll)
			}
			if len(cap.MediaTypes) != tt.wantMediaLen {
				t.Fatalf("MediaTypes length = %d, want %d", len(cap.MediaTypes), tt.wantMediaLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ModelsByCapability (0% → 100%)
// ---------------------------------------------------------------------------

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	for _, key := range []string{"image", "video"} {
		models, ok := m[key]
		if !ok {
			t.Fatalf("missing key %q", key)
		}
		if len(models) == 0 {
			t.Fatalf("key %q has no models", key)
		}
	}
}

// ---------------------------------------------------------------------------
// ConfigSchema (0% → 100%)
// ---------------------------------------------------------------------------

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("ConfigSchema() returned %d fields, want >= 2", len(fields))
	}

	keys := map[string]bool{}
	for _, f := range fields {
		keys[f.Key] = true
	}
	for _, want := range []string{"baseUrl", "apiKey"} {
		if !keys[want] {
			t.Fatalf("ConfigSchema() missing key %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultProvider (0% → 100%)
// ---------------------------------------------------------------------------

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "comfyui" {
		t.Fatalf("Provider.Name = %q, want %q", p.Name, "comfyui")
	}
	if len(p.Configs) == 0 {
		t.Fatal("Provider.Configs is empty")
	}
	cfg := p.Configs[0]
	if cfg.Name != "comfyui" {
		t.Fatalf("ProviderConfig.Name = %q, want %q", cfg.Name, "comfyui")
	}
	if cfg.Engine == nil {
		t.Fatal("ProviderConfig.Engine is nil")
	}
}

// ---------------------------------------------------------------------------
// Execute – error paths (68.6% → higher)
// ---------------------------------------------------------------------------

func TestExecute_EmptyBaseURL(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	_, err := e.Execute(context.Background(), workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
}

func TestExecute_InvalidGraph(t *testing.T) {
	t.Parallel()

	e := New(Config{BaseURL: "http://localhost"})
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

func TestExecute_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	e := New(Config{BaseURL: server.URL})
	_, err := e.Execute(context.Background(), workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestExecute_EmptyPromptID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prompt_id":""}`))
	}))
	defer server.Close()

	e := New(Config{BaseURL: server.URL})
	_, err := e.Execute(context.Background(), workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for empty prompt_id")
	}
}

func TestExecute_InvalidResponseJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	e := New(Config{BaseURL: server.URL})
	_, err := e.Execute(context.Background(), workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// ---------------------------------------------------------------------------
// Execute – waitForCompletion with no output URL falls back to prompt ID
// ---------------------------------------------------------------------------

func TestExecute_WaitNoOutputURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"p-empty"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/p-empty":
			w.Header().Set("Content-Type", "application/json")
			// History item with no images/gifs/videos → empty firstOutputURL
			_, _ = w.Write([]byte(`{"outputs":{"9":{}}}`))
		}
	}))
	defer server.Close()

	e := New(Config{
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := e.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "p-empty" {
		t.Fatalf("Execute() = %q, want %q", got.Value, "p-empty")
	}
	if got.Kind != engine.OutputPlainText {
		t.Fatalf("Execute().Kind = %v, want OutputPlainText", got.Kind)
	}
}

// ---------------------------------------------------------------------------
// fetchResult – HTTP error from history endpoint
// ---------------------------------------------------------------------------

func TestExecute_WaitHistoryHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"p-fail"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/p-fail":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
		}
	}))
	defer server.Close()

	e := New(Config{
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := e.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for history HTTP error")
	}
}

// ---------------------------------------------------------------------------
// fetchResult – 404 then success (covers the 404 early-return)
// ---------------------------------------------------------------------------

func TestExecute_WaitHistory404ThenSuccess(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"p-404"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/p-404":
			callCount.Add(1)
			if callCount.Load() == 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"outputs":{"1":{"images":[{"filename":"out.png","type":"output"}]}}}`))
		}
	}))
	defer server.Close()

	e := New(Config{
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := e.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Kind != engine.OutputURL {
		t.Fatalf("Execute().Kind = %v, want OutputURL", got.Kind)
	}
}

// ---------------------------------------------------------------------------
// decodeHistoryItem – direct format, wrapped with missing key, invalid JSON
// ---------------------------------------------------------------------------

func TestDecodeHistoryItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		promptID string
		wantOK   bool
		wantErr  bool
	}{
		{
			name:     "direct format",
			body:     `{"outputs":{"1":{"images":[{"filename":"a.png"}]}}}`,
			promptID: "any",
			wantOK:   true,
		},
		{
			name:     "wrapped format hit",
			body:     `{"pid-1":{"outputs":{"1":{"images":[{"filename":"b.png"}]}}}}`,
			promptID: "pid-1",
			wantOK:   true,
		},
		{
			name:     "wrapped format miss",
			body:     `{"pid-1":{"outputs":{"1":{"images":[{"filename":"c.png"}]}}}}`,
			promptID: "pid-other",
			wantOK:   false,
		},
		{
			name:     "invalid JSON",
			body:     `<<<not json>>>`,
			promptID: "x",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item, ok, err := decodeHistoryItem([]byte(tt.body), tt.promptID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && item.Outputs == nil {
				t.Fatal("expected non-nil outputs")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// firstOutputURL – various asset types, empty outputs, empty filename
// ---------------------------------------------------------------------------

func TestFirstOutputURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    historyItem
		wantURL bool
	}{
		{
			name: "image asset",
			item: historyItem{Outputs: map[string]historyOutputs{
				"1": {Images: []historyAsset{{Filename: "a.png", Type: "output"}}},
			}},
			wantURL: true,
		},
		{
			name: "gif asset",
			item: historyItem{Outputs: map[string]historyOutputs{
				"1": {GIFs: []historyAsset{{Filename: "a.gif"}}},
			}},
			wantURL: true,
		},
		{
			name: "video asset",
			item: historyItem{Outputs: map[string]historyOutputs{
				"1": {Videos: []historyAsset{{Filename: "a.mp4"}}},
			}},
			wantURL: true,
		},
		{
			name: "empty filename skipped",
			item: historyItem{Outputs: map[string]historyOutputs{
				"1": {Images: []historyAsset{{Filename: ""}}},
			}},
			wantURL: false,
		},
		{
			name:    "no outputs",
			item:    historyItem{Outputs: map[string]historyOutputs{}},
			wantURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstOutputURL("http://host", tt.item)
			if tt.wantURL && got == "" {
				t.Fatal("expected non-empty URL")
			}
			if !tt.wantURL && got != "" {
				t.Fatalf("expected empty URL, got %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildViewURL – subfolder and type are optional
// ---------------------------------------------------------------------------

func TestBuildViewURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		asset historyAsset
		want  string
	}{
		{
			name:  "all fields",
			asset: historyAsset{Filename: "out.png", Subfolder: "sub", Type: "output"},
			want:  "http://host/view?filename=out.png&subfolder=sub&type=output",
		},
		{
			name:  "filename only",
			asset: historyAsset{Filename: "out.png"},
			want:  "http://host/view?filename=out.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildViewURL("http://host", tt.asset)
			if got != tt.want {
				t.Fatalf("buildViewURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// New – default poll interval
// ---------------------------------------------------------------------------

func TestNew_DefaultPollInterval(t *testing.T) {
	t.Parallel()

	e := New(Config{BaseURL: "http://host"})
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v, want %v", e.pollInterval, defaultPollInterval)
	}
}

// ---------------------------------------------------------------------------
// Execute – GIF and video output URL types in wait mode
// ---------------------------------------------------------------------------

func TestExecute_WaitGIFOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"p-gif"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/p-gif":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"outputs":{"1":{"gifs":[{"filename":"out.gif","type":"output"}]}}}`))
		}
	}))
	defer server.Close()

	e := New(Config{BaseURL: server.URL, WaitForCompletion: true, PollInterval: 5 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := e.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Kind != engine.OutputURL {
		t.Fatalf("Execute().Kind = %v, want OutputURL", got.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute – history returns wrapped format keyed by prompt ID
// ---------------------------------------------------------------------------

func TestExecute_WaitWrappedHistory(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"p-wrap"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/history/p-wrap":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]historyItem{
				"p-wrap": {Outputs: map[string]historyOutputs{
					"1": {Images: []historyAsset{{Filename: "wrapped.png", Type: "output"}}},
				}},
			}
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
		}
	}))
	defer server.Close()

	e := New(Config{BaseURL: server.URL, WaitForCompletion: true, PollInterval: 5 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := e.Execute(ctx, workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Kind != engine.OutputURL {
		t.Fatalf("Execute().Kind = %v, want OutputURL", got.Kind)
	}
}

// ---------------------------------------------------------------------------
// ModelInfos – verify returned metadata
// ---------------------------------------------------------------------------

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos() returned empty slice")
	}
	info := infos[0]
	if info.Provider != "comfyui" {
		t.Fatalf("ModelInfo.Provider = %q, want %q", info.Provider, "comfyui")
	}
	if info.Capability != "image" {
		t.Fatalf("ModelInfo.Capability = %q, want %q", info.Capability, "image")
	}
}
