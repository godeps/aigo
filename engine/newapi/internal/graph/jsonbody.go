package graph

import (
	"encoding/json"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// MergeJSONObject 将图中 JSON 对象字符串合并进 dst（浅合并，嵌套不递归）。
func MergeJSONObject(g workflow.Graph, dst map[string]any, inputKeys ...string) error {
	for _, key := range inputKeys {
		raw, ok := StringOption(g, key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		var extra map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err != nil {
			return err
		}
		for k, v := range extra {
			dst[k] = v
		}
	}
	return nil
}

// RawJSONBody 若图中存在 request_body / generate_content_body 等整段 JSON，则返回其字节。
func RawJSONBody(g workflow.Graph) ([]byte, bool) {
	for _, key := range []string{"request_body", "generate_content_body", "gemini_body", "json_body"} {
		if raw, ok := StringOption(g, key); ok && strings.TrimSpace(raw) != "" {
			return []byte(raw), true
		}
	}
	return nil, false
}
