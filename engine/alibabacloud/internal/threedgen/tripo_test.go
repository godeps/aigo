package threedgen

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

// promptGraph 构造一个仅含文本提示的最小 graph。
func promptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": prompt},
		},
	}
}

// imageGraph 构造一个含 N 张参考图的 graph，节点 id 单调递增以保证顺序稳定。
func imageGraph(urls ...string) workflow.Graph {
	g := workflow.Graph{}
	for i, u := range urls {
		// 用形如 "01"..."99" 的 id 保证 SortedNodeIDs 顺序与传参一致。
		id := "0" + string(rune('0'+i+1))
		g[id] = workflow.Node{
			ClassType: "ImageRef",
			Inputs:    map[string]any{"url": u},
		}
	}
	return g
}

func TestTripoPrompt_FromCLIPTextEncode(t *testing.T) {
	g := promptGraph("一只赛博朋克风格的机械猫")
	got, ok := tripoPrompt(g)
	if !ok || got != "一只赛博朋克风格的机械猫" {
		t.Fatalf("tripoPrompt(CLIPTextEncode) = %q, %v; want non-empty", got, ok)
	}
}

func TestTripoPrompt_FromOptionFallback(t *testing.T) {
	g := workflow.Graph{
		"opt": {
			ClassType: "Options",
			Inputs:    map[string]any{"prompt": "fallback prompt"},
		},
	}
	got, ok := tripoPrompt(g)
	if !ok || got != "fallback prompt" {
		t.Fatalf("tripoPrompt(option fallback) = %q, %v", got, ok)
	}
}

func TestTripoPrompt_Missing(t *testing.T) {
	if v, ok := tripoPrompt(workflow.Graph{}); ok {
		t.Fatalf("tripoPrompt(empty) = %q, want missing", v)
	}
}

func TestBuildTripoInput_PromptOnly(t *testing.T) {
	in, err := buildTripoInput(promptGraph("a cute robot"))
	if err != nil {
		t.Fatalf("buildTripoInput(prompt) err = %v", err)
	}
	if got, _ := in["prompt"].(string); got != "a cute robot" {
		t.Fatalf("input[prompt] = %v, want %q", in["prompt"], "a cute robot")
	}
	if _, has := in["image"]; has {
		t.Fatalf("prompt-only input must not contain image: %v", in)
	}
	if _, has := in["images"]; has {
		t.Fatalf("prompt-only input must not contain images: %v", in)
	}
}

func TestBuildTripoInput_SingleImage(t *testing.T) {
	in, err := buildTripoInput(imageGraph("https://example.com/a.png"))
	if err != nil {
		t.Fatalf("buildTripoInput(single image) err = %v", err)
	}
	if got, _ := in["image"].(string); got != "https://example.com/a.png" {
		t.Fatalf("input[image] = %v, want %q", in["image"], "https://example.com/a.png")
	}
	if _, has := in["images"]; has {
		t.Fatalf("single-image input must not contain images: %v", in)
	}
}

