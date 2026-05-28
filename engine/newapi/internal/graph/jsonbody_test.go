package graph

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestMergeJSONObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		g         workflow.Graph
		inputKeys []string
		init      map[string]any
		wantKey   string
		wantVal   any
		wantErr   bool
	}{
		{
			name: "valid JSON merges into dst",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"extra": `{"temperature":0.7,"top_p":0.9}`,
				}},
			},
			inputKeys: []string{"extra"},
			init:      map[string]any{"model": "gpt-4"},
			wantKey:   "temperature",
			wantVal:   0.7,
			wantErr:   false,
		},
		{
			name: "invalid JSON returns error",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"extra": `{not valid json}`,
				}},
			},
			inputKeys: []string{"extra"},
			init:      map[string]any{},
			wantErr:   true,
		},
		{
			name: "no matching key leaves dst unchanged",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"other": `{"foo":"bar"}`,
				}},
			},
			inputKeys: []string{"extra"},
			init:      map[string]any{"existing": "value"},
			wantKey:   "existing",
			wantVal:   "value",
			wantErr:   false,
		},
		{
			name:      "empty graph no error",
			g:         workflow.Graph{},
			inputKeys: []string{"extra"},
			init:      map[string]any{},
			wantErr:   false,
		},
		{
			name: "merge overwrites existing key",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"extra": `{"model":"gpt-3.5"}`,
				}},
			},
			inputKeys: []string{"extra"},
			init:      map[string]any{"model": "gpt-4"},
			wantKey:   "model",
			wantVal:   "gpt-3.5",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dst := make(map[string]any)
			for k, v := range tt.init {
				dst[k] = v
			}
			err := MergeJSONObject(tt.g, dst, tt.inputKeys...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantKey != "" {
				got, ok := dst[tt.wantKey]
				if !ok {
					t.Fatalf("key %q not found in dst", tt.wantKey)
				}
				if got != tt.wantVal {
					t.Fatalf("dst[%q] = %v, want %v", tt.wantKey, got, tt.wantVal)
				}
			}
		})
	}
}

func TestRawJSONBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		wantOK bool
		want   string
	}{
		{
			name: "graph with request_body",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"request_body": `{"messages":[]}`,
				}},
			},
			wantOK: true,
			want:   `{"messages":[]}`,
		},
		{
			name: "graph with gemini_body",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"gemini_body": `{"contents":[]}`,
				}},
			},
			wantOK: true,
			want:   `{"contents":[]}`,
		},
		{
			name: "graph with json_body",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"json_body": `{"data":"test"}`,
				}},
			},
			wantOK: true,
			want:   `{"data":"test"}`,
		},
		{
			name: "graph with generate_content_body",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"generate_content_body": `{"parts":[]}`,
				}},
			},
			wantOK: true,
			want:   `{"parts":[]}`,
		},
		{
			name: "no matching key returns false",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"prompt": "hello",
				}},
			},
			wantOK: false,
		},
		{
			name:   "empty graph returns false",
			g:      workflow.Graph{},
			wantOK: false,
		},
		{
			name: "blank value is skipped",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{
					"request_body": "   ",
				}},
			},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := RawJSONBody(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && string(got) != tt.want {
				t.Fatalf("got %q, want %q", string(got), tt.want)
			}
		})
	}
}
