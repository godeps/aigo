package ark

import (
	"encoding/json"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// extractPrompt extracts the text prompt from the workflow graph.
func extractPrompt(g workflow.Graph) string {
	for _, ref := range g.FindByClassType("CLIPTextEncode") {
		if v, ok := ref.Node.StringInput("text"); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	for _, key := range []string{"prompt", "text"} {
		if v, ok := stringOption(g, key); ok {
			return v
		}
	}
	return ""
}

// appendImages adds image_url content entries from LoadImage nodes.
func appendImages(g workflow.Graph, content []map[string]any) []map[string]any {
	for _, ref := range g.FindByClassType("LoadImage") {
		url, ok := ref.Node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		entry := map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		}
		if role, ok := ref.Node.StringInput("role"); ok && role != "" {
			entry["role"] = role
		}
		content = append(content, entry)
	}
	return content
}

// appendVideos adds video_url content entries from LoadVideo nodes.
func appendVideos(g workflow.Graph, content []map[string]any) []map[string]any {
	for _, ref := range g.FindByClassType("LoadVideo") {
		url, ok := ref.Node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		entry := map[string]any{
			"type":      "video_url",
			"video_url": map[string]any{"url": url},
		}
		if role, ok := ref.Node.StringInput("role"); ok && role != "" {
			entry["role"] = role
		} else {
			entry["role"] = "reference_video"
		}
		content = append(content, entry)
	}
	return content
}

// appendAudios adds audio_url content entries from LoadAudio nodes.
func appendAudios(g workflow.Graph, content []map[string]any) []map[string]any {
	for _, ref := range g.FindByClassType("LoadAudio") {
		url, ok := ref.Node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		entry := map[string]any{
			"type":      "audio_url",
			"audio_url": map[string]any{"url": url},
		}
		if role, ok := ref.Node.StringInput("role"); ok && role != "" {
			entry["role"] = role
		} else {
			entry["role"] = "reference_audio"
		}
		content = append(content, entry)
	}
	return content
}

// extractDuration returns the video duration from VideoOptions or top-level option.
func extractDuration(g workflow.Graph) int {
	for _, ref := range g.FindByClassType("VideoOptions") {
		if v, ok := ref.Node.IntInput("duration"); ok && v != 0 {
			return v
		}
	}
	if v, ok := intOption(g, "duration"); ok {
		return v
	}
	return 0
}

// stringOption searches for a string value across all graph nodes.
func stringOption(g workflow.Graph, keys ...string) (string, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			if v, ok := node.StringInput(key); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v), true
			}
		}
	}
	return "", false
}

// intOption searches for an integer value across all graph nodes.
func intOption(g workflow.Graph, keys ...string) (int, bool) {
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

// boolOption searches for a boolean value across all graph nodes.
func boolOption(g workflow.Graph, keys ...string) (bool, bool) {
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		for _, key := range keys {
			raw, ok := node.Input(key)
			if !ok {
				continue
			}
			switch v := raw.(type) {
			case bool:
				return v, true
			case float64:
				return v != 0, true
			case string:
				switch strings.ToLower(strings.TrimSpace(v)) {
				case "true", "1", "yes":
					return true, true
				case "false", "0", "no":
					return false, true
				}
			}
		}
	}
	return false, false
}

// mergeJSONOption merges JSON object strings from graph inputs into dst.
func mergeJSONOption(g workflow.Graph, dst map[string]any, keys ...string) {
	for _, key := range keys {
		raw, ok := stringOption(g, key)
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
