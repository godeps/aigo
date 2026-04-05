// Package graphx 从 workflow.Graph 抽取各域（图/视频/音频）共用字段。
package graphx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
	"github.com/godeps/aigo/workflow"
)

func Prompt(graph workflow.Graph) (string, error) {
	for _, ref := range graph.FindByClassType("CLIPTextEncode") {
		prompt, ok, err := resolveNodeString(graph, ref.ID, map[string]bool{})
		if err != nil {
			return "", fmt.Errorf("aliyun: resolve prompt from node %q: %w", ref.ID, err)
		}
		if ok && strings.TrimSpace(prompt) != "" {
			return prompt, nil
		}
	}

	for _, key := range []string{"prompt", "text", "value"} {
		if value, ok := StringOption(graph, key); ok && strings.TrimSpace(value) != "" {
			return value, nil
		}
	}

	return "", ierr.ErrMissingPrompt
}

func StringOption(graph workflow.Graph, keys ...string) (string, bool) {
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		for _, key := range keys {
			if value, ok := node.StringInput(key); ok && strings.TrimSpace(value) != "" {
				return value, true
			}
		}
	}
	return "", false
}

func IntOption(graph workflow.Graph, keys ...string) (int, bool) {
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		for _, key := range keys {
			if value, ok := node.IntInput(key); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func BoolOption(graph workflow.Graph, keys ...string) (bool, bool) {
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
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

func Size(graph workflow.Graph, fallback string) string {
	if size, ok := StringOption(graph, "size"); ok {
		return size
	}
	if size, ok := WidthHeightSize(graph); ok {
		return size
	}
	return fallback
}

func WidthHeightSize(graph workflow.Graph) (string, bool) {
	for _, ref := range graph.FindByClassType("EmptyLatentImage") {
		width, okW := ref.Node.IntInput("width")
		height, okH := ref.Node.IntInput("height")
		if okW && okH {
			return fmt.Sprintf("%d*%d", width, height), true
		}
	}
	return "", false
}

func Resolution(graph workflow.Graph) (string, bool) {
	if resolution, ok := StringOption(graph, "resolution"); ok {
		return resolution, true
	}
	return DeriveResolution(graph)
}

func DeriveResolution(graph workflow.Graph) (string, bool) {
	if size, ok := StringOption(graph, "size"); ok {
		switch size {
		case "1280*720":
			return "720P", true
		case "1920*1080":
			return "1080P", true
		}
	}

	for _, ref := range graph.FindByClassType("EmptyLatentImage") {
		width, okW := ref.Node.IntInput("width")
		height, okH := ref.Node.IntInput("height")
		if !okW || !okH {
			continue
		}
		switch {
		case width >= 1920 || height >= 1080:
			return "1080P", true
		case width >= 1280 || height >= 720:
			return "720P", true
		}
	}
	return "", false
}

func ImageURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		if strings.Contains(strings.ToLower(node.ClassType), "image") {
			urls = append(urls, url)
		}
	}
	return urls
}

func MediaURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		classType := strings.ToLower(node.ClassType)
		if strings.Contains(classType, "video") || strings.Contains(classType, "image") {
			urls = append(urls, url)
		}
	}
	return urls
}

func VideoEditMedia(graph workflow.Graph) []map[string]any {
	media := make([]map[string]any, 0)
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		classType := strings.ToLower(node.ClassType)
		switch {
		case strings.Contains(classType, "video"):
			media = append(media, map[string]any{"type": "video", "url": url})
		case strings.Contains(classType, "image"):
			media = append(media, map[string]any{"type": "reference_image", "url": url})
		}
	}
	return media
}

func stringFromClassType(graph workflow.Graph, classType, inputKey string) (string, bool) {
	for _, ref := range graph.FindByClassType(classType) {
		if v, ok := ref.Node.StringInput(inputKey); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func AudioVoice(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "AudioOptions", "voice"); ok {
		return v, true
	}
	return StringOption(graph, "voice")
}

func AudioLanguageType(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "AudioOptions", "language_type"); ok {
		return v, true
	}
	return StringOption(graph, "language_type")
}

