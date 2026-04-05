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
