package workflow

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestGraphValidate(t *testing.T) {
	t.Parallel()

	err := Graph{}.Validate()
	if !errors.Is(err, ErrEmptyGraph) {
		t.Fatalf("Validate() error = %v, want %v", err, ErrEmptyGraph)
	}

	err = Graph{
		"1": {ClassType: "CLIPTextEncode"},
	}.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestNodeIntInput(t *testing.T) {
	t.Parallel()

	node := Node{
		ClassType: "EmptyLatentImage",
		Inputs: map[string]any{
			"width":  json.Number("1024"),
			"height": "1536",
		},
	}

	width, ok := node.IntInput("width")
	if !ok || width != 1024 {
		t.Fatalf("IntInput(width) = (%d, %t), want (1024, true)", width, ok)
	}

	height, ok := node.IntInput("height")
	if !ok || height != 1536 {
		t.Fatalf("IntInput(height) = (%d, %t), want (1536, true)", height, ok)
	}
}

func TestFindByClassType(t *testing.T) {
	t.Parallel()

	graph := Graph{
		"2": {ClassType: "EmptyLatentImage"},
		"1": {ClassType: "CLIPTextEncode"},
	}

	refs := graph.FindByClassType("CLIPTextEncode")
	if len(refs) != 1 || refs[0].ID != "1" {
		t.Fatalf("FindByClassType() = %#v, want node id 1", refs)
	}
}

func TestGraphValidate_WhitespaceID(t *testing.T) {
	t.Parallel()

	err := Graph{"  ": {ClassType: "Foo"}}.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for whitespace-only node ID, got nil")
	}
	if got := err.Error(); got != "workflow: graph contains empty node id" {
		t.Fatalf("Validate() error = %q, want 'workflow: graph contains empty node id'", got)
	}
}

func TestGraphValidate_EmptyClassType(t *testing.T) {
	t.Parallel()

	err := Graph{"node1": {ClassType: ""}}.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty ClassType, got nil")
	}
	if got := err.Error(); got != `workflow: node "node1" is missing class_type` {
		t.Fatalf("Validate() error = %q, want mention of node1", got)
	}
}

func TestGraphValidate_MultipleValidNodes(t *testing.T) {
	t.Parallel()

	err := Graph{
		"1": {ClassType: "CLIPTextEncode"},
		"2": {ClassType: "EmptyLatentImage"},
		"3": {ClassType: "KSampler"},
	}.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestGraphSortedNodeIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		g    Graph
		want []string
	}{
		{
			name: "multiple unordered nodes",
			g: Graph{
				"c": {ClassType: "C"},
				"a": {ClassType: "A"},
				"b": {ClassType: "B"},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "single node",
			g:    Graph{"only": {ClassType: "X"}},
			want: []string{"only"},
		},
		{
			name: "empty graph",
			g:    Graph{},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.g.SortedNodeIDs()
			if len(got) != len(tc.want) {
				t.Fatalf("SortedNodeIDs() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("SortedNodeIDs()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestNodeStringInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		node   Node
		key    string
		wantS  string
		wantOK bool
	}{
		{
			name:   "valid string input",
			node:   Node{Inputs: map[string]any{"prompt": "hello"}},
			key:    "prompt",
			wantS:  "hello",
			wantOK: true,
		},
		{
			name:   "non-string value",
			node:   Node{Inputs: map[string]any{"count": 42}},
			key:    "count",
			wantS:  "",
			wantOK: false,
		},
		{
			name:   "nil inputs map",
			node:   Node{Inputs: nil},
			key:    "prompt",
			wantS:  "",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.node.StringInput(tc.key)
			if got != tc.wantS || ok != tc.wantOK {
				t.Fatalf("StringInput(%q) = (%q, %t), want (%q, %t)", tc.key, got, ok, tc.wantS, tc.wantOK)
			}
		})
	}
}

func TestNodeIntInput_AdditionalTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  any
		wantI  int
		wantOK bool
	}{
		{
			name:   "plain int",
			value:  42,
			wantI:  42,
			wantOK: true,
		},
		{
			name:   "int64",
			value:  int64(99),
			wantI:  99,
			wantOK: true,
		},
		{
			name:   "float32",
			value:  float32(7.0),
			wantI:  7,
			wantOK: true,
		},
		{
			name:   "float64 whole",
			value:  float64(42.0),
			wantI:  42,
			wantOK: true,
		},
		{
			name:   "float64 fractional truncates",
			value:  float64(3.7),
			wantI:  3,
			wantOK: true,
		},
		{
			name:   "unsupported type bool",
			value:  true,
			wantI:  0,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			node := Node{Inputs: map[string]any{"v": tc.value}}
			got, ok := node.IntInput("v")
			if got != tc.wantI || ok != tc.wantOK {
				t.Fatalf("IntInput() = (%d, %t), want (%d, %t)", got, ok, tc.wantI, tc.wantOK)
			}
		})
	}
}

func TestNodeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		node   Node
		key    string
		wantV  any
		wantOK bool
	}{
		{
			name:   "nil inputs map",
			node:   Node{Inputs: nil},
			key:    "x",
			wantV:  nil,
			wantOK: false,
		},
		{
			name:   "key exists with nil value",
			node:   Node{Inputs: map[string]any{"x": nil}},
			key:    "x",
			wantV:  nil,
			wantOK: true,
		},
		{
			name:   "key does not exist",
			node:   Node{Inputs: map[string]any{"a": 1}},
			key:    "missing",
			wantV:  nil,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.node.Input(tc.key)
			if got != tc.wantV || ok != tc.wantOK {
				t.Fatalf("Input(%q) = (%v, %t), want (%v, %t)", tc.key, got, ok, tc.wantV, tc.wantOK)
			}
		})
	}
}

func TestFindByClassType_NoMatch(t *testing.T) {
	t.Parallel()

	graph := Graph{
		"1": {ClassType: "CLIPTextEncode"},
		"2": {ClassType: "KSampler"},
	}
	refs := graph.FindByClassType("NonExistent")
	if len(refs) != 0 {
		t.Fatalf("FindByClassType() = %v, want empty slice", refs)
	}
}

func TestFindByClassType_MultipleMatches(t *testing.T) {
	t.Parallel()

	graph := Graph{
		"3": {ClassType: "CLIPTextEncode"},
		"1": {ClassType: "CLIPTextEncode"},
		"2": {ClassType: "KSampler"},
	}
	refs := graph.FindByClassType("CLIPTextEncode")
	if len(refs) != 2 {
		t.Fatalf("FindByClassType() got %d matches, want 2", len(refs))
	}
	// Results should be sorted by ID
	if refs[0].ID != "1" || refs[1].ID != "3" {
		t.Fatalf("FindByClassType() IDs = [%s, %s], want [1, 3]", refs[0].ID, refs[1].ID)
	}
}
