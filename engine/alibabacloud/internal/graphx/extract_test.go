package graphx

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestImageURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"single image",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			[]string{"http://a.png"},
		},
		{
			"video node ignored",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			nil,
		},
		{
			"mixed nodes",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
				"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://b.png"}},
			},
			[]string{"http://a.png", "http://b.png"},
		},
		{
			"empty url ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": ""}},
			},
			nil,
		},
		{
			"no url input ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"text": "hello"}},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("ImageURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"single video",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			[]string{"http://v.mp4"},
		},
		{
			"image node ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			nil,
		},
		{
			"mixed nodes",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v1.mp4"}},
				"3": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v2.mp4"}},
			},
			[]string{"http://v1.mp4", "http://v2.mp4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VideoURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("VideoURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"collects both image and video",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			[]string{"http://a.png", "http://v.mp4"},
		},
		{
			"ignores non-media nodes",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"url": "http://x.png"}},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MediaURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("MediaURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoEditMedia(t *testing.T) {
	tests := []struct {
		name  string
		graph workflow.Graph
		want  []map[string]any
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"video and image",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
				"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			[]map[string]any{
				{"type": "video", "url": "http://v.mp4"},
				{"type": "reference_image", "url": "http://a.png"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VideoEditMedia(tt.graph)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("VideoEditMedia() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i]["type"] != tt.want[i]["type"] || got[i]["url"] != tt.want[i]["url"] {
					t.Errorf("VideoEditMedia()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPrompt(t *testing.T) {
	tests := []struct {
		name    string
		graph   workflow.Graph
		want    string
		wantErr bool
	}{
		{
			"from CLIPTextEncode",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
			},
			"a cat", false,
		},
		{
			"from option fallback",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"prompt": "a dog"}},
			},
			"a dog", false,
		},
		{
			"missing prompt",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 512}},
			},
			"", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Prompt(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Prompt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Prompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeSize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"1024x1024", "1024*1024"},
		{"1280*720", "1280*720"},
		{"512x512", "512*512"},
	}
	for _, tt := range tests {
		if got := NormalizeSize(tt.input); got != tt.want {
			t.Errorf("NormalizeSize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSize(t *testing.T) {
	tests := []struct {
		name     string
		graph    workflow.Graph
		fallback string
		want     string
	}{
		{
			"from option",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280x720"}},
			},
			"default",
			"1280*720",
		},
		{
			"from EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"default",
			"1920*1080",
		},
		{
			"fallback",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"default",
			"default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Size(tt.graph, tt.fallback); got != tt.want {
				t.Errorf("Size() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveResolution(t *testing.T) {
	tests := []struct {
		name  string
		graph workflow.Graph
		want  string
		ok    bool
	}{
		{
			"from size 1280*720",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280*720"}},
			},
			"720P", true,
		},
		{
			"from size 1920*1080",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1920*1080"}},
			},
			"1080P", true,
		},
		{
			"from EmptyLatentImage 1920x1080",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"1080P", true,
		},
		{
			"no resolution available",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DeriveResolution(tt.graph)
			if ok != tt.ok || got != tt.want {
				t.Errorf("DeriveResolution() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// strSliceEqual compares two string slices, treating nil and empty as equal.
func strSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
