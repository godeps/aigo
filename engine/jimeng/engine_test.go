package jimeng

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteImageSuccess(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.jimeng.com/img/result.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   ModelJimeng21,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.jimeng.com/img/result.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if gotPayload["prompt"] != "a beautiful sunset" {
		t.Fatalf("prompt = %v", gotPayload["prompt"])
	}
	if gotPayload["model"] != ModelJimeng21 {
		t.Fatalf("model = %v", gotPayload["model"])
	}
}

func TestExecuteVideoSuccess(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/v1/video/generations" {
			_, _ = w.Write([]byte(`{"id":"task-vid-001"}`))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/video/generations/task-vid-001") {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":"https://cdn.jimeng.com/video/out.mp4"}}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a rocket launch"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.jimeng.com/video/out.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteVideoNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/video/generations" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-nowait-002"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset over ocean"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "task-nowait-002" {
		t.Fatalf("expected task id, got %q", result.Value)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
}

func TestExecuteVideoPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"task-fail-003"}`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"status":"failed","error":{"code":"content_filter","message":"content blocked"}}`))
		}
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content blocked") {
		t.Fatalf("expected content blocked error, got: %v", err)
	}
}

func TestResumeSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer resume-key" {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":"https://cdn.jimeng.com/video/resumed.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "resume-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	result, err := e.Resume(context.Background(), "task-resume-004")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Value != "https://cdn.jimeng.com/video/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

