package flux

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

func TestExecuteCreateAndPoll(t *testing.T) {
	t.Parallel()

	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Key") != "test-key" {
			t.Errorf("X-Key = %q", r.Header.Get("X-Key"))
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "sunset over mountains" {
				t.Errorf("prompt = %v", body["prompt"])
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"id":"task-123"}`))
			return
		}
		// GET poll
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"status":"Pending"}`))
			return
		}
		w.Write([]byte(`{"status":"Ready","result":{"sample":"https://cdn.bfl.ml/image.png"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		WaitForCompletion: true,
		PollInterval:      1, // minimal for tests
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset over mountains"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.bfl.ml/image.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"task-456"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: false})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "task-456" {
		t.Fatalf("Value = %q, want task ID", result.Value)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelPro11, WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion is set")
	}
}

func TestCapabilitiesNoWait(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelDev, WaitForCompletion: false})
	cap := e.Capabilities()
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false when WaitForCompletion is not set")
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true when WaitForCompletion is not set")
	}
	if len(cap.Models) != 1 || cap.Models[0] != ModelDev {
		t.Fatalf("Models = %v, want [%s]", cap.Models, ModelDev)
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "k"})
	if e.model != ModelPro11 {
		t.Fatalf("default model = %q, want %q", e.model, ModelPro11)
	}
	if e.baseURL != defaultBaseURL {
		t.Fatalf("default baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("default pollInterval = %v, want %v", e.pollInterval, defaultPollInterval)
	}
}

func TestNewTrimsBaseURL(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "k", BaseURL: "  https://example.com/  "})
	if e.baseURL != "https://example.com" {
		t.Fatalf("baseURL = %q, want trailing slash and whitespace trimmed", e.baseURL)
	}
}

func TestNewBaseURLFromEnv(t *testing.T) {
	t.Setenv("BFL_BASE_URL", "https://env.example.com/")
	e := New(Config{APIKey: "k"})
	if e.baseURL != "https://env.example.com" {
		t.Fatalf("baseURL from env = %q", e.baseURL)
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "test-key"})
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
	if !strings.Contains(err.Error(), "validate graph") {
		t.Fatalf("error = %v, want validate graph error", err)
	}
}

func TestExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("BFL_API_KEY", "")
	e := New(Config{})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("error = %v, want missing API key", err)
	}
}

func TestExecuteEmptyPrompt(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": ""}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("error = %v, want ErrMissingPrompt", err)
	}
}

func TestExecutePayloadOptions(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte(`{"id":"task-opt"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: false})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"width":        float64(1024),
			"height":       float64(768),
			"aspect_ratio": "16:9",
			"seed":         float64(42),
		}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if captured["prompt"] != "a cat" {
		t.Fatalf("prompt = %v", captured["prompt"])
	}
	if captured["width"] != float64(1024) {
		t.Fatalf("width = %v", captured["width"])
	}
	if captured["height"] != float64(768) {
		t.Fatalf("height = %v", captured["height"])
	}
	if captured["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio = %v", captured["aspect_ratio"])
	}
	if captured["seed"] != float64(42) {
		t.Fatalf("seed = %v", captured["seed"])
	}
}

func TestExecuteCreateEmptyID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":""}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: false})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Fatalf("error = %v, want empty id error", err)
	}
}

func TestExecuteCreateHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "bad-key", BaseURL: server.URL, WaitForCompletion: false})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

func TestExecutePollErrorStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"task-err"}`))
			return
		}
		w.Write([]byte(`{"status":"Error"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: true, PollInterval: 1})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for Error status")
	}
	if !strings.Contains(err.Error(), "task failed") {
		t.Fatalf("error = %v, want task failed", err)
	}
}

func TestExecutePollReadyNoSample(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"task-nosample"}`))
			return
		}
		w.Write([]byte(`{"status":"Ready","result":{"sample":""}}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: true, PollInterval: 1})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for Ready with no sample URL")
	}
	if !strings.Contains(err.Error(), "no sample URL") {
		t.Fatalf("error = %v, want no sample URL", err)
	}
}

