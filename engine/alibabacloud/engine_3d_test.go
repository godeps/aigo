package alibabacloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

const tripoEndpoint = "/api/v1/services/aigc/video-generation/3d-generation"

func TestExecuteTripoP1TextTo3D(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tripoEndpoint:
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("Authorization header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Errorf("X-DashScope-Async header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-t-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/tripo-t-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-t-task","task_status":"SUCCEEDED","results":[{"pbr_model_url":"https://cdn.tripo3d.com/cat.glb","rendered_image_url":"https://cdn.tripo3d.com/cat.webp"}]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoP1,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只可爱的猫"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"texture_quality": "standard"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://cdn.tripo3d.com/cat.glb" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelTripoP1 {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	if input["prompt"] != "一只可爱的猫" {
		t.Fatalf("input.prompt = %#v", input["prompt"])
	}
	if _, hasImage := input["image"]; hasImage {
		t.Fatalf("text-to-3d must omit input.image, got %#v", input["image"])
	}
	if _, hasImages := input["images"]; hasImages {
		t.Fatalf("text-to-3d must omit input.images, got %#v", input["images"])
	}
	parameters, ok := createPayload["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("parameters missing: %#v", createPayload)
	}
	if parameters["texture_quality"] != "standard" {
		t.Fatalf("texture_quality = %#v", parameters["texture_quality"])
	}
	// geometry_quality is H3.1-only; must not appear for P1.0.
	if _, ok := parameters["geometry_quality"]; ok {
		t.Fatalf("P1.0 must not include geometry_quality, got %#v", parameters)
	}
}

func TestExecuteTripoP1SingleImageTo3D(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tripoEndpoint:
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-i-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/tripo-i-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-i-task","task_status":"SUCCEEDED","results":[{"pbr_model_url":"https://cdn.tripo3d.com/single.glb"}]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoP1,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "ignored when image present"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/cat.png"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://cdn.tripo3d.com/single.glb" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	input := createPayload["input"].(map[string]any)
	if input["image"] != "https://assets.example.com/cat.png" {
		t.Fatalf("input.image = %#v", input["image"])
	}
	if _, hasPrompt := input["prompt"]; hasPrompt {
		t.Fatalf("single-image-to-3d must omit input.prompt, got %#v", input["prompt"])
	}
	if _, hasImages := input["images"]; hasImages {
		t.Fatalf("single-image must use input.image, not images, got %#v", input["images"])
	}
}

