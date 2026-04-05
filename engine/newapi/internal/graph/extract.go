package graph

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godeps/aigo/workflow"
)

func ExtractPrompt(g workflow.Graph) (string, error) {
	for _, ref := range g.FindByClassType("CLIPTextEncode") {
		prompt, ok, err := resolveNodeString(g, ref.ID, map[string]bool{})
		if err != nil {
			return "", fmt.Errorf("graph: resolve prompt from node %q: %w", ref.ID, err)
		}
		if ok && strings.TrimSpace(prompt) != "" {
			return prompt, nil
		}
	}
	for _, key := range []string{"prompt", "text", "value"} {
		if v, ok := StringOption(g, key); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
	}
	return "", ErrMissingPrompt
}

func StringOption(g workflow.Graph, keys ...string) (string, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if v, ok := node.StringInput(key); ok && strings.TrimSpace(v) != "" {
				return v, true
			}
		}
	}
	return "", false
}

func IntOption(g workflow.Graph, keys ...string) (int, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if v, ok := node.IntInput(key); ok {
				return v, true
			}
		}
	}
	return 0, false
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

func ExtractImageSizeOpenAI(g workflow.Graph) string {
	if s, ok := StringOptionFromClassTypes(g, []string{"ImageOptions"}, "size"); ok {
		return strings.ReplaceAll(s, "*", "x")
	}
	if s, ok := StringOption(g, "size"); ok {
		return strings.ReplaceAll(s, "*", "x")
	}
	for _, ref := range g.FindByClassType("EmptyLatentImage") {
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH {
			return normalizeOpenAIImageSize(w, h)
		}
	}
	return "1024x1024"
}

func normalizeOpenAIImageSize(width, height int) string {
	switch {
	case width == 1024 && height == 1024:
		return "1024x1024"
	case width == 1024 && height == 1536:
		return "1024x1536"
	case width == 1536 && height == 1024:
		return "1536x1024"
	case width > height:
		return "1536x1024"
	case height > width:
		return "1024x1536"
	default:
		return "1024x1024"
	}
}

// FirstImageURL 返回首张参考图 URL（图生视频等）。
func FirstImageURL(g workflow.Graph) (string, bool) {
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.StringInput("url"); ok && u != "" {
			return u, true
		}
	}
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if !strings.Contains(strings.ToLower(node.ClassType), "image") {
			continue
		}
		if u, ok := node.StringInput("url"); ok && u != "" {
			return u, true
		}
	}
	return "", false
}

func ExtractVideoDimensions(g workflow.Graph) (width, height int, ok bool) {
	for _, ref := range g.FindByClassType("VideoOptions") {
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH {
			return w, h, true
		}
	}
	if w, okW := IntOption(g, "width"); okW {
		if h, okH := IntOption(g, "height"); okH {
			return w, h, true
		}
	}
	for _, ref := range g.FindByClassType("EmptyLatentImage") {
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH {
			return w, h, true
		}
	}
	return 0, 0, false
}

func resolveNodeString(g workflow.Graph, nodeID string, seen map[string]bool) (string, bool, error) {
	if seen[nodeID] {
		return "", false, fmt.Errorf("cycle detected at node %q", nodeID)
	}
	seen[nodeID] = true

	node, ok := g[nodeID]
	if !ok {
		return "", false, fmt.Errorf("node %q not found", nodeID)
	}

	if value, ok := node.StringInput("text"); ok && strings.TrimSpace(value) != "" {
		return value, true, nil
	}

	for _, key := range []string{"text", "prompt", "string", "value"} {
		raw, exists := node.Input(key)
		if !exists {
			continue
		}
		resolved, ok, err := resolveValueString(g, raw, seen)
		if err != nil {
			return "", false, err
		}
		if ok && strings.TrimSpace(resolved) != "" {
			return resolved, true, nil
		}
	}

	return "", false, nil
}

func resolveValueString(g workflow.Graph, value any, seen map[string]bool) (string, bool, error) {
	switch v := value.(type) {
	case string:
		return v, true, nil
	case []any:
		return resolveLinkString(g, v, seen)
	default:
		return "", false, nil
	}
}

func ExtractSpeechVoice(g workflow.Graph) (string, bool) {
	for _, ref := range g.FindByClassType("AudioOptions") {
		if v, ok := ref.Node.StringInput("voice"); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return StringOption(g, "voice")
}

func ExtractSpeechResponseFormat(g workflow.Graph) string {
	for _, ref := range g.FindByClassType("AudioOptions") {
		if v, ok := ref.Node.StringInput("response_format"); ok && v != "" {
			return v
		}
	}
	if v, ok := StringOption(g, "response_format"); ok {
		return v
	}
	return "mp3"
}

func ExtractSpeechSpeed(g workflow.Graph) (float64, bool) {
	for _, ref := range g.FindByClassType("AudioOptions") {
		if raw, ok := ref.Node.Input("speed"); ok {
			switch t := raw.(type) {
			case float64:
				return t, true
			case int:
				return float64(t), true
			case string:
				if f, err := strconv.ParseFloat(t, 64); err == nil {
					return f, true
				}
			}
		}
	}
	return Float64Option(g, "speed")
}

// ExtractNegativePrompt 优先从 NegativePrompt 节点读取，再回退全图 stringOption。
func ExtractNegativePrompt(g workflow.Graph) (string, bool) {
	for _, ref := range g.FindByClassType("NegativePrompt") {
		if v, ok := ref.Node.StringInput("negative_prompt"); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return StringOption(g, "negative_prompt")
}

// ExtractVideoDuration 优先 VideoOptions.duration，再回退全图。
func ExtractVideoDuration(g workflow.Graph) (float64, bool) {
	if d, ok := Float64OptionFromClassTypes(g, []string{"VideoOptions"}, "duration"); ok && d > 0 {
		return d, true
	}
	if d, ok := IntOptionFromClassTypes(g, []string{"VideoOptions"}, "duration"); ok && d > 0 {
		return float64(d), true
	}
	if d, ok := Float64Option(g, "duration"); ok && d > 0 {
		return d, true
	}
	if d, ok := IntOption(g, "duration"); ok && d > 0 {
		return float64(d), true
	}
	return 0, false
}

func resolveLinkString(g workflow.Graph, ref []any, seen map[string]bool) (string, bool, error) {
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
	return resolveNodeString(g, nodeID, nextSeen)
}
