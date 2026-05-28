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
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/run" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["deployment_id"] != "deploy-abc" {
				t.Errorf("deployment_id = %v", body["deployment_id"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"run_id":"run-xyz"}`))
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/run" {
			if got := r.URL.Query().Get("run_id"); got != "run-xyz" {
				t.Errorf("run_id query param = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
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
			t.Errorf("unexpected method %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if got := r.URL.Query().Get("run_id"); got != "run-resume" {
			t.Errorf("run_id = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
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

func TestCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		waitForComp  bool
		wantSync     bool
		wantPoll     bool
	}{
		{"wait mode", true, false, true},
		{"no-wait mode", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New(Config{WaitForCompletion: tt.waitForComp})
			cap := e.Capabilities()
			if cap.SupportsSync != tt.wantSync {
				t.Fatalf("SupportsSync = %v, want %v", cap.SupportsSync, tt.wantSync)
			}
			if cap.SupportsPoll != tt.wantPoll {
				t.Fatalf("SupportsPoll = %v, want %v", cap.SupportsPoll, tt.wantPoll)
			}
			if len(cap.MediaTypes) != 2 {
				t.Fatalf("MediaTypes = %v, want [image video]", cap.MediaTypes)
			}
		})
	}
}

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

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "comfydeploy" {
		t.Fatalf("Name = %q, want %q", p.Name, "comfydeploy")
	}
	if len(p.Configs) == 0 {
		t.Fatal("Configs is empty")
	}
	if p.Configs[0].Name != "comfydeploy" {
		t.Fatalf("Configs[0].Name = %q, want %q", p.Configs[0].Name, "comfydeploy")
	}
	if p.Configs[0].Engine == nil {
		t.Fatal("Configs[0].Engine is nil")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos() returned empty slice")
	}
	if infos[0].Provider != "comfydeploy" {
		t.Fatalf("Provider = %q, want %q", infos[0].Provider, "comfydeploy")
	}
}

func TestExecute_MissingDeploymentID(t *testing.T) {
	t.Parallel()

	e := New(Config{APIToken: "tok", DeploymentID: ""})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err != ErrMissingDeploymentID {
		t.Fatalf("error = %v, want ErrMissingDeploymentID", err)
	}
}

func TestExecute_InvalidGraph(t *testing.T) {
	t.Parallel()

	e := New(Config{APIToken: "tok", DeploymentID: "d"})
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

func TestExecute_EmptyRunID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"run_id":""}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:     "tok",
		BaseURL:      server.URL,
		DeploymentID: "d",
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty run_id")
	}
}

func TestExecute_Webhook(t *testing.T) {
	t.Parallel()

	var gotWebhook string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if wh, ok := body["webhook"].(string); ok {
				gotWebhook = wh
			}
			w.Write([]byte(`{"run_id":"run-wh"}`))
			return
		}
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "tok",
		BaseURL:           server.URL,
		DeploymentID:      "d",
		Webhook:           "https://hook.example.com/cb",
		WaitForCompletion: false,
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotWebhook != "https://hook.example.com/cb" {
		t.Fatalf("webhook = %q, want %q", gotWebhook, "https://hook.example.com/cb")
	}
}

func TestExecute_PollTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"run_id":"run-to"}`))
			return
		}
		w.Write([]byte(`{"id":"run-to","status":"timeout"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "tok",
		BaseURL:           server.URL,
		DeploymentID:      "d",
		WaitForCompletion: true,
		PollInterval:      1,
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for timeout status")
	}
	if got := err.Error(); got != "comfydeploy: run timed out" {
		t.Fatalf("error = %q", got)
	}
}

func TestExecute_SuccessNoOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"run_id":"run-empty"}`))
			return
		}
		w.Write([]byte(`{"id":"run-empty","status":"success","outputs":[]}`))
	}))
	defer server.Close()

	e := New(Config{
		APIToken:          "tok",
		BaseURL:           server.URL,
		DeploymentID:      "d",
		WaitForCompletion: true,
		PollInterval:      1,
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for success with no output URL")
	}
}

func TestFirstOutputURL(t *testing.T) {
	t.Parallel()

	// Helper: unmarshal JSON into pollResponse to avoid verbose struct literals.
	parsePR := func(t *testing.T, raw string) pollResponse {
		t.Helper()
		var pr pollResponse
		if err := json.Unmarshal([]byte(raw), &pr); err != nil {
			t.Fatalf("parse pollResponse: %v", err)
		}
		return pr
	}

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "image URL",
			json: `{"outputs":[{"data":{"images":[{"url":"https://img.example.com/a.png"}]}}]}`,
			want: "https://img.example.com/a.png",
		},
		{
			name: "file URL when no images",
			json: `{"outputs":[{"data":{"files":[{"url":"https://files.example.com/b.mp4"}]}}]}`,
			want: "https://files.example.com/b.mp4",
		},
		{
			name: "GIF URL when no images or files",
			json: `{"outputs":[{"data":{"gifs":[{"url":"https://gifs.example.com/c.gif"}]}}]}`,
			want: "https://gifs.example.com/c.gif",
		},
		{
			name: "empty outputs",
			json: `{}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr := parsePR(t, tt.json)
			got := firstOutputURL(pr)
			if got != tt.want {
				t.Fatalf("firstOutputURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew_BaseURLFromEnv(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel
	t.Setenv("COMFYDEPLOY_BASE_URL", "https://custom.example.com/api/")
	e := New(Config{})
	if e.baseURL != "https://custom.example.com/api" {
		t.Fatalf("baseURL = %q, want trimmed env value", e.baseURL)
	}
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v, want %v", e.pollInterval, defaultPollInterval)
	}
}

func TestResume_MissingToken(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel
	e := New(Config{APIToken: "", DeploymentID: "d"})
	t.Setenv("COMFYDEPLOY_API_TOKEN", "")
	_, err := e.Resume(context.Background(), "run-id")
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestBuildInputs_ExtraStringInputs(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "SomeNode", Inputs: map[string]any{
			"style": "anime",
			"count": 42, // non-string, should be skipped
		}},
	}
	inputs := buildInputs(g)
	if inputs["style"] != "anime" {
		t.Fatalf("style = %q, want %q", inputs["style"], "anime")
	}
	if _, ok := inputs["count"]; ok {
		t.Fatal("non-string input 'count' should not be included")
	}
}

func TestBuildInputs_WhitespaceSkipped(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "   "}},
	}
	inputs := buildInputs(g)
	if _, ok := inputs["prompt"]; ok {
		t.Fatal("whitespace-only text should not produce a prompt")
	}
}
