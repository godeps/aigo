package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var ErrEmptyGraph = errors.New("workflow: graph is empty")

// Node is the simplified ComfyUI-compatible node payload.
type Node struct {
	ClassType string         `json:"class_type"`
	Inputs    map[string]any `json:"inputs,omitempty"`
}

// Graph is a simplified DAG keyed by node id.
type Graph map[string]Node

// NodeRef preserves both the node id and the node payload.
type NodeRef struct {
	ID   string
	Node Node
}

// Validate ensures the graph has the minimum shape expected by the SDK.
func (g Graph) Validate() error {
	if len(g) == 0 {
		return ErrEmptyGraph
	}

	for id, node := range g {
		if strings.TrimSpace(id) == "" {
			return errors.New("workflow: graph contains empty node id")
		}
		if strings.TrimSpace(node.ClassType) == "" {
			return fmt.Errorf("workflow: node %q is missing class_type", id)
		}
	}

	return nil
}

// SortedNodeIDs returns graph keys in deterministic order.
func (g Graph) SortedNodeIDs() []string {
	ids := make([]string, 0, len(g))
	for id := range g {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// FindByClassType returns all nodes matching the requested class type.
func (g Graph) FindByClassType(classType string) []NodeRef {
	refs := make([]NodeRef, 0)
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if node.ClassType == classType {
			refs = append(refs, NodeRef{ID: id, Node: node})
		}
	}
	return refs
}

// Input fetches a raw input value.
func (n Node) Input(name string) (any, bool) {
	if n.Inputs == nil {
		return nil, false
	}
	v, ok := n.Inputs[name]
	return v, ok
}

// StringInput extracts a string input when present.
func (n Node) StringInput(name string) (string, bool) {
	v, ok := n.Input(name)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// IntInput extracts an integer-like input when present.
func (n Node) IntInput(name string) (int, bool) {
	v, ok := n.Input(name)
	if !ok {
		return 0, false
	}
	i, err := asInt(v)
	if err != nil {
		return 0, false
	}
	return i, true
}

func asInt(v any) (int, error) {
	switch value := v.(type) {
	case int:
		return value, nil
	case int8:
		return int(value), nil
	case int16:
		return int(value), nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	case float32:
		return int(value), nil
	case float64:
		return int(value), nil
	case json.Number:
		i64, err := value.Int64()
		if err == nil {
			return int(i64), nil
		}
		f64, ferr := value.Float64()
		if ferr != nil {
			return 0, ferr
		}
		return int(f64), nil
	case string:
		i, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unsupported integer type %T", v)
	}
}
