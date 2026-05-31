package resolve

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/godeps/aigo/workflow"
)

func ResolveNodeString(g workflow.Graph, nodeID string, seen map[string]bool) (string, bool, error) {
	if seen[nodeID] {
		return "", false, fmt.Errorf("cycle detected at node %q", nodeID)
	}
	seen[nodeID] = true

	node, ok := g[nodeID]
	if !ok {
		return "", false, fmt.Errorf("node %q not found", nodeID)
	}

	for _, key := range []string{"text", "prompt", "string", "value"} {
		if value, ok := node.StringInput(key); ok && strings.TrimSpace(value) != "" {
			return value, true, nil
		}
		raw, exists := node.Input(key)
		if !exists {
			continue
		}
		resolved, ok, err := ResolveValueString(g, raw, seen)
		if err != nil {
			return "", false, err
		}
		if ok && strings.TrimSpace(resolved) != "" {
			return resolved, true, nil
		}
	}

	return "", false, nil
}

func ResolveValueString(g workflow.Graph, value any, seen map[string]bool) (string, bool, error) {
	switch v := value.(type) {
	case string:
		return v, true, nil
	case []any:
		return ResolveLinkString(g, v, seen)
	default:
		return "", false, nil
	}
}

func ResolveLinkString(g workflow.Graph, ref []any, seen map[string]bool) (string, bool, error) {
	if len(ref) == 0 {
		return "", false, nil
	}

	nodeID, ok := ref[0].(string)
	if !ok {
		return "", false, nil
	}

	nextSeen := make(map[string]bool, len(seen))
	for k, v := range seen {
		nextSeen[k] = v
	}
	return ResolveNodeString(g, nodeID, nextSeen)
}

func StringOption(g workflow.Graph, keys ...string) (string, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if value, ok := node.StringInput(key); ok && strings.TrimSpace(value) != "" {
				return value, true
			}
		}
	}
	return "", false
}

func IntOption(g workflow.Graph, keys ...string) (int, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if value, ok := node.IntInput(key); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func BoolOption(g workflow.Graph, keys ...string) (bool, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			raw, ok := node.Input(key)
			if !ok {
				continue
			}
			switch value := raw.(type) {
			case bool:
				return value, true
			case string:
				if parsed, err := strconv.ParseBool(value); err == nil {
					return parsed, true
				}
			}
		}
	}
	return false, false
}

func Float64Option(g workflow.Graph, keys ...string) (float64, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if v, ok := node.IntInput(key); ok {
				return float64(v), true
			}
			raw, ok := node.Input(key)
			if !ok {
				continue
			}
			switch t := raw.(type) {
			case float64:
				return t, true
			case string:
				if f, err := strconv.ParseFloat(t, 64); err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

// ExtractPrompt extracts a text prompt from the workflow graph.
// It first checks CLIPTextEncode nodes (with link resolution), then falls back
// to common option keys: "prompt", "text", "value".
func ExtractPrompt(g workflow.Graph) (string, error) {
	for _, ref := range g.FindByClassType("CLIPTextEncode") {
		prompt, ok, err := ResolveNodeString(g, ref.ID, map[string]bool{})
		if err != nil {
			return "", fmt.Errorf("resolve prompt from node %q: %w", ref.ID, err)
		}
		if ok && strings.TrimSpace(prompt) != "" {
			return prompt, nil
		}
	}
	for _, key := range []string{"prompt", "text", "value"} {
		if value, ok := StringOption(g, key); ok && strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
	return "", nil
}

// MergeJSONOption searches for JSON-string inputs in the graph under the given
// keys and merges them into dst.
func MergeJSONOption(g workflow.Graph, dst map[string]any, keys ...string) {
	for _, key := range keys {
		raw, ok := StringOption(g, key)
		if !ok {
			continue
		}
		var extra map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err != nil {
			continue
		}
		for k, v := range extra {
			dst[k] = v
		}
	}
}

// StringSliceOption extracts a string slice from the graph under the given keys.
// It handles both []string and []any (JSON-decoded arrays).
func StringSliceOption(g workflow.Graph, keys ...string) ([]string, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			raw, ok := node.Input(key)
			if !ok || raw == nil {
				continue
			}
			switch v := raw.(type) {
			case []string:
				if len(v) > 0 {
					return v, true
				}
			case []any:
				result := make([]string, 0, len(v))
				for _, item := range v {
					if s, ok := item.(string); ok && s != "" {
						result = append(result, s)
					}
				}
				if len(result) > 0 {
					return result, true
				}
			}
		}
	}
	return nil, false
}

// Float64SliceOption extracts a float64 slice from the graph under the given keys.
func Float64SliceOption(g workflow.Graph, keys ...string) ([]float64, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			raw, ok := node.Input(key)
			if !ok || raw == nil {
				continue
			}
			if arr, ok := raw.([]any); ok && len(arr) > 0 {
				result := make([]float64, 0, len(arr))
				for _, item := range arr {
					switch n := item.(type) {
					case float64:
						result = append(result, n)
					case int:
						result = append(result, float64(n))
					case json.Number:
						if f, err := n.Float64(); err == nil {
							result = append(result, f)
						}
					}
				}
				if len(result) > 0 {
					return result, true
				}
			}
		}
	}
	return nil, false
}

func NormalizeOpenAIImageSize(width, height int) string {
	return NormalizeImageSize([]string{"1024x1024", "1024x1536", "1536x1024"}, width, height)
}
