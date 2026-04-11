// Package graphx 从 workflow.Graph 抽取各域（图/视频/音频）共用字段。
package graphx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
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
	if size, ok := StringOption(graph, "size"); ok {
		return NormalizeSize(size)
	}
	if size, ok := WidthHeightSize(graph); ok {
		return size
	}
	return fallback
}

// NormalizeSize converts "WxH" (letter x) to "W*H" (asterisk) as required by the aliyun API.
func NormalizeSize(s string) string {
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
	if resolution, ok := StringOption(graph, "resolution"); ok {
		return resolution, true
	}
	return DeriveResolution(graph)
}

func DeriveResolution(graph workflow.Graph) (string, bool) {
	if size, ok := StringOption(graph, "size"); ok {
		switch NormalizeSize(size) {
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