func TestMissingAPIKey(t *testing.T) {
	t.Setenv("JIMENG_API_KEY", "")

	e := New(Config{
		BaseURL: "https://example.com",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://example.com",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("expected ErrMissingPrompt, got: %v", err)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 2 {
		t.Fatalf("expected 2 config fields, got %d", len(fields))
	}

	apiKeyField := fields[0]
	if apiKeyField.Key != "apiKey" {
		t.Fatalf("first field key = %q", apiKeyField.Key)
	}
	if !apiKeyField.Required {
		t.Fatal("apiKey should be required")
	}
	if apiKeyField.EnvVar != "JIMENG_API_KEY" {
		t.Fatalf("apiKey envVar = %q", apiKeyField.EnvVar)
	}

	baseURLField := fields[1]
	if baseURLField.Key != "baseUrl" {
		t.Fatalf("second field key = %q", baseURLField.Key)
	}
	if baseURLField.Default != defaultBaseURL {
		t.Fatalf("baseUrl default = %q", baseURLField.Default)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	imageModels, ok := models["image"]
	if !ok {
		t.Fatal("expected image capability")
	}
	if len(imageModels) != 2 {
		t.Fatalf("expected 2 image models, got %d", len(imageModels))
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()

	e := New(Config{Model: ModelJimeng21})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync for image model")
	}
}

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()

	e := New(Config{Endpoint: "/v1/video/generations", WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("expected SupportsPoll for video with WaitForCompletion")
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q", e.baseURL)
	}
	if e.model != defaultModel {
		t.Fatalf("model = %q", e.model)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v", e.pollInterval)
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "jimeng" {
		t.Fatalf("Name = %q, want jimeng", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("len(Configs) = %d, want 1", len(p.Configs))
	}
	cfg := p.Configs[0]
	if cfg.Name != "jimeng-image" {
		t.Fatalf("Config.Name = %q", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Fatal("Config.Engine is nil")
	}
	if len(cfg.EnvVars) == 0 || cfg.EnvVars[0] != "JIMENG_API_KEY" {
		t.Fatalf("EnvVars = %v", cfg.EnvVars)
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) != 2 {
		t.Fatalf("len(ModelInfos) = %d, want 2", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "jimeng" {
			t.Fatalf("Provider = %q, want jimeng", info.Provider)
		}
		if info.Capability != "image" {
			t.Fatalf("Capability = %q, want image", info.Capability)
		}
	}
	if infos[0].Name != ModelJimeng21 {
		t.Fatalf("infos[0].Name = %q, want %q", infos[0].Name, ModelJimeng21)
	}
	if infos[1].Name != ModelJimeng20Pro {
		t.Fatalf("infos[1].Name = %q, want %q", infos[1].Name, ModelJimeng20Pro)
	}
}

func TestExecuteValidateError(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key", BaseURL: "https://example.com"})

	// Empty graph should fail validation.
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
	if !strings.Contains(err.Error(), "validate graph") {
		t.Fatalf("expected validate graph error, got: %v", err)
	}
}

func TestExtractImageResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		format  string
		want    string
		wantErr string
	}{
		{
			name:   "url format",
			body:   `{"data":[{"url":"https://cdn.jimeng.com/img.png"}]}`,
			format: "url",
			want:   "https://cdn.jimeng.com/img.png",
		},
		{
			name:   "b64_json format",
			body:   `{"data":[{"b64_json":"dGVzdA=="}]}`,
			format: "b64_json",
			want:   "data:image/png;base64,dGVzdA==",
		},
		{
			name:   "b64 fallback when url empty",
			body:   `{"data":[{"b64_json":"ZmFsbGJhY2s="}]}`,
			format: "url",
			want:   "data:image/png;base64,ZmFsbGJhY2s=",
		},
		{
			name:    "api error",
			body:    `{"error":{"code":"rate_limit","message":"too many requests"}}`,
			format:  "url",
			wantErr: "image api error rate_limit: too many requests",
		},
		{
			name:    "empty data",
			body:    `{"data":[]}`,
			format:  "url",
			wantErr: "image response had no data",
		},
		{
			name:    "no url or b64",
			body:    `{"data":[{"url":"","b64_json":""}]}`,
			format:  "url",
			wantErr: "image response had no url or b64_json",
		},
		{
			name:    "invalid json",
			body:    `{invalid`,
			format:  "url",
			wantErr: "decode image response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractImageResult([]byte(tt.body), tt.format)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteImageWithOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.jimeng.com/img.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   ModelJimeng21,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"size":            "1024x1024",
			"seed":            float64(42),
			"negative_prompt": "blurry",
		}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotPayload["size"] != "1024x1024" {
		t.Fatalf("size = %v", gotPayload["size"])
	}
	if gotPayload["negative_prompt"] != "blurry" {
		t.Fatalf("negative_prompt = %v", gotPayload["negative_prompt"])
	}
}

func TestExecuteImageCustomEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/images" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.jimeng.com/custom.png"}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Endpoint: "/custom/images",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.jimeng.com/custom.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteVideoWithLoadImageAndOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			_, _ = w.Write([]byte(`{"id":"task-img2vid"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":"https://cdn.jimeng.com/video.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "animate this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.png"}},
		"3": {ClassType: "Options", Inputs: map[string]any{
			"duration":     float64(5),
			"aspect_ratio": "16:9",
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.jimeng.com/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if gotPayload["image_url"] != "https://example.com/ref.png" {
		t.Fatalf("image_url = %v", gotPayload["image_url"])
	}
	if gotPayload["ratio"] != "16:9" {
		t.Fatalf("ratio = %v", gotPayload["ratio"])
	}
}

func TestExecuteVideoEmptyID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":""}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty id")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Fatalf("error = %v, want containing 'empty id'", err)
	}
}

func TestPollCancelled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"task-cancel"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"cancelled"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for cancelled task")
	}
	if !strings.Contains(err.Error(), "task cancelled") {
		t.Fatalf("error = %v, want containing 'task cancelled'", err)
	}
}

func TestPollSucceededNoURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"task-nourl"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"succeeded","output":{"video_url":""}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for succeeded but no video_url")
	}
	if !strings.Contains(err.Error(), "no video_url") {
		t.Fatalf("error = %v, want containing 'no video_url'", err)
	}
}

func TestPollHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"task-httperr"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestPollFailedNoMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"task-failnomsg"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"failed"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Endpoint:          "/v1/video/generations",
		WaitForCompletion: true,
		PollInterval:      1 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "task failed: failed") {
		t.Fatalf("error = %v, want containing 'task failed: failed'", err)
	}
}

func TestResumeAPIKeyError(t *testing.T) {
	t.Setenv("JIMENG_API_KEY", "")

	e := New(Config{BaseURL: "https://example.com"})
	_, err := e.Resume(context.Background(), "some-task-id")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestCapabilitiesVideoNoWait(t *testing.T) {
	t.Parallel()

	e := New(Config{Endpoint: "/v1/video/generations", WaitForCompletion: false})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.SupportsPoll {
		t.Fatal("expected SupportsPoll=false when WaitForCompletion=false")
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync=true when WaitForCompletion=false")
	}
}

func TestImageB64Response(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"dGVzdA=="}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"response_format": "b64_json"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value = %q", result.Value)
	}
}
