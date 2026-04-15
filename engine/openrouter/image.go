package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

// runImageGeneration generates an image via chat completions with image modality.
//
// Request format:
//
//	{
//	  "model": "openai/gpt-5-image",
//	  "messages": [{"role": "user", "content": "<prompt>"}],
//	  "modalities": ["text", "image"]
//	}
//
// Response: choices[0].message.content is an array of content blocks;
// we extract the first block with type "image_url".
func runImageGeneration(ctx context.Context, e *Engine, apiKey, model string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"modalities": []string{"text", "image"},
	}

	// Optional: include reference image for editing.
	if imgURL, ok := resolve.StringOption(graph, "image_url", "url"); ok && strings.TrimSpace(imgURL) != "" {
		payload["messages"] = []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]any{"url": strings.TrimSpace(imgURL)}},
				},
			},
		}
	}

	// Optional size hint.
	if size, ok := resolve.StringOption(graph, "size"); ok && strings.TrimSpace(size) != "" {
		payload["size"] = strings.TrimSpace(size)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal image request: %w", err)
	}

	respBody, err := doRequest(ctx, e.httpClient, http.MethodPost, e.baseURL+"/v1/chat/completions", apiKey, body)
	if err != nil {
		return "", err
	}
	return extractImageFromChat(respBody)
}

// extractImageFromChat parses a chat completion response and extracts the
// first image URL or base64 data URI from the content blocks.
//
// Response shapes handled:
//
//  1. content is array: [{"type":"image_url","image_url":{"url":"data:..."}}]
//  2. content is array: [{"type":"image_url","image_url":{"url":"https://..."}}]
//  3. Inline data (Gemini-style): content with inlineData containing base64
func extractImageFromChat(body []byte) (string, error) {
	var resp struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error,omitempty"`
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("openrouter: decode image response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("openrouter: api error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: image response had no choices")
	}

	raw := resp.Choices[0].Message.Content

	// Try as array of content blocks.
	var blocks []map[string]any
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if block["type"] == "image_url" {
				if imgObj, ok := block["image_url"].(map[string]any); ok {
					if url, ok := imgObj["url"].(string); ok && url != "" {
						return url, nil
					}
				}
			}
		}
	}

	// Try as plain string (some models return a URL directly).
	var text string
	if json.Unmarshal(raw, &text) == nil {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "data:") || strings.HasPrefix(text, "http") {
			return text, nil
		}
	}

	return "", fmt.Errorf("openrouter: no image found in chat response")
}
