package newapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/workflow"
)

const chatCompletionsTimeout = 5 * time.Minute

func (e *Engine) runChatCompletions(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, chatCompletionsTimeout)
		defer cancel()
	}
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}

	content := buildVisionContent(g, prompt)

	maxTokens := 4096
	if v, ok := graph.IntOption(g, "max_tokens"); ok && v > 0 {
		maxTokens = v
	}

	payload := map[string]any{
		"model":      e.model,
		"max_tokens": maxTokens,
		"stream":     true,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}
	_ = graph.MergeJSONObject(g, payload, "extra_body", "chat_extra")

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL("/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("newapi: build chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("newapi: chat http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", aigoerr.FromHTTPResponse(resp, out, "newapi")
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return parseChatSSE(resp.Body)
	}
	return parseChatJSON(resp.Body)
}

func parseChatJSON(r io.Reader) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return "", fmt.Errorf("newapi: decode chat response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("newapi: chat response contained no choices")
	}
	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("newapi: chat response had empty content")
	}
	return text, nil
}

func parseChatSSE(r io.Reader) (string, error) {
	var sb strings.Builder
	var parseErrors int
	var lastErr error
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			parseErrors++
			lastErr = err
			continue
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("newapi: read chat stream: %w", err)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		if lastErr != nil {
			return "", fmt.Errorf("newapi: chat stream produced empty content (%d parse errors, last: %w)", parseErrors, lastErr)
		}
		return "", errors.New("newapi: chat stream produced empty content")
	}
	return text, nil
}

// buildVisionContent constructs the messages content field supporting
// text, image_url, and video_url parts for multimodal understanding.
func buildVisionContent(g workflow.Graph, prompt string) any {
	imageRefs := g.FindByClassType("LoadImage")
	videoRefs := g.FindByClassType("LoadVideo")

	if len(imageRefs) == 0 && len(videoRefs) == 0 {
		return prompt
	}

	parts := []map[string]any{}

	for _, ref := range videoRefs {
		u, ok := ref.Node.Inputs["url"].(string)
		if !ok || u == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "video_url",
			"video_url": map[string]any{"url": u},
		})
	}

	for _, ref := range imageRefs {
		u, ok := ref.Node.Inputs["url"].(string)
		if !ok || u == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": u},
		})
	}

	parts = append(parts, map[string]any{
		"type": "text",
		"text": prompt,
	})

	if len(parts) == 1 {
		return prompt
	}

	return parts
}
