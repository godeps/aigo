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