func AudioInstructions(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "AudioOptions", "instructions"); ok {
		return v, true
	}
	return StringOption(graph, "instructions")
}

func AudioOptimizeInstructions(graph workflow.Graph) (bool, bool) {
	for _, ref := range graph.FindByClassType("AudioOptions") {
		if v, ok := ref.Node.Input("optimize_instructions"); ok {
			switch t := v.(type) {
			case bool:
				return t, true
			case string:
				if parsed, err := strconv.ParseBool(t); err == nil {
					return parsed, true
				}
			}
		}
	}
	return BoolOption(graph, "optimize_instructions")
}

func VoiceDesignOmitPreview(graph workflow.Graph) bool {
	for _, ref := range graph.FindByClassType("VoiceDesignInput") {
		if v, ok := ref.Node.Input("omit_preview"); ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				if parsed, err := strconv.ParseBool(t); err == nil {
					return parsed
				}
			}
		}
	}
	if v, ok := BoolOption(graph, "omit_preview"); ok {
		return v
	}
	return false
}

func VoiceDesignFields(graph workflow.Graph) (voicePrompt, previewText, targetModel string, err error) {
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "voice_prompt"); ok {
		voicePrompt = v
	}
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "preview_text"); ok {
		previewText = v
	}
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "target_model"); ok {
		targetModel = v
	}
	if voicePrompt == "" {
		if v, ok := StringOption(graph, "voice_prompt"); ok {
			voicePrompt = v
		}
	}
	if previewText == "" {
		if v, ok := StringOption(graph, "preview_text"); ok {
			previewText = v
		}
	}
	if targetModel == "" {
		if v, ok := StringOption(graph, "target_model"); ok {
			targetModel = v
		}
	}
	if voicePrompt == "" || previewText == "" || targetModel == "" {
		return "", "", "", ierr.ErrMissingVoiceDesign
	}
	return voicePrompt, previewText, targetModel, nil
}

func VoiceDesignPreferredName(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "preferred_name"); ok {
		return v, true
	}
	return StringOption(graph, "preferred_name")
}

func VoiceDesignLanguage(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "language"); ok {
		return v, true
	}
	return StringOption(graph, "language")
}

func VoiceDesignSampleRate(graph workflow.Graph) (int, bool) {
	for _, ref := range graph.FindByClassType("VoiceDesignInput") {
		if n, ok := ref.Node.IntInput("sample_rate"); ok {
			return n, true
		}
	}
	return IntOption(graph, "sample_rate")
}

func VoiceDesignResponseFormat(graph workflow.Graph) (string, bool) {
	if v, ok := stringFromClassType(graph, "VoiceDesignInput", "response_format"); ok {
		return v, true
	}
	return StringOption(graph, "response_format")
}

func resolveNodeString(graph workflow.Graph, nodeID string, seen map[string]bool) (string, bool, error) {
	if seen[nodeID] {
		return "", false, fmt.Errorf("cycle detected at node %q", nodeID)
	}
	seen[nodeID] = true

	node, ok := graph[nodeID]
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
		resolved, ok, err := resolveValueString(graph, raw, seen)
		if err != nil {
			return "", false, err
		}
		if ok && strings.TrimSpace(resolved) != "" {
			return resolved, true, nil
		}
	}

	return "", false, nil
}

func resolveValueString(graph workflow.Graph, value any, seen map[string]bool) (string, bool, error) {
	switch v := value.(type) {
	case string:
		return v, true, nil
	case []any:
		return resolveLinkString(graph, v, seen)
	default:
		return "", false, nil
	}
}

func resolveLinkString(graph workflow.Graph, ref []any, seen map[string]bool) (string, bool, error) {
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
	return resolveNodeString(graph, nodeID, nextSeen)
}
