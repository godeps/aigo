package hailuo

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

func TestExecute_Success(t *testing.T) {
	t.Parallel()
	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/video_generation":
			json.NewEncoder(w).Encode(map[string]any{
				"task_id":   "task-abc",
				"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
			})
		case strings.HasPrefix(r.URL.Path, "/v1/query/video_generation"):
			n := pollCount.Add(1)
			if n < 2 {
				json.NewEncoder(w).Encode(map[string]any{
					"status":    "Processing",
					"base_resp": map[string]any{"status_code": 0},
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"status":    "Success",
					"file_id":   "file-xyz",
					"base_resp": map[string]any{"status_code": 0},
				})
			}
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:            "test-key",
		BaseURL:           srv.URL,
		Model:             ModelT2V01,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat walking"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Value, "file_id=file-xyz") {
		t.Errorf("expected file_id in URL, got %q", result.Value)
	}
}

func TestExecute_NoWait(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"task_id":   "task-nowait",
			"base_resp": map[string]any{"status_code": 0},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:            "key",
		BaseURL:           srv.URL,
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value != "task-nowait" {
		t.Errorf("expected task_id, got %q", result.Value)
	}
}

func TestExecute_PollFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/video_generation" {
			json.NewEncoder(w).Encode(map[string]any{
				"task_id":   "task-fail",
				"base_resp": map[string]any{"status_code": 0},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"status":    "Failed",
				"base_resp": map[string]any{"status_code": 1, "status_msg": "insufficient credits"},
			})
		}
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:            "key",
		BaseURL:           srv.URL,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "Success",
			"file_id":   "file-resumed",
			"base_resp": map[string]any{"status_code": 0},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:       "key",
		BaseURL:      srv.URL,
		PollInterval: 10 * time.Millisecond,
	})

	result, err := eng.Resume(context.Background(), "task-old")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if !strings.Contains(result.Value, "file-resumed") {
		t.Errorf("expected file_id in URL, got %q", result.Value)
	}
}

func TestExecute_MissingKey(t *testing.T) {
	t.Parallel()
	eng := New(Config{})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestExecute_MissingPrompt(t *testing.T) {
	t.Parallel()
	eng := New(Config{APIKey: "key"})
	g := workflow.Graph{
		"1": {ClassType: "Something", Inputs: map[string]any{}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) != 3 {
		t.Errorf("expected 3 config fields, got %d", len(fields))
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	eng := New(Config{WaitForCompletion: true})
	cap := eng.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Errorf("expected [video], got %v", cap.MediaTypes)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	if len(m["video"]) != 4 {
		t.Errorf("expected 4 video models, got %d", len(m["video"]))
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "hailuo" {
		t.Fatalf("Name = %q, want hailuo", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("len(Configs) = %d, want 1", len(p.Configs))
	}
	cfg := p.Configs[0]
	if cfg.Name != "hailuo-video" {
		t.Fatalf("Config.Name = %q, want hailuo-video", cfg.Name)
	}
	if len(cfg.EnvVars) != 1 || cfg.EnvVars[0] != "HAILUO_API_KEY" {
		t.Fatalf("EnvVars = %v", cfg.EnvVars)
	}
	if cfg.Engine == nil {
		t.Fatal("Engine is nil")
	}
}

func TestExecute_WithImage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/video_generation":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["first_frame_image"] != "https://example.com/frame.jpg" {
				t.Errorf("expected first_frame_image, got %v", body["first_frame_image"])
			}
			json.NewEncoder(w).Encode(map[string]any{
				"task_id":   "task-img",
				"base_resp": map[string]any{"status_code": 0},
			})
		case strings.HasPrefix(r.URL.Path, "/v1/query/video_generation"):
			json.NewEncoder(w).Encode(map[string]any{
				"status":    "Success",
				"file_id":   "file-img",
				"base_resp": map[string]any{"status_code": 0},
			})
		}
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:            "key",
		BaseURL:           srv.URL,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "animate this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/frame.jpg"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Value, "file-img") {
		t.Errorf("expected file_id in URL, got %q", result.Value)
	}
}

func TestExecute_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"task_id":   "",
			"base_resp": map[string]any{"status_code": 1003, "status_msg": "rate limited"},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %v, want 'rate limited'", err)
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 4 {
		t.Fatalf("len(ModelInfos) = %d, want 4", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "hailuo" {
			t.Errorf("Provider = %q, want hailuo", info.Provider)
		}
		if info.Capability != "video" {
			t.Errorf("Capability = %q, want video", info.Capability)
		}
	}
}
