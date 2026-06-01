// Package graphx 从 workflow.Graph 抽取各域（图/视频/音频）共用字段。
package graphx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

func Prompt(graph workflow.Graph) (string, error) {
	for _, ref := range graph.FindByClassType("CLIPTextEncode") {
		prompt, ok, err := resolve.ResolveNodeString(graph, ref.ID, map[string]bool{})
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
	return resolve.StringOption(graph, keys...)
}

func IntOption(graph workflow.Graph, keys ...string) (int, bool) {
	return resolve.IntOption(graph, keys...)
}

func BoolOption(graph workflow.Graph, keys ...string) (bool, bool) {
	return resolve.BoolOption(graph, keys...)
}

func Size(graph workflow.Graph, fallback string) string {
	spec := resolve.ExtractImageSizeSpec(graph)
	if spec.IsZero() {
		spec = resolve.ExtractVideoSizeSpec(graph)
	}
	if s := spec.ToWAsteriskH(); s != "" {
		return s
	}
	return fallback
}

// NormalizeSize converts any size string to "W*H" (asterisk) as required by the aliyun API.
func NormalizeSize(s string) string {
	spec := resolve.ParseSize(s)
	if r := spec.ToWAsteriskH(); r != "" {
		return r
	}
	return strings.Replace(s, "x", "*", 1)
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
	spec := resolve.ExtractVideoSizeSpec(graph)
	if spec.Resolution != "" {
		return spec.Resolution, true
	}
	if r := spec.ToResolution(); r != "" {
		return r, true
	}
	return "", false
}

func DeriveResolution(graph workflow.Graph) (string, bool) {
	return Resolution(graph)
}

// appendUnique appends url to urls only if it has not been added before,
// preserving insertion order. Used by *URLs helpers to dedup graph inputs
// where the same asset URL may appear in multiple nodes.
func appendUnique(urls []string, seen map[string]struct{}, url string) []string {
	if _, dup := seen[url]; dup {
		return urls
	}
	seen[url] = struct{}{}
	return append(urls, url)
}

func ImageURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		if strings.Contains(strings.ToLower(node.ClassType), "image") {
			urls = appendUnique(urls, seen, url)
		}
	}
	return urls
}

func VideoURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		if strings.Contains(strings.ToLower(node.ClassType), "video") {
			urls = appendUnique(urls, seen, url)
		}
	}
	return urls
}

func MediaURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		classType := strings.ToLower(node.ClassType)
		if strings.Contains(classType, "video") || strings.Contains(classType, "image") {
			urls = appendUnique(urls, seen, url)
		}
	}
	return urls
}

// FirstFrameURL returns the URL from a node with ClassType containing
// "firstframe" or "first_frame", used by r2v models to specify a first frame
// alongside reference images.
func FirstFrameURL(graph workflow.Graph) (string, bool) {
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		ct := strings.ToLower(node.ClassType)
		if strings.Contains(ct, "firstframe") || strings.Contains(ct, "first_frame") {
			return url, true
		}
	}
	return "", false
}

func AudioURLs(graph workflow.Graph) []string {
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url == "" {
			continue
		}
		if strings.Contains(strings.ToLower(node.ClassType), "audio") {
			urls = appendUnique(urls, seen, url)
		}
	}
	return urls
}

// ReferenceVoiceForURL returns the reference_voice input associated with
// a media node that has the given URL. Used by r2v to attach voice cloning
// audio to reference_image / reference_video media objects.
func ReferenceVoiceForURL(graph workflow.Graph, mediaURL string) (string, bool) {
	for _, id := range graph.SortedNodeIDs() {
		node := graph[id]
		url, ok := node.StringInput("url")
		if !ok || url != mediaURL {
			continue
		}
		if voice, ok := node.StringInput("reference_voice"); ok && voice != "" {
			return voice, true
		}
	}
	return "", false
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
