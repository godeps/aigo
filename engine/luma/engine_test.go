package luma

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

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func promptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func promptGraphWithOptions(prompt string, opts map[string]any) workflow.Graph {
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
		"2": {ClassType: "Options", Inputs: opts},
	}
	return g
}

func promptGraphWithImage(prompt, imageURL string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": imageURL}},
	}
}

// videoServer returns a test server that handles create + poll for video.
func videoServer(t *testing.T, wantPayloadCheck func(body map[string]any)) *httptest.Server {
	t.Helper()
	var pollCount int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if wantPayloadCheck != nil {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				wantPayloadCheck(body)
			}
			w.Write([]byte(`{"id":"gen-abc"}`))
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"state":"dreaming"}`))
			return
		}
		w.Write([]byte(`{"state":"completed","assets":{"video":"https://cdn.luma.ai/video.mp4"}}`))
	}))
}

// imageServer returns a test server that handles create + poll for image.
func imageServer(t *testing.T, wantPayloadCheck func(body map[string]any)) *httptest.Server {
	t.Helper()
	var pollCount int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if wantPayloadCheck != nil {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				wantPayloadCheck(body)
			}
			w.Write([]byte(`{"id":"img-123"}`))
			return
		}
		count := atomic.AddInt32(&pollCount, 1)
		if count < 2 {
			w.Write([]byte(`{"state":"queued"}`))
			return
		}
		w.Write([]byte(`{"state":"completed","assets":{"image":"https://cdn.luma.ai/photo.png"}}`))
	}))
}

// ---------------------------------------------------------------------------
// New() configuration
// ---------------------------------------------------------------------------

func TestNewDefaults(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "k"})
	if e.model != ModelRay2 {
		t.Fatalf("default model = %q, want %q", e.model, ModelRay2)
	}
	if e.baseURL != defaultBaseURL {
		t.Fatalf("default baseURL = %q, want %q", e.baseURL, defaultBaseURL)
	}
	if e.pollInterval != defaultPollInterval {
		t.Fatalf("default pollInterval = %v, want %v", e.pollInterval, defaultPollInterval)
	}
}

func TestNewTrimsBaseURLSlash(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "k", BaseURL: "https://example.com/"})
	if strings.HasSuffix(e.baseURL, "/") {
		t.Fatalf("baseURL should not end with slash: %q", e.baseURL)
	}
}

func TestNewBaseURLFromEnv(t *testing.T) {
	t.Setenv("LUMA_BASE_URL", "https://env.example.com/")
	e := New(Config{APIKey: "k"})
	if e.baseURL != "https://env.example.com" {
		t.Fatalf("baseURL = %q, want trimmed env value", e.baseURL)
	}
}

func TestNewCustomPollInterval(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "k", PollInterval: 10 * time.Second})
	if e.pollInterval != 10*time.Second {
		t.Fatalf("pollInterval = %v, want 10s", e.pollInterval)
	}
}

// ---------------------------------------------------------------------------
// Execute — basic video
// ---------------------------------------------------------------------------

func TestExecuteVideo(t *testing.T) {
	t.Parallel()

	server := videoServer(t, func(body map[string]any) {
		if body["prompt"] != "a dancing robot" {
			t.Errorf("prompt = %v", body["prompt"])
		}
	})
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Execute(context.Background(), promptGraph("a dancing robot"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/video.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — basic image
// ---------------------------------------------------------------------------

func TestExecuteImage(t *testing.T) {
	t.Parallel()

	server := imageServer(t, nil)
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhoton1,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Execute(context.Background(), promptGraph("a sunset"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/photo.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

// ---------------------------------------------------------------------------
// Execute — video with options (aspect_ratio, duration, loop)
// ---------------------------------------------------------------------------

func TestExecuteVideoWithOptions(t *testing.T) {
	t.Parallel()

	server := videoServer(t, func(body map[string]any) {
		if body["aspect_ratio"] != "16:9" {
			t.Errorf("aspect_ratio = %v, want 16:9", body["aspect_ratio"])
		}
		// duration comes through as float64 from JSON
		if d, ok := body["duration"].(float64); !ok || int(d) != 10 {
			t.Errorf("duration = %v, want 10", body["duration"])
		}
		if body["loop"] != true {
			t.Errorf("loop = %v, want true", body["loop"])
		}
	})
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := promptGraphWithOptions("a robot", map[string]any{
		"aspect_ratio": "16:9",
		"duration":     10,
		"loop":         true,
	})

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — image-to-video (LoadImage keyframes)
// ---------------------------------------------------------------------------

func TestExecuteImageToVideo(t *testing.T) {
	t.Parallel()

	server := videoServer(t, func(body map[string]any) {
		kf, ok := body["keyframes"].(map[string]any)
		if !ok {
			t.Fatal("missing keyframes in payload")
		}
		f0, ok := kf["frame0"].(map[string]any)
		if !ok {
			t.Fatal("missing frame0 in keyframes")
		}
		if f0["type"] != "image" {
			t.Errorf("frame0.type = %v, want image", f0["type"])
		}
		if f0["url"] != "https://example.com/start.png" {
			t.Errorf("frame0.url = %v", f0["url"])
		}
	})
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRayFlash2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := promptGraphWithImage("animate this", "https://example.com/start.png")
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — image with reference image (style transfer)
// ---------------------------------------------------------------------------

func TestExecuteImageWithRef(t *testing.T) {
	t.Parallel()

	server := imageServer(t, func(body map[string]any) {
		ref, ok := body["image_ref"].(map[string]any)
		if !ok {
			t.Fatal("missing image_ref in payload")
		}
		if ref["url"] != "https://example.com/ref.png" {
			t.Errorf("image_ref.url = %v", ref["url"])
		}
	})
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhotonFlash1,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := promptGraphWithImage("style transfer", "https://example.com/ref.png")
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — image with aspect_ratio
// ---------------------------------------------------------------------------

func TestExecuteImageWithAspectRatio(t *testing.T) {
	t.Parallel()

	server := imageServer(t, func(body map[string]any) {
		if body["aspect_ratio"] != "1:1" {
			t.Errorf("aspect_ratio = %v, want 1:1", body["aspect_ratio"])
		}
	})
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhoton1,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	graph := promptGraphWithOptions("square photo", map[string]any{"aspect_ratio": "1:1"})
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — no-wait mode (returns task ID)
// ---------------------------------------------------------------------------

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"gen-nowait"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: false,
	})

	result, err := e.Execute(context.Background(), promptGraph("quick test"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "gen-nowait" {
		t.Fatalf("Value = %q, want gen-nowait", result.Value)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}
}

// ---------------------------------------------------------------------------
// Execute — error paths
// ---------------------------------------------------------------------------

func TestExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "")

	e := New(Config{Model: ModelRay2})
	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("error = %v, want 'missing API key'", err)
	}
}

func TestExecuteMissingPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestExecuteEmptyPrompt(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})
	_, err := e.Execute(context.Background(), promptGraph(""))
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	t.Parallel()

	e := New(Config{APIKey: "test-key"})
	_, err := e.Execute(context.Background(), workflow.Graph{})
	if err == nil {
		t.Fatal("expected error for empty graph")
	}
}

func TestExecuteCreateReturnsEmptyID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":""}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for empty id")
	}
	if !strings.Contains(err.Error(), "empty id") {
		t.Fatalf("error = %v, want 'empty id'", err)
	}
}

func TestExecuteCreateReturnsInvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "decode create") {
		t.Fatalf("error = %v, want 'decode create'", err)
	}
}

func TestExecuteHTTPErrorOnCreate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "bad-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestExecuteHTTP500OnCreate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhoton1,
		WaitForCompletion: true,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------------------------------------------------------------------------
// Poll — failure / edge cases
// ---------------------------------------------------------------------------

func TestPollGenerationFailed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-fail"}`))
			return
		}
		w.Write([]byte(`{"state":"failed","failure_reason":"content policy violation"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for failed generation")
	}
	if !strings.Contains(err.Error(), "content policy violation") {
		t.Fatalf("error = %v, want failure reason", err)
	}
}

func TestPollGenerationFailedNoReason(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-fail2"}`))
			return
		}
		w.Write([]byte(`{"state":"failed"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for failed generation")
	}
	if !strings.Contains(err.Error(), "generation failed: failed") {
		t.Fatalf("error = %v, want generic failure message", err)
	}
}

func TestPollCompletedNoAssetURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-empty"}`))
			return
		}
		w.Write([]byte(`{"state":"completed","assets":{}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for completed with no asset URL")
	}
	if !strings.Contains(err.Error(), "no asset URL") {
		t.Fatalf("error = %v, want 'no asset URL'", err)
	}
}

func TestPollCompletedImageAsset(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-img"}`))
			return
		}
		// completed with image asset only (no video)
		w.Write([]byte(`{"state":"completed","assets":{"image":"https://cdn.luma.ai/result.png"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelPhoton1,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Execute(context.Background(), promptGraph("image test"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/result.png" {
		t.Fatalf("Value = %q", result.Value)
	}
}

func TestPollHTTPError(t *testing.T) {
	t.Parallel()

	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-pollerr"}`))
			return
		}
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	_, err := e.Execute(context.Background(), promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for poll HTTP error")
	}
}