func TestBuildTripoInput_MultiImage(t *testing.T) {
	in, err := buildTripoInput(imageGraph(
		"https://example.com/a.png",
		"https://example.com/b.png",
	))
	if err != nil {
		t.Fatalf("buildTripoInput(2 images) err = %v", err)
	}
	images, ok := in["images"].([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("input[images] = %v, want 2 elements", in["images"])
	}
	if images[0] != "https://example.com/a.png" || images[1] != "https://example.com/b.png" {
		t.Fatalf("input[images] order broken: %v", images)
	}
	if _, has := in["image"]; has {
		t.Fatalf("multi-image input must not contain image: %v", in)
	}
}

func TestBuildTripoInput_TooManyImages(t *testing.T) {
	g := imageGraph(
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
		"https://example.com/d.png",
		"https://example.com/e.png",
	)
	_, err := buildTripoInput(g)
	if !errors.Is(err, ierr.ErrTooManyTripoImages) {
		t.Fatalf("buildTripoInput(5 images) err = %v, want ErrTooManyTripoImages", err)
	}
}

func TestBuildTripoInput_MissingAll(t *testing.T) {
	_, err := buildTripoInput(workflow.Graph{})
	if !errors.Is(err, ierr.ErrMissingTripoInput) {
		t.Fatalf("buildTripoInput(empty) err = %v, want ErrMissingTripoInput", err)
	}
}

func TestBuildTripoInput_PromptTooLong(t *testing.T) {
	long := strings.Repeat("猫", tripoMaxPromptCP+1)
	_, err := buildTripoInput(promptGraph(long))
	if !errors.Is(err, ierr.ErrTripoPromptTooLong) {
		t.Fatalf("buildTripoInput(long prompt) err = %v, want ErrTripoPromptTooLong", err)
	}
}

func TestBuildTripoInput_ImageWinsOverPrompt(t *testing.T) {
	g := imageGraph("https://example.com/a.png")
	g["txt"] = workflow.Node{
		ClassType: "CLIPTextEncode",
		Inputs:    map[string]any{"text": "ignored prompt"},
	}
	in, err := buildTripoInput(g)
	if err != nil {
		t.Fatalf("buildTripoInput err = %v", err)
	}
	if _, has := in["prompt"]; has {
		t.Fatalf("when image is present, prompt must not appear in input: %v", in)
	}
	if got, _ := in["image"].(string); got != "https://example.com/a.png" {
		t.Fatalf("input[image] = %v, want %q", in["image"], "https://example.com/a.png")
	}
}

func TestIsTripoModel(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"Tripo/Tripo-P1.0", true},
		{"Tripo/Tripo-H3.1", true},
		{"qwen-image", false},
		{"", false},
		{"tripo/lower-case", false},
	}
	for _, c := range cases {
		if got := IsTripoModel(c.model); got != c.want {
			t.Errorf("IsTripoModel(%q) = %v, want %v", c.model, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// tripoPrompt edge cases
// ---------------------------------------------------------------------------

func TestTripoPrompt_FromOptionText(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"opt": {
			ClassType: "Options",
			Inputs:    map[string]any{"text": "text fallback"},
		},
	}
	got, ok := tripoPrompt(g)
	if !ok || got != "text fallback" {
		t.Fatalf("tripoPrompt(text option) = %q, %v", got, ok)
	}
}

func TestTripoPrompt_FromOptionValue(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"opt": {
			ClassType: "Options",
			Inputs:    map[string]any{"value": "value fallback"},
		},
	}
	got, ok := tripoPrompt(g)
	if !ok || got != "value fallback" {
		t.Fatalf("tripoPrompt(value option) = %q, %v", got, ok)
	}
}

func TestTripoPrompt_WhitespaceOnlyCLIP(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": "   "},
		},
		"opt": {
			ClassType: "Options",
			Inputs:    map[string]any{"prompt": "real prompt"},
		},
	}
	got, ok := tripoPrompt(g)
	if !ok || got != "real prompt" {
		t.Fatalf("tripoPrompt(whitespace CLIP) = %q, %v; want 'real prompt'", got, ok)
	}
}

// ---------------------------------------------------------------------------
// buildTripoInput boundary cases
// ---------------------------------------------------------------------------

func TestBuildTripoInput_ThreeImages(t *testing.T) {
	t.Parallel()
	in, err := buildTripoInput(imageGraph(
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
	))
	if err != nil {
		t.Fatalf("buildTripoInput(3 images) err = %v", err)
	}
	images, ok := in["images"].([]any)
	if !ok || len(images) != 3 {
		t.Fatalf("input[images] = %v, want 3 elements", in["images"])
	}
}

func TestBuildTripoInput_FourImages(t *testing.T) {
	t.Parallel()
	in, err := buildTripoInput(imageGraph(
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
		"https://example.com/d.png",
	))
	if err != nil {
		t.Fatalf("buildTripoInput(4 images) err = %v", err)
	}
	images, ok := in["images"].([]any)
	if !ok || len(images) != 4 {
		t.Fatalf("input[images] = %v, want 4 elements", in["images"])
	}
}

func TestBuildTripoInput_PromptExactlyAtLimit(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("a", tripoMaxPromptCP)
	in, err := buildTripoInput(promptGraph(exact))
	if err != nil {
		t.Fatalf("buildTripoInput(1024-char prompt) err = %v", err)
	}
	if got, _ := in["prompt"].(string); got != exact {
		t.Fatalf("input[prompt] length = %d, want %d", len(got), tripoMaxPromptCP)
	}
}

