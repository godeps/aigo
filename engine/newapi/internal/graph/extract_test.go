package graph

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestExtractImageSizePrefersImageOptions(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1536*1024"}},
		"2": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
		"3": {ClassType: "NegativePrompt", Inputs: map[string]any{"size": "256x256"}},
	}
	got := ExtractImageSizeOpenAI(g)
	if got != "1536x1024" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractNegativePromptPrefersNode(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "NegativePrompt", Inputs: map[string]any{"negative_prompt": "from node"}},
		"2": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "p", "negative_prompt": "wrong"}},
	}
	v, ok := ExtractNegativePrompt(g)
	if !ok || v != "from node" {
		t.Fatalf("got %q %v", v, ok)
	}
}