func TestExecuteTripoH31MultiImageTo3D(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tripoEndpoint:
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-m-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/tripo-m-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-m-task","task_status":"SUCCEEDED","results":[{"pbr_model_url":"https://cdn.tripo3d.com/multi.glb"}]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoH31,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/a.png"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/b.png"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/c.png"}},
		"4": {ClassType: "ImageOptions", Inputs: map[string]any{
			"texture_quality":  "detailed",
			"geometry_quality": "ultra",
		}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://cdn.tripo3d.com/multi.glb" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelTripoH31 {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	images, ok := input["images"].([]any)
	if !ok {
		t.Fatalf("input.images missing or wrong type: %#v", input["images"])
	}
	if len(images) != 3 {
		t.Fatalf("input.images len = %d, want 3", len(images))
	}
	if images[0] != "https://assets.example.com/a.png" ||
		images[1] != "https://assets.example.com/b.png" ||
		images[2] != "https://assets.example.com/c.png" {
		t.Fatalf("input.images = %#v", images)
	}
	if _, hasImage := input["image"]; hasImage {
		t.Fatalf("multi-image must use input.images, not image, got %#v", input["image"])
	}
	parameters := createPayload["parameters"].(map[string]any)
	if parameters["texture_quality"] != "detailed" {
		t.Fatalf("texture_quality = %#v", parameters["texture_quality"])
	}
	if parameters["geometry_quality"] != "ultra" {
		t.Fatalf("geometry_quality = %#v", parameters["geometry_quality"])
	}
}

func TestExecuteTripoP1OmitsParametersWhenEmpty(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == tripoEndpoint:
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-x-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/tripo-x-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"tripo-x-task","task_status":"SUCCEEDED","results":[{"pbr_model_url":"https://cdn.tripo3d.com/notex.glb"}]}}`))
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoP1,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "no-texture model"}},
	}

	if _, err := engine.Execute(context.Background(), graph); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// 不传 texture_quality 时整个 parameters 字段应被省略，触发服务端「无贴图模型」分支。
	if _, ok := createPayload["parameters"]; ok {
		t.Fatalf("parameters must be omitted when no quality flags set, got %#v", createPayload["parameters"])
	}
}

func TestExecuteTripoRejectsTooManyImages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected upstream call: %s %s", r.Method, r.URL.Path)
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoH31,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{}
	for i, url := range []string{
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
		"https://example.com/d.png",
		"https://example.com/e.png",
	} {
		graph[string(rune('1'+i))] = workflow.Node{
			ClassType: "LoadImage",
			Inputs:    map[string]any{"url": url},
		}
	}

	if _, err := engine.Execute(context.Background(), graph); !errors.Is(err, ErrTooManyTripoImages) {
		t.Fatalf("Execute() err = %v, want ErrTooManyTripoImages", err)
	}
}

func TestExecuteTripoRejectsLongPrompt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected upstream call: %s %s", r.Method, r.URL.Path)
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoP1,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{
			"text": strings.Repeat("龙", 1025), // 1025 个 UTF-8 字符 → 超限
		}},
	}

	if _, err := engine.Execute(context.Background(), graph); !errors.Is(err, ErrTripoPromptTooLong) {
		t.Fatalf("Execute() err = %v, want ErrTripoPromptTooLong", err)
	}
}

func TestExecuteTripoMissingInputs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected upstream call: %s %s", r.Method, r.URL.Path)
		http.Error(w, "test assertion failed", http.StatusInternalServerError)
		return
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelTripoP1,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	// 既无 prompt 也无 image。
	graph := workflow.Graph{
		"1": {ClassType: "ImageOptions", Inputs: map[string]any{"texture_quality": "standard"}},
	}

	if _, err := engine.Execute(context.Background(), graph); !errors.Is(err, ErrMissingTripoInput) {
		t.Fatalf("Execute() err = %v, want ErrMissingTripoInput", err)
	}
}

func TestTripoCapabilitiesAndModelInfos(t *testing.T) {
	t.Parallel()

	for _, name := range []string{ModelTripoP1, ModelTripoH31} {
		eng := New(Config{Model: name, BaseURL: "https://example.com/api/v1"})
		cap := eng.Capabilities()
		if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "3d" {
			t.Errorf("model=%q MediaTypes = %v, want [3d]", name, cap.MediaTypes)
		}
	}

	infos := ModelInfos()
	want := map[string]bool{ModelTripoP1: false, ModelTripoH31: false}
	for _, info := range infos {
		if _, ok := want[info.Name]; ok {
			want[info.Name] = true
			if info.Capability != "3d" {
				t.Errorf("ModelInfo %q Capability = %q, want 3d", info.Name, info.Capability)
			}
			if info.DocURL != "https://help.aliyun.com/zh/model-studio/tripo-3d-generation-api-reference" {
				t.Errorf("ModelInfo %q DocURL = %q", info.Name, info.DocURL)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ModelInfos missing entry for %q", name)
		}
	}

	groups := ModelsByCapability()
	threeD := groups["3d"]
	hasP1, hasH31 := false, false
	for _, m := range threeD {
		if m == ModelTripoP1 {
			hasP1 = true
		}
		if m == ModelTripoH31 {
			hasH31 = true
		}
	}
	if !hasP1 || !hasH31 {
		t.Errorf("ModelsByCapability[\"3d\"] = %v, want both Tripo models", threeD)
	}
}
