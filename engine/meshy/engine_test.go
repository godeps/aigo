package meshy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteTextTo3DWithPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "a medieval castle" {
				t.Errorf("prompt = %v", body["prompt"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"result":"task-abc"}`))
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"status":"IN_PROGRESS"}`))
			return
		}
		w.Write([]byte(`{"status":"SUCCEEDED","model_urls":{"glb":"https://cdn.meshy.ai/model.glb","fbx":"https://cdn.meshy.ai/model.fbx"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a medieval castle"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.meshy.ai/model.glb" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"task-xyz"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a spaceship"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
	if result.Value != "task-xyz" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestExecuteImageTo3D(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if r.URL.Path != "/openapi/v2/image-to-3d" {
				t.Errorf("path = %q, want /openapi/v2/image-to-3d", r.URL.Path)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["image_url"] != "https://example.com/chair.png" {
				t.Errorf("image_url = %v", body["image_url"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"result":"task-img"}`))
			return
		}
		w.Write([]byte(`{"status":"SUCCEEDED","model_urls":{"glb":"https://cdn.meshy.ai/chair.glb"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a chair"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/chair.png"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.meshy.ai/chair.glb" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestPollFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"result":"task-fail"}`))
			return
		}
		w.Write([]byte(`{"status":"FAILED","task_error":{"message":"invalid mesh topology"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a broken model"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "invalid mesh topology") {
		t.Fatalf("error = %q, want to contain 'invalid mesh topology'", got)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/openapi/v2/text-to-3d/task-resume" {
			t.Errorf("path = %q", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"status":"SUCCEEDED","model_urls":{"glb":"https://cdn.meshy.ai/resumed.glb"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		PollInterval: 1,
	})

	result, err := e.Resume(context.Background(), "task-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
	if result.Value != "https://cdn.meshy.ai/resumed.glb" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingAPIKey(t *testing.T) {
	t.Parallel()

	e := New(Config{})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 3 {
		t.Fatalf("ConfigSchema() returned %d fields, want 3", len(fields))
	}

	apiKeyField := fields[0]
	if apiKeyField.Key != "apiKey" {
		t.Fatalf("fields[0].Key = %q", apiKeyField.Key)
	}
	if apiKeyField.EnvVar != "MESHY_API_KEY" {
		t.Fatalf("fields[0].EnvVar = %q", apiKeyField.EnvVar)
	}
	if !apiKeyField.Required {
		t.Fatal("apiKey field should be required")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	models := ModelsByCapability()
	if got, ok := models["3d"]; !ok || len(got) != 2 {
		t.Fatalf("ModelsByCapability()[\"3d\"] = %v", got)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	e := New(Config{WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "3d" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is true")
	}
}

func TestFallbackModelURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"result":"task-fbx"}`))
			return
		}
		// Only FBX available, no GLB.
		w.Write([]byte(`{"status":"SUCCEEDED","model_urls":{"fbx":"https://cdn.meshy.ai/model.fbx"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.meshy.ai/model.fbx" {
		t.Fatalf("Value = %q, want FBX fallback URL", result.Value)
	}
}

func TestPollHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"result":"task-err"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for poll HTTP failure")
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "meshy" {
		t.Fatalf("Name = %q, want meshy", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("len(Configs) = %d, want 1", len(p.Configs))
	}
	cfg := p.Configs[0]
	if cfg.Name != "meshy-3d" {
		t.Fatalf("Config.Name = %q, want meshy-3d", cfg.Name)
	}
	if len(cfg.EnvVars) != 1 || cfg.EnvVars[0] != "MESHY_API_KEY" {
		t.Fatalf("EnvVars = %v", cfg.EnvVars)
	}
	if cfg.Engine == nil {
		t.Fatal("Engine is nil")
	}
}

func TestExecuteWithOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["negative_prompt"] != "low quality" {
				t.Errorf("negative_prompt = %v", body["negative_prompt"])
			}
			if body["art_style"] != "realistic" {
				t.Errorf("art_style = %v", body["art_style"])
			}
			if body["topology"] != "quad" {
				t.Errorf("topology = %v", body["topology"])
			}
			w.Write([]byte(`{"result":"task-opts"}`))
			return
		}
		w.Write([]byte(`{"status":"SUCCEEDED","model_urls":{"glb":"https://cdn.meshy.ai/opts.glb"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a dragon"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"negative_prompt": "low quality",
			"art_style":       "realistic",
			"topology":        "quad",
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.meshy.ai/opts.glb" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestMissingPrompt(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "key"})
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"mode": "preview"}},
	}
	_, err := e.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 2 {
		t.Fatalf("len(ModelInfos) = %d, want 2", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "meshy" {
			t.Errorf("Provider = %q, want meshy", info.Provider)
		}
		if info.Capability != "3d" {
			t.Errorf("Capability = %q, want 3d", info.Capability)
		}
	}
}


