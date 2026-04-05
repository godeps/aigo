package graph

import (
	"strconv"
	"strings"

	"github.com/godeps/aigo/workflow"
)

func classSet(classTypes []string) map[string]struct{} {
	m := make(map[string]struct{}, len(classTypes))
	for _, ct := range classTypes {
		m[ct] = struct{}{}
	}
	return m
}

// StringOptionFromClassTypes 仅在指定 class_type 的节点上查找 keys（顺序：图中节点 id 序）。
func StringOptionFromClassTypes(g workflow.Graph, classTypes []string, keys ...string) (string, bool) {
	want := classSet(classTypes)
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if _, ok := want[node.ClassType]; !ok {
			continue
		}
		for _, key := range keys {
			if v, ok := node.StringInput(key); ok && strings.TrimSpace(v) != "" {
				return v, true
			}
		}
	}
	return "", false
}

// IntOptionFromClassTypes 仅在指定 class_type 的节点上查找整型 keys。
func IntOptionFromClassTypes(g workflow.Graph, classTypes []string, keys ...string) (int, bool) {
	want := classSet(classTypes)
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if _, ok := want[node.ClassType]; !ok {
			continue
		}
		for _, key := range keys {
			if v, ok := node.IntInput(key); ok {
				return v, true
			}
		}
	}
	return 0, false
}

// Float64OptionFromClassTypes 仅在指定 class_type 的节点上查找 keys（支持 int/float64/数字字符串）。
func Float64OptionFromClassTypes(g workflow.Graph, classTypes []string, keys ...string) (float64, bool) {
	want := classSet(classTypes)
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if _, ok := want[node.ClassType]; !ok {
			continue
		}
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

// IntOptionPreferVideoOptions 先查 VideoOptions 节点，再全图 IntOption。
func IntOptionPreferVideoOptions(g workflow.Graph, keys ...string) (int, bool) {
	if v, ok := IntOptionFromClassTypes(g, []string{"VideoOptions"}, keys...); ok {
		return v, true
	}
	return IntOption(g, keys...)
}
