package threedgen

import (
	"errors"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
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