func TestPollContextCancelled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"gen-ctx"}`))
			return
		}
		w.Write([]byte(`{"state":"dreaming"}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := e.Execute(ctx, promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Resume
// ---------------------------------------------------------------------------

func TestResume(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "gen-resume") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"state":"completed","assets":{"video":"https://cdn.luma.ai/resumed.mp4"}}`))
	}))
	defer server.Close()

	e := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             ModelRay2,
		WaitForCompletion: true,
		PollInterval:      1,
	})

	result, err := e.Resume(context.Background(), "gen-resume")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if result.Value != "https://cdn.luma.ai/resumed.mp4" {
		t.Fatalf("Value = %q", result.Value)
	}
	if result.Kind != engine.OutputURL {
		t.Fatalf("Kind = %v, want OutputURL", result.Kind)
	}
}

func TestResumeMissingAPIKey(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "")

	e := New(Config{})
	_, err := e.Resume(context.Background(), "gen-123")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelRay2, WaitForCompletion: true})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.MaxDuration != 10 {
		t.Fatalf("MaxDuration = %v, want 10", cap.MaxDuration)
	}
	if !cap.SupportsPoll {
		t.Fatal("SupportsPoll should be true when WaitForCompletion=true")
	}
	if cap.SupportsSync {
		t.Fatal("SupportsSync should be false when WaitForCompletion=true")
	}
}

func TestCapabilitiesVideoNoWait(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelRayFlash2, WaitForCompletion: false})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "video" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.SupportsPoll {
		t.Fatal("SupportsPoll should be false when WaitForCompletion=false")
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true when WaitForCompletion=false")
	}
}

func TestCapabilitiesImage(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelPhoton1})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if cap.MaxDuration != 0 {
		t.Fatalf("MaxDuration = %v, want 0 for image", cap.MaxDuration)
	}
}

func TestCapabilitiesImageFlash(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelPhotonFlash1})
	cap := e.Capabilities()
	if cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
}

func TestCapabilitiesModelsField(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelRay2})
	cap := e.Capabilities()
	if len(cap.Models) != 1 || cap.Models[0] != ModelRay2 {
		t.Fatalf("Models = %v, want [%s]", cap.Models, ModelRay2)
	}
}

// ---------------------------------------------------------------------------
// ConfigSchema
// ---------------------------------------------------------------------------

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("expected at least 2 config fields, got %d", len(fields))
	}

	var foundKey, foundURL bool
	for _, f := range fields {
		if f.Key == "apiKey" && f.Required && f.EnvVar == "LUMA_API_KEY" {
			foundKey = true
		}
		if f.Key == "baseUrl" && f.EnvVar == "LUMA_BASE_URL" && f.Default == defaultBaseURL {
			foundURL = true
		}
	}
	if !foundKey {
		t.Fatal("ConfigSchema() missing required apiKey field")
	}
	if !foundURL {
		t.Fatal("ConfigSchema() missing baseUrl field")
	}
}

// ---------------------------------------------------------------------------
// ModelsByCapability
// ---------------------------------------------------------------------------

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()

	videos, ok := m["video"]
	if !ok || len(videos) == 0 {
		t.Fatal("expected video models")
	}
	images, ok := m["image"]
	if !ok || len(images) == 0 {
		t.Fatal("expected image models")
	}

	hasRay2 := false
	for _, v := range videos {
		if v == ModelRay2 {
			hasRay2 = true
		}
	}
	if !hasRay2 {
		t.Fatalf("video models missing %q", ModelRay2)
	}

	hasPhoton := false
	for _, v := range images {
		if v == ModelPhoton1 {
			hasPhoton = true
		}
	}
	if !hasPhoton {
		t.Fatalf("image models missing %q", ModelPhoton1)
	}
}

// ---------------------------------------------------------------------------
// ModelInfos
// ---------------------------------------------------------------------------

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 4 {
		t.Fatalf("ModelInfos() returned %d entries, want 4", len(infos))
	}

	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
		if info.Provider != "luma" {
			t.Errorf("ModelInfo %q provider = %q, want luma", info.Name, info.Provider)
		}
		if info.DocURL == "" {
			t.Errorf("ModelInfo %q has empty DocURL", info.Name)
		}
	}

	for _, want := range []string{ModelRay2, ModelRayFlash2, ModelPhoton1, ModelPhotonFlash1} {
		if !names[want] {
			t.Errorf("ModelInfos() missing %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultProvider
// ---------------------------------------------------------------------------

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "luma" {
		t.Fatalf("Provider.Name = %q, want luma", p.Name)
	}
	if len(p.Configs) != 2 {
		t.Fatalf("Provider.Configs length = %d, want 2", len(p.Configs))
	}

	names := map[string]bool{}
	for _, c := range p.Configs {
		names[c.Name] = true
		if len(c.EnvVars) == 0 || c.EnvVars[0] != "LUMA_API_KEY" {
			t.Errorf("config %q missing LUMA_API_KEY env var", c.Name)
		}
		if c.Engine == nil {
			t.Errorf("config %q has nil Engine", c.Name)
		}
	}
	if !names["luma-video"] {
		t.Fatal("missing luma-video config")
	}
	if !names["luma-image"] {
		t.Fatal("missing luma-image config")
	}
}
