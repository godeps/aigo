package ark

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

func TestExecuteTextToVideo(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/contents/generations/tasks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-test-001"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/contents/generations/tasks/cgt-test-001":
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if calls.Load() < 2 {
				_, _ = w.Write([]byte(`{"id":"cgt-test-001","status":"running"}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"cgt-test-001","status":"succeeded","content":{"video_url":"https://v.example.com/seedance.mp4"}}`))
			}
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "A cat playing piano"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/seedance.mp4" {
		t.Fatalf("got %q", out.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/contents/generations/tasks" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cgt-nowait-002"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Sunset over ocean"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "cgt-nowait-002" {
		t.Fatalf("expected task id, got %q", out.Value)
	}
}

func TestExecuteFailed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-fail-003"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-fail-003","status":"failed","error":{"code":"content_filter","message":"content blocked"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content blocked") {
		t.Fatalf("expected content blocked error, got: %v", err)
	}
}

func TestExecuteImageToVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-i2v-004"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-i2v-004","status":"succeeded","content":{"video_url":"https://v.example.com/i2v.mp4"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "animate this image"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{
			"url":  "https://example.com/photo.jpg",
			"role": "first_frame",
		}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/i2v.mp4" {
		t.Fatalf("got %q", out.Value)
	}

	// verify payload structure
	contentArr, ok := gotPayload["content"].([]any)
	if !ok || len(contentArr) < 2 {
		t.Fatalf("expected at least 2 content items, got %v", gotPayload["content"])
	}
	imgItem, ok := contentArr[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", contentArr[1])
	}
	if imgItem["type"] != "image_url" {
		t.Fatalf("expected image_url type, got %v", imgItem["type"])
	}
	if imgItem["role"] != "first_frame" {
		t.Fatalf("expected first_frame role, got %v", imgItem["role"])
	}
}

func TestExecuteMultiModal(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-mm-005"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-mm-005","status":"succeeded","content":{"video_url":"https://v.example.com/mm.mp4"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "multi-modal prompt"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{
			"url":  "https://example.com/ref.jpg",
			"role": "reference_image",
		}},
		"3": {ClassType: "LoadVideo", Inputs: map[string]any{
			"url": "https://example.com/ref.mp4",
		}},
		"4": {ClassType: "LoadAudio", Inputs: map[string]any{
			"url": "https://example.com/bgm.mp3",
		}},
		"5": {ClassType: "VideoOptions", Inputs: map[string]any{
			"duration":       10,
			"generate_audio": true,
		}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/mm.mp4" {
		t.Fatalf("got %q", out.Value)
	}

	contentArr, ok := gotPayload["content"].([]any)
	if !ok {
		t.Fatalf("expected content array, got %T", gotPayload["content"])
	}
	// text + image + video + audio = 4
	if len(contentArr) != 4 {
		t.Fatalf("expected 4 content items, got %d", len(contentArr))
	}
}

func TestExecuteExpired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-exp-006"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-exp-006","status":"expired"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for expired task")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}
}

func TestMissingContent(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test-model",
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1280, "height": 720}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
	if !strings.Contains(err.Error(), "no content") {
		t.Fatalf("expected missing content error, got: %v", err)
	}
}

func TestExecuteMissingModel(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		APIKey:  "sk-test",
		Model:   "",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if err != ErrMissingModel {
		t.Fatalf("expected ErrMissingModel, got: %v", err)
	}
}

func TestExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("ARK_API_KEY", "")

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test-model",
		APIKey:  "",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing api key")
	}
}

func TestExecuteEmptyCreateID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":""}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for empty create id")
	}
	if !strings.Contains(err.Error(), "create missing id") {
		t.Fatalf("expected create missing id error, got: %v", err)
	}
}

func TestExecuteBadCreateJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for bad json")
	}
	if !strings.Contains(err.Error(), "decode create") {
		t.Fatalf("expected decode create error, got: %v", err)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if calls.Load() < 2 {
			_, _ = w.Write([]byte(`{"id":"cgt-resume-001","status":"running"}`))
		} else {
			_, _ = w.Write([]byte(`{"id":"cgt-resume-001","status":"succeeded","content":{"video_url":"https://v.example.com/resumed.mp4"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	result, err := eng.Resume(context.Background(), "cgt-resume-001")
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "https://v.example.com/resumed.mp4" {
		t.Fatalf("expected video URL, got %q", result.Value)
	}
}

func TestResumeMissingAPIKey(t *testing.T) {
	t.Setenv("ARK_API_KEY", "")

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test-model",
		APIKey:  "",
	})

	_, err := eng.Resume(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for missing api key")
	}
}

func TestCapabilitiesVideo(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key", Model: "doubao-seedance-2-0-260128", WaitForCompletion: true})
	cap := eng.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "video" {
		t.Fatalf("expected video media type, got %v", cap.MediaTypes)
	}
	if cap.MaxDuration != 15 {
		t.Fatalf("expected max duration 15, got %d", cap.MaxDuration)
	}
	if !cap.SupportsPoll {
		t.Fatal("expected SupportsPoll=true when WaitForCompletion=true")
	}
	if cap.SupportsSync {
		t.Fatal("expected SupportsSync=false when WaitForCompletion=true")
	}
}

func TestCapabilitiesVideoNoWait(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key", Model: "doubao-seedance-2-0-260128", WaitForCompletion: false})
	cap := eng.Capabilities()
	if cap.SupportsPoll {
		t.Fatal("expected SupportsPoll=false when WaitForCompletion=false")
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync=true when WaitForCompletion=false")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) == 0 {
		t.Fatal("expected config fields")
	}

	foundKey := false
	foundURL := false
	for _, f := range fields {
		switch f.Key {
		case "apiKey":
			foundKey = true
			if f.EnvVar != "ARK_API_KEY" {
				t.Fatalf("expected ARK_API_KEY env var, got %s", f.EnvVar)
			}
			if !f.Required {
				t.Fatal("apiKey should be required")
			}
		case "baseUrl":
			foundURL = true
			if f.Default != defaultBaseURL {
				t.Fatalf("expected default base URL %s, got %s", defaultBaseURL, f.Default)
			}
		}
	}
	if !foundKey {
		t.Fatal("missing apiKey field")
	}
	if !foundURL {
		t.Fatal("missing baseUrl field")
	}
}

func TestModelsByCapabilityVideoAndImage(t *testing.T) {
	t.Parallel()

	caps := ModelsByCapability()
	if len(caps["video"]) == 0 {
		t.Fatal("expected video models")
	}
	if len(caps["image"]) == 0 {
		t.Fatal("expected image models")
	}
	// Check specific video models exist.
	foundSeedance := false
	for _, m := range caps["video"] {
		if m == "doubao-seedance-2-0-260128" {
			foundSeedance = true
		}
	}
	if !foundSeedance {
		t.Fatal("missing doubao-seedance-2-0-260128 in video models")
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "ark" {
		t.Fatalf("expected provider name 'ark', got %q", p.Name)
	}
	if len(p.Configs) < 3 {
		t.Fatalf("expected at least 3 configs, got %d", len(p.Configs))
	}

	names := make(map[string]bool)
	for _, c := range p.Configs {
		names[c.Name] = true
		if c.Engine == nil {
			t.Fatalf("config %s has nil engine", c.Name)
		}
	}
	for _, want := range []string{"ark-image", "ark-video", "ark-video-fast"} {
		if !names[want] {
			t.Fatalf("missing config %q", want)
		}
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) < 5 {
		t.Fatalf("expected at least 5 model infos, got %d", len(infos))
	}
	for _, info := range infos {
		if info.Provider != "ark" {
			t.Fatalf("expected provider 'ark', got %q for model %s", info.Provider, info.Name)
		}
		if info.Name == "" {
			t.Fatal("model info has empty name")
		}
		if info.Capability == "" {
			t.Fatalf("model %s has empty capability", info.Name)
		}
	}
}

func TestParseTaskResponseCancelled(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseTaskResponse([]byte(`{"id":"t","status":"cancelled"}`))
	if err == nil {
		t.Fatal("expected error for cancelled task")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled error, got: %v", err)
	}
}

func TestParseTaskResponseSucceededNoURL(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseTaskResponse([]byte(`{"id":"t","status":"succeeded","content":{}}`))
	if err == nil {
		t.Fatal("expected error for succeeded but no url")
	}
	if !strings.Contains(err.Error(), "no video_url") {
		t.Fatalf("expected no video_url error, got: %v", err)
	}
}

func TestParseTaskResponseSucceededNilContent(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseTaskResponse([]byte(`{"id":"t","status":"succeeded"}`))
	if err == nil {
		t.Fatal("expected error for succeeded but nil content")
	}
	if !strings.Contains(err.Error(), "no video_url") {
		t.Fatalf("expected no video_url error, got: %v", err)
	}
}

func TestParseTaskResponseFailedNoMessage(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseTaskResponse([]byte(`{"id":"t","status":"failed"}`))
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "task failed: failed") {
		t.Fatalf("expected generic failed error, got: %v", err)
	}
}

func TestParseTaskResponseInvalidJSON(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseTaskResponse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
	if !strings.Contains(err.Error(), "decode task") {
		t.Fatalf("expected decode task error, got: %v", err)
	}
}

func TestParseTaskResponseRunning(t *testing.T) {
	t.Parallel()

	url, _, done, err := parseTaskResponse([]byte(`{"id":"t","status":"running"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatal("expected done=false for running")
	}
	if url != "" {
		t.Fatalf("expected empty url for running, got %q", url)
	}
}

func TestParseTaskResponseQueued(t *testing.T) {
	t.Parallel()

	_, _, done, err := parseTaskResponse([]byte(`{"id":"t","status":"queued"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatal("expected done=false for queued")
	}
}

func TestBuildPayloadAllOptions(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "test-video-model", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test prompt"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"ratio":                   "16:9",
			"resolution":              "1080p",
			"seed":                    42,
			"generate_audio":          true,
			"watermark":               false,
			"return_last_frame":       true,
			"draft":                   true,
			"service_tier":            "standard",
			"callback_url":            "https://example.com/cb",
			"execution_expires_after": 3600,
			"frames":                  30,
			"camera_fixed":            true,
		}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 10}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	checks := map[string]any{
		"model":                   "test-video-model",
		"ratio":                   "16:9",
		"resolution":              "1080p",
		"seed":                    42,
		"generate_audio":          true,
		"watermark":               false,
		"return_last_frame":       true,
		"draft":                   true,
		"service_tier":            "standard",
		"callback_url":            "https://example.com/cb",
		"execution_expires_after": 3600,
		"frames":                  30,
		"camera_fixed":            true,
		"duration":                10,
	}
	for k, want := range checks {
		got, ok := payload[k]
		if !ok {
			t.Fatalf("payload missing key %q", k)
			continue
		}
		// Compare as JSON to handle int/float ambiguity.
		wantJSON, _ := json.Marshal(want)
		gotJSON, _ := json.Marshal(got)
		if string(gotJSON) != string(wantJSON) {
			t.Fatalf("payload[%q] = %s, want %s", k, gotJSON, wantJSON)
		}
	}
}

func TestBuildPayloadWithTools(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "test-model", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "search the web"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"tools": []any{
				map[string]any{"type": "web_search"},
			},
		}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	tools, ok := payload["tools"]
	if !ok {
		t.Fatal("payload missing tools")
	}
	toolSlice, ok := tools.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", tools)
	}
	if len(toolSlice) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolSlice))
	}
	if toolSlice[0]["type"] != "web_search" {
		t.Fatalf("expected web_search tool, got %v", toolSlice[0])
	}
}

func TestBuildPayloadExtraBody(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "test-model", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"extra_body": `{"custom_field":"custom_value","n":2}`,
		}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	if payload["custom_field"] != "custom_value" {
		t.Fatalf("expected custom_field from extra_body, got %v", payload["custom_field"])
	}
}

func TestBuildPayloadVideoWithRole(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "test-model", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{
			"url": "https://example.com/vid.mp4",
		}},
		"3": {ClassType: "LoadAudio", Inputs: map[string]any{
			"url": "https://example.com/audio.mp3",
		}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	contentArr, ok := payload["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content array, got %T", payload["content"])
	}

	// Find video and audio entries, verify default roles.
	for _, item := range contentArr {
		switch item["type"] {
		case "video_url":
			if item["role"] != "reference_video" {
				t.Fatalf("expected default role 'reference_video', got %v", item["role"])
			}
		case "audio_url":
			if item["role"] != "reference_audio" {
				t.Fatalf("expected default role 'reference_audio', got %v", item["role"])
			}
		}
	}
}

func TestBuildPayloadVideoWithCustomRole(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "test-model", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{
			"url":  "https://example.com/vid.mp4",
			"role": "custom_role",
		}},
		"3": {ClassType: "LoadAudio", Inputs: map[string]any{
			"url":  "https://example.com/audio.mp3",
			"role": "bgm",
		}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	contentArr, ok := payload["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content array, got %T", payload["content"])
	}

	for _, item := range contentArr {
		switch item["type"] {
		case "video_url":
			if item["role"] != "custom_role" {
				t.Fatalf("expected custom_role, got %v", item["role"])
			}
		case "audio_url":
			if item["role"] != "bgm" {
				t.Fatalf("expected bgm, got %v", item["role"])
			}
		}
	}
}

func TestBuildPayloadDraftTask(t *testing.T) {
	t.Parallel()

	eng := New(Config{Model: "doubao-seedance-1-5-pro-251215", APIKey: "sk-test"})

	graph := workflow.Graph{
		"1": {ClassType: "LoadDraftTask", Inputs: map[string]any{"id": "cgt-2026xxxx-pzjqb"}},
	}

	payload, err := eng.buildPayload(graph)
	if err != nil {
		t.Fatal(err)
	}

	contentArr, ok := payload["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content array, got %T", payload["content"])
	}
	if len(contentArr) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(contentArr))
	}
	item := contentArr[0]
	if item["type"] != "draft_task" {
		t.Fatalf("expected type draft_task, got %v", item["type"])
	}
	dt, ok := item["draft_task"].(map[string]any)
	if !ok {
		t.Fatalf("expected draft_task object, got %T", item["draft_task"])
	}
	if dt["id"] != "cgt-2026xxxx-pzjqb" {
		t.Fatalf("expected draft task id cgt-2026xxxx-pzjqb, got %v", dt["id"])
	}
}

func TestParseTaskResponseLastFrameURL(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"t","status":"succeeded","content":{"video_url":"https://example.com/video.mp4","last_frame_url":"https://example.com/last_frame.png"}}`)
	videoURL, lastFrameURL, done, err := parseTaskResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true")
	}
	if videoURL != "https://example.com/video.mp4" {
		t.Fatalf("expected video URL, got %q", videoURL)
	}
	if lastFrameURL != "https://example.com/last_frame.png" {
		t.Fatalf("expected last frame URL, got %q", lastFrameURL)
	}
}

func TestParseTaskResponseNoLastFrame(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"t","status":"succeeded","content":{"video_url":"https://example.com/video.mp4"}}`)
	videoURL, lastFrameURL, done, err := parseTaskResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true")
	}
	if videoURL != "https://example.com/video.mp4" {
		t.Fatalf("expected video URL, got %q", videoURL)
	}
	if lastFrameURL != "" {
		t.Fatalf("expected empty last frame URL, got %q", lastFrameURL)
	}
}

func TestExecuteCancelled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-cancel-007"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-cancel-007","status":"cancelled"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for cancelled task")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled error, got: %v", err)
	}
}

func TestNewDefaultBaseURL(t *testing.T) {
	t.Setenv("ARK_BASE_URL", "")

	eng := New(Config{APIKey: "key", Model: "m"})
	if eng.baseURL != defaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", defaultBaseURL, eng.baseURL)
	}
}

func TestNewBaseURLFromEnv(t *testing.T) {
	t.Setenv("ARK_BASE_URL", "https://custom.example.com")

	eng := New(Config{APIKey: "key", Model: "m"})
	if eng.baseURL != "https://custom.example.com" {
		t.Fatalf("expected env base URL, got %q", eng.baseURL)
	}
}

func TestNewBaseURLTrailingSlash(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key", Model: "m", BaseURL: "https://example.com/"})
	if strings.HasSuffix(eng.baseURL, "/") {
		t.Fatalf("expected trailing slash trimmed, got %q", eng.baseURL)
	}
}

func TestNewDefaultPollInterval(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key", Model: "m"})
	if eng.pollInterval != defaultPollInterval {
		t.Fatalf("expected default poll interval %v, got %v", defaultPollInterval, eng.pollInterval)
	}
}

func TestNewCustomPollInterval(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key", Model: "m", PollInterval: 10 * time.Second})
	if eng.pollInterval != 10*time.Second {
		t.Fatalf("expected 10s poll interval, got %v", eng.pollInterval)
	}
}

func TestExecuteContextCancelled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-ctx-008"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-ctx-008","status":"running"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(ctx, graph)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
