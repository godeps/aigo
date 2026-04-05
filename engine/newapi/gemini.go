package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/workflow"
)

func (e *Engine) runGeminiGenerateContent(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	path := fmt.Sprintf("/v1beta/models/%s:generateContent", e.model)
	full := e.apiURL(path)

	var body []byte
	if raw, ok := graph.RawJSONBody(g); ok {
		body = raw
	} else {
		prompt, err := graph.ExtractPrompt(g)
		if err != nil {
			return "", wrapGraphErr(err)
		}
		payload := map[string]any{
			"contents": []any{
				map[string]any{
					"role": "user",
					"parts": []any{
						map[string]any{"text": prompt},
					},
				},
			},
		}
		_ = graph.MergeJSONObject(g, payload, "gemini_extra", "extra_body")
		b, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		body = b
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, full, apiKey, body, "application/json")
	if err != nil {
		return "", err
	}
	return geminiExtractOutput(respBody)
}

func geminiExtractOutput(respBody []byte) (string, error) {
	var root map[string]any
	if err := json.Unmarshal(respBody, &root); err != nil {
		return "", fmt.Errorf("newapi: gemini decode: %w", err)
	}
	if u := deepGeminiURL(root); u != "" {
		return u, nil
	}
	if data := deepGeminiInlineData(root); data.mime != "" && data.b64 != "" {
		return "data:" + data.mime + ";base64," + data.b64, nil
	}
	if t := deepGeminiText(root); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("newapi: gemini response had no url, inlineData, or text: %s", truncate(string(respBody), 500))
}

type inlinePair struct {
	mime, b64 string
}

func deepGeminiInlineData(v any) inlinePair {
	switch t := v.(type) {
	case map[string]any:
		if id, ok := t["inlineData"].(map[string]any); ok {
			mime, _ := id["mimeType"].(string)
			if mime == "" {
				mime, _ = id["mime_type"].(string)
			}
			b64, _ := id["data"].(string)
			if b64 != "" {
				return inlinePair{mime: mime, b64: b64}
			}
		}
		for _, child := range t {
			if p := deepGeminiInlineData(child); p.b64 != "" {
				return p
			}
		}
	case []any:
		for _, it := range t {
			if p := deepGeminiInlineData(it); p.b64 != "" {
				return p
			}
		}
	}
	return inlinePair{}
}

func deepGeminiURL(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok && strings.HasPrefix(s, "http") &&
				(strings.Contains(strings.ToLower(k), "url") || strings.Contains(s, "://")) {
				return s
			}
		}
		for _, child := range t {
			if u := deepGeminiURL(child); u != "" {
				return u
			}
		}
	case []any:
		for _, it := range t {
			if u := deepGeminiURL(it); u != "" {
				return u
			}
		}
	}
	return ""
}

func deepGeminiText(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if txt, ok := t["text"].(string); ok && strings.TrimSpace(txt) != "" {
			return txt
		}
		for _, child := range t {
			if s := deepGeminiText(child); s != "" {
				return s
			}
		}
	case []any:
		for _, it := range t {
			if s := deepGeminiText(it); s != "" {
				return s
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