// ---------------------------------------------------------------------------
// RunTripo3D tests (httptest mock, WaitForCompletion=false)
// ---------------------------------------------------------------------------

// tripoHandler returns an http.HandlerFunc that responds with a task ID.
// If check is non-nil, it is called with the decoded request payload.
func tripoHandler(t *testing.T, check func(t *testing.T, payload map[string]any)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tripoEndpoint {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if check != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
				http.Error(w, "read error", http.StatusBadRequest)
				return
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("unmarshal body: %v", err)
				http.Error(w, "unmarshal error", http.StatusBadRequest)
				return
			}
			check(t, payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"task_id":"task-abc-123"}}`))
	}
}

func TestRunTripo3D_PromptInput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		if p["model"] != modelTripoP1 {
			t.Errorf("model = %v, want %s", p["model"], modelTripoP1)
		}
		input, _ := p["input"].(map[string]any)
		if input["prompt"] != "a cute robot" {
			t.Errorf("input.prompt = %v, want 'a cute robot'", input["prompt"])
		}
		if _, has := p["parameters"]; has {
			t.Error("parameters should not be present when no options set")
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	taskID, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, promptGraph("a cute robot"))
	if err != nil {
		t.Fatalf("RunTripo3D err = %v", err)
	}
	if taskID != "task-abc-123" {
		t.Fatalf("taskID = %q, want %q", taskID, "task-abc-123")
	}
}

func TestRunTripo3D_SingleImage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		input, _ := p["input"].(map[string]any)
		if input["image"] != "https://example.com/cat.png" {
			t.Errorf("input.image = %v", input["image"])
		}
		if _, has := input["images"]; has {
			t.Error("single-image input must not have images key")
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	taskID, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, imageGraph("https://example.com/cat.png"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if taskID == "" {
		t.Fatal("taskID is empty")
	}
}

func TestRunTripo3D_MultiImage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		input, _ := p["input"].(map[string]any)
		images, ok := input["images"].([]any)
		if !ok || len(images) != 3 {
			t.Errorf("input.images = %v, want 3 elements", input["images"])
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	g := imageGraph("https://a.png", "https://b.png", "https://c.png")
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, g)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTripo3D_TextureQuality(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		params, ok := p["parameters"].(map[string]any)
		if !ok {
			t.Fatal("parameters missing")
		}
		if params["texture_quality"] != "high" {
			t.Errorf("texture_quality = %v, want high", params["texture_quality"])
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	g := workflow.Graph{
		"1":   {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a robot"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{"texture_quality": "high"}},
	}
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, g)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTripo3D_GeometryQualityH31(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		params, ok := p["parameters"].(map[string]any)
		if !ok {
			t.Fatal("parameters missing")
		}
		if params["geometry_quality"] != "extra_high" {
			t.Errorf("geometry_quality = %v, want extra_high", params["geometry_quality"])
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	g := workflow.Graph{
		"1":   {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a robot"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{"geometry_quality": "extra_high"}},
	}
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoH31, g)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTripo3D_GeometryQualityIgnoredOnP1(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(tripoHandler(t, func(t *testing.T, p map[string]any) {
		if params, ok := p["parameters"].(map[string]any); ok {
			if _, has := params["geometry_quality"]; has {
				t.Error("geometry_quality must not be set for P1 model")
			}
		}
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	g := workflow.Graph{
		"1":   {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a robot"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{"geometry_quality": "extra_high"}},
	}
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, g)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTripo3D_InputError(t *testing.T) {
	t.Parallel()
	rt := &runtime.RT{BaseURL: "http://unused", HTTPClient: &http.Client{}, WaitForCompletion: false}
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, workflow.Graph{})
	if !errors.Is(err, ierr.ErrMissingTripoInput) {
		t.Fatalf("err = %v, want ErrMissingTripoInput", err)
	}
}

func TestRunTripo3D_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"InternalError","message":"server error"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: srv.Client(), WaitForCompletion: false}
	_, err := RunTripo3D(context.Background(), rt, "test-key", modelTripoP1, promptGraph("test"))
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
