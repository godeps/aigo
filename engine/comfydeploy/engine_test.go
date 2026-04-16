package comfydeploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecute_Success(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/run" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["deployment_id"] != "deploy-abc" {
				t.Fatalf("deployment_id = %v", body["deployment_id"])
			}
			w.Write([]byte(`{"run_id":"run-xyz"}`))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/run" {
			if got := r.URL.Query().Get("run_id"); got != "run-xyz" {
				t.Fatalf("run_id query param = %q", got)
			}
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				w.Write([]byte(`{"id":"run-xyz","status":"running"}`))
				return
			}
			w.Write([]byte(`{"id":"run-xyz","status":"success","outputs":[{"data":{"images":[{"url":"https://cdn.comfydeploy.com/output.png","filename":"output.png"}]}}]}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "test-token",
		BaseURL:           server.URL,
		DeploymentID:      "deploy-abc",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful landscape"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.comfydeploy.com/output.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecute_NoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"run_id":"run-nowait"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "test-token",
		BaseURL:           server.URL,
		DeploymentID:      "deploy-abc",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "quick test"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "run-nowait" {
		t.Fatalf("Value = %q, want %q", result.Value, "run-nowait")
	}
}

func TestExecute_PollFailure(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			w.Write([]byte(`{"run_id":"run-fail"}`))
			return
		}

		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"id":"run-fail","status":"running"}`))
			return
		}
		w.Write([]byte(`{"id":"run-fail","status":"failed"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "test-token",
		BaseURL:           server.URL,
		DeploymentID:      "deploy-abc",
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "will fail"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if err.Error() != "comfydeploy: run failed" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.URL.Query().Get("run_id"); got != "run-resume" {
			t.Fatalf("run_id = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"run-resume","status":"success","outputs":[{"data":{"files":[{"url":"https://cdn.comfydeploy.com/video.mp4","filename":"video.mp4"}]}}]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:     "test-token",
		BaseURL:      server.URL,
		DeploymentID: "deploy-abc",
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "run-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.comfydeploy.com/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestBuildInputs(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sunny beach"}},
		"2": {ClassType: "NegativePrompt", Inputs: map[string]any{"text": "blurry, dark"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
		"4": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/vid.mp4"}},
	}

	inputs := buildInputs(g)

	if inputs["prompt"] != "a sunny beach" {
		t.Fatalf("prompt = %q", inputs["prompt"])
	}
	if inputs["negative_prompt"] != "blurry, dark" {
		t.Fatalf("negative_prompt = %q", inputs["negative_prompt"])
	}
	if inputs["image"] != "https://example.com/img.png" {
		t.Fatalf("image = %q", inputs["image"])
	}
	if inputs["video"] != "https://example.com/vid.mp4" {
		t.Fatalf("video = %q", inputs["video"])
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) == 0 {
		t.Fatal("ConfigSchema() returned empty slice")
	}

	keys := make(map[string]bool)
	for _, f := range fields {
		keys[f.Key] = true
	}

	for _, required := range []string{"apiToken", "deploymentId"} {
		if !keys[required] {
			t.Fatalf("ConfigSchema() missing field %q", required)
		}
	}
}