func TestExecutePollHTTPError(t *testing.T) {
	t.Parallel()

	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"task-pollerr"}`))
			return
		}
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: true, PollInterval: 1})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for poll HTTP error")
	}
}

func TestExecuteContextCanceled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"task-ctx"}`))
			return
		}
		w.Write([]byte(`{"status":"Pending"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: true, PollInterval: 1})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(ctx, graph)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.String(), "id=resume-task") {
			t.Errorf("expected task ID in poll URL, got %s", r.URL.String())
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"status":"Ready","result":{"sample":"https://cdn.bfl.ml/resumed.png"}}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, WaitForCompletion: true, PollInterval: 1})

	result, err := e.Resume(context.Background(), "resume-task")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Value != "https://cdn.bfl.ml/resumed.png" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestResumeMissingKey(t *testing.T) {
	t.Setenv("BFL_API_KEY", "")
	e := New(Config{})
	_, err := e.Resume(context.Background(), "some-task")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("error = %v, want missing API key", err)
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) != 2 {
		t.Fatalf("ConfigSchema() returned %d fields, want 2", len(fields))
	}

	keyField := fields[0]
	if keyField.Key != "apiKey" {
		t.Fatalf("first field key = %q, want apiKey", keyField.Key)
	}
	if !keyField.Required {
		t.Fatal("apiKey field should be required")
	}
	if keyField.EnvVar != "BFL_API_KEY" {
		t.Fatalf("apiKey EnvVar = %q, want BFL_API_KEY", keyField.EnvVar)
	}

	urlField := fields[1]
	if urlField.Key != "baseUrl" {
		t.Fatalf("second field key = %q, want baseUrl", urlField.Key)
	}
	if urlField.Required {
		t.Fatal("baseUrl field should not be required")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	models := ModelsByCapability()
	imageModels, ok := models["image"]
	if !ok {
		t.Fatal("missing image capability")
	}
	if len(imageModels) != 4 {
		t.Fatalf("image models count = %d, want 4", len(imageModels))
	}

	expected := []string{ModelProUltra, ModelPro11, ModelPro, ModelDev}
	for i, want := range expected {
		if imageModels[i] != want {
			t.Fatalf("image model[%d] = %q, want %q", i, imageModels[i], want)
		}
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 4 {
		t.Fatalf("ModelInfos() returned %d, want 4", len(infos))
	}

	for _, info := range infos {
		if info.Provider != "flux" {
			t.Fatalf("provider = %q, want flux", info.Provider)
		}
		if info.Capability != "image" {
			t.Fatalf("capability = %q, want image", info.Capability)
		}
		if info.Name == "" {
			t.Fatal("model name is empty")
		}
		if info.DisplayName["en"] == "" {
			t.Fatalf("missing en display name for %s", info.Name)
		}
		if info.DocURL == "" {
			t.Fatalf("missing doc URL for %s", info.Name)
		}
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "flux" {
		t.Fatalf("provider name = %q, want flux", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("configs count = %d, want 1", len(p.Configs))
	}
	cfg := p.Configs[0]
	if cfg.Name != "flux-image" {
		t.Fatalf("config name = %q, want flux-image", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Fatal("engine is nil")
	}
	if len(cfg.EnvVars) != 1 || cfg.EnvVars[0] != "BFL_API_KEY" {
		t.Fatalf("EnvVars = %v, want [BFL_API_KEY]", cfg.EnvVars)
	}
}

func TestExecuteUsesConfiguredModel(t *testing.T) {
	t.Parallel()

	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			requestPath = r.URL.Path
			w.Write([]byte(`{"id":"task-model"}`))
			return
		}
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: ModelProUltra, WaitForCompletion: false})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if requestPath != "/v1/"+ModelProUltra {
		t.Fatalf("request path = %q, want /v1/%s", requestPath, ModelProUltra)
	}
}

func TestExecuteNoPromptNode(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "SomeOtherNode", Inputs: map[string]any{"data": "value"}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for graph with no prompt")
	}
	if err != ErrMissingPrompt {
		t.Fatalf("error = %v, want ErrMissingPrompt", err)
	}
}
