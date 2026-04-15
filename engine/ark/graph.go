package ark

import (
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

// extractPrompt extracts the text prompt from the workflow graph.
func extractPrompt(g workflow.Graph) string {
	p, _ := resolve.ExtractPrompt(g)
	return p
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
	if v, ok := resolve.IntOption(g, "duration"); ok {
		return v
	}
	return 0
}

// stringOption delegates to resolve.StringOption.
func stringOption(g workflow.Graph, keys ...string) (string, bool) {
	return resolve.StringOption(g, keys...)
}

// intOption delegates to resolve.IntOption.
func intOption(g workflow.Graph, keys ...string) (int, bool) {
	return resolve.IntOption(g, keys...)
}

// boolOption delegates to resolve.BoolOption.
func boolOption(g workflow.Graph, keys ...string) (bool, bool) {
	return resolve.BoolOption(g, keys...)
}

// mergeJSONOption delegates to resolve.MergeJSONOption.
func mergeJSONOption(g workflow.Graph, dst map[string]any, keys ...string) {
	resolve.MergeJSONOption(g, dst, keys...)
}
