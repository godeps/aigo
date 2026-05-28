package graph

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestStringOptionFromClassTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		g          workflow.Graph
		classTypes []string
		keys       []string
		wantVal    string
		wantOK     bool
	}{
		{
			name: "match by class type",
			g: workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "512x512"}},
				"2": {ClassType: "Other", Inputs: map[string]any{"size": "256x256"}},
			},
			classTypes: []string{"ImageOptions"},
			keys:       []string{"size"},
			wantVal:    "512x512",
			wantOK:     true,
		},
		{
			name: "no match returns empty",
			g: workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"size": "512x512"}},
			},
			classTypes: []string{"ImageOptions"},
			keys:       []string{"size"},
			wantVal:    "",
			wantOK:     false,
		},
		{
			name: "multiple keys returns first found",
			g: workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"format": "png", "quality": "high"}},
			},
			classTypes: []string{"ImageOptions"},
			keys:       []string{"format", "quality"},
			wantVal:    "png",
			wantOK:     true,
		},
		{
			name: "multiple class types",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"codec": "h264"}},
				"2": {ClassType: "AudioOptions", Inputs: map[string]any{"codec": "aac"}},
			},
			classTypes: []string{"VideoOptions", "AudioOptions"},
			keys:       []string{"codec"},
			wantVal:    "h264",
			wantOK:     true,
		},
		{
			name:       "empty graph",
			g:          workflow.Graph{},
			classTypes: []string{"ImageOptions"},
			keys:       []string{"size"},
			wantVal:    "",
			wantOK:     false,
		},
		{
			name: "blank value is skipped",
			g: workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "   "}},
			},
			classTypes: []string{"ImageOptions"},
			keys:       []string{"size"},
			wantVal:    "",
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := StringOptionFromClassTypes(tt.g, tt.classTypes, tt.keys...)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Fatalf("got %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestIntOptionFromClassTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		g          workflow.Graph
		classTypes []string
		keys       []string
		wantVal    int
		wantOK     bool
	}{
		{
			name: "valid int",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"width": 1920}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"width"},
			wantVal:    1920,
			wantOK:     true,
		},
		{
			name: "float64 coerced to int",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"width": float64(1920)}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"width"},
			wantVal:    1920,
			wantOK:     true,
		},
		{
			name: "missing key",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"height": 1080}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"width"},
			wantVal:    0,
			wantOK:     false,
		},
		{
			name: "wrong class type",
			g: workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"width": 1920}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"width"},
			wantVal:    0,
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := IntOptionFromClassTypes(tt.g, tt.classTypes, tt.keys...)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Fatalf("got %d, want %d", got, tt.wantVal)
			}
		})
	}
}

func TestFloat64OptionFromClassTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		g          workflow.Graph
		classTypes []string
		keys       []string
		wantVal    float64
		wantOK     bool
	}{
		{
			name: "float64 whole value",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5.0}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    5.0,
			wantOK:     true,
		},
		{
			name: "int coerced to float64",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 10}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    10.0,
			wantOK:     true,
		},
		{
			name: "string number parsed",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": "3.14"}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    3.14,
			wantOK:     true,
		},
		{
			name: "missing key",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"other": 5.0}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    0,
			wantOK:     false,
		},
		{
			name:       "empty graph",
			g:          workflow.Graph{},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    0,
			wantOK:     false,
		},
		{
			name: "unparseable string",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": "abc"}},
			},
			classTypes: []string{"VideoOptions"},
			keys:       []string{"duration"},
			wantVal:    0,
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Float64OptionFromClassTypes(tt.g, tt.classTypes, tt.keys...)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Fatalf("got %f, want %f", got, tt.wantVal)
			}
		})
	}
}

func TestIntOptionPreferVideoOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		g       workflow.Graph
		keys    []string
		wantVal int
		wantOK  bool
	}{
		{
			name: "prefers VideoOptions over other class types",
			g: workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"fps": 30}},
				"2": {ClassType: "VideoOptions", Inputs: map[string]any{"fps": 60}},
			},
			keys:    []string{"fps"},
			wantVal: 60,
			wantOK:  true,
		},
		{
			name: "falls back to flat option when no VideoOptions",
			g: workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"fps": 24}},
			},
			keys:    []string{"fps"},
			wantVal: 24,
			wantOK:  true,
		},
		{
			name:    "missing everywhere",
			g:       workflow.Graph{},
			keys:    []string{"fps"},
			wantVal: 0,
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := IntOptionPreferVideoOptions(tt.g, tt.keys...)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Fatalf("got %d, want %d", got, tt.wantVal)
			}
		})
	}
}
