package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/alibabacloud"
)

// Router API style:
//   - "responses": OpenAI /v1/responses (structured output via text.format json_schema).
//   - "chat": OpenAI-compatible /v1/chat/completions (百炼、vLLM 等).
//
// 若未设置 AIGO_ROUTER_API，且 BaseURL 包含 "compatible-mode"，则默认使用 "chat"。
func main() {
	client := aigo.NewClient()

	must(client.RegisterEngine("alibabacloud-image", alibabacloud.New(alibabacloud.Config{
		Model: alibabacloud.ModelQwenImage,
	})))
	must(client.RegisterEngine("alibabacloud-video", alibabacloud.New(alibabacloud.Config{
		Model:             alibabacloud.ModelWanTextToVideo,
		WaitForCompletion: true,
		PollInterval:      15 * time.Second,
	})))

	baseURL := strings.TrimSpace(os.Getenv("AIGO_ROUTER_BASE_URL"))
	apiStyle := strings.TrimSpace(os.Getenv("AIGO_ROUTER_API"))
	if apiStyle == "" && strings.Contains(baseURL, "compatible-mode") {
		apiStyle = "chat"
	}
	// 仅配置了百炼 Key、未配置 OpenAI 路由 Key 时，默认走 chat/completions（百炼无 /v1/responses）。
	if apiStyle == "" && baseURL == "" &&
		os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("AGENT_OPENAI_API_KEY") == "" &&
		os.Getenv("DASHSCOPE_API_KEY") != "" {
		apiStyle = "chat"
	}
	if apiStyle == "" {
		apiStyle = "responses"
	}

	apiKey := firstEnv("AIGO_ROUTER_API_KEY", "DASHSCOPE_API_KEY", "OPENAI_API_KEY", "AGENT_OPENAI_API_KEY")
	defaultModel := envOr("AIGO_ROUTER_MODEL", "gpt-5-mini")
	if apiStyle == "chat" {
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		if os.Getenv("AIGO_ROUTER_MODEL") == "" && defaultModel == "gpt-5-mini" {
			defaultModel = "qwen3.6-plus"
		}
	}

	selector := &OpenAISelector{
		APIKey:  apiKey,
		Model:   defaultModel,
		BaseURL: baseURL,
		API:     apiStyle,
		Filter:  &aigo.RuleFilter{}, // Pre-filter by media type, size, duration constraints.
	}

	task := aigo.AgentTask{
		Prompt:   "make a 2 second cinematic ad video of a silver concept car driving through neon rain",
		Size:     "1280*720",
		Duration: 2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := client.ExecuteTaskAuto(ctx, selector, task)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("selected_engine=%s\nreason=%s\noutput=%s\n", result.Engine, result.Reason, result.Output)
}

type OpenAISelector struct {
	APIKey     string
	Model      string
	BaseURL    string
	// API is "responses" (OpenAI native) or "chat" (OpenAI-compatible chat completions).
	API        string
	HTTPClient *http.Client
	// Filter optionally pre-filters candidates by task constraints.
	Filter     *aigo.RuleFilter
}

// SelectEngine implements aigo.Selector (plain engine names, no capabilities).
func (s *OpenAISelector) SelectEngine(ctx context.Context, task aigo.AgentTask, engines []string) (aigo.Selection, error) {
	// Convert flat names to EngineInfo without capabilities.
	candidates := make([]aigo.EngineInfo, len(engines))
	for i, name := range engines {
		candidates[i] = aigo.EngineInfo{Name: name}
	}
	return s.SelectEngineFromCandidates(ctx, task, candidates)
}

// SelectEngineFromCandidates implements aigo.RichSelector — receives capability metadata.
func (s *OpenAISelector) SelectEngineFromCandidates(ctx context.Context, task aigo.AgentTask, candidates []aigo.EngineInfo) (aigo.Selection, error) {
	if s.APIKey == "" {
		return aigo.Selection{}, fmt.Errorf("missing API key (set DASHSCOPE_API_KEY or OPENAI_API_KEY)")
	}

	// Apply rule-based pre-filtering if configured.
	if s.Filter != nil {
		candidates = s.Filter.Filter(task, candidates)
	}
	if len(candidates) == 0 {
		return aigo.Selection{}, fmt.Errorf("no compatible engines after filtering")
	}

	// Build engine names and capability summary for the LLM.
	engines := make([]string, len(candidates))
	for i, c := range candidates {
		engines[i] = c.Name
	}
	capSummary := buildCapabilitySummary(candidates)

	switch strings.ToLower(strings.TrimSpace(s.API)) {
	case "", "responses":
		return s.selectViaResponses(ctx, task, engines, capSummary)
	case "chat":
		return s.selectViaChatCompletions(ctx, task, engines, capSummary)
	default:
		return aigo.Selection{}, fmt.Errorf("unknown AIGO_ROUTER_API %q (use responses or chat)", s.API)
	}
}

// buildCapabilitySummary formats engine capabilities as a concise text block for LLM context.
func buildCapabilitySummary(candidates []aigo.EngineInfo) string {
	var sb strings.Builder
	for _, c := range candidates {
		cap := c.Capability
		sb.WriteString(fmt.Sprintf("- %s:", c.Name))
		if len(cap.MediaTypes) > 0 {
			sb.WriteString(fmt.Sprintf(" types=%s", strings.Join(cap.MediaTypes, "/")))
		}
		if len(cap.Models) > 0 {
			sb.WriteString(fmt.Sprintf(" models=%s", strings.Join(cap.Models, ",")))
		}
		if cap.MaxDuration > 0 {
			sb.WriteString(fmt.Sprintf(" max_duration=%ds", cap.MaxDuration))
		}
		if len(cap.Sizes) > 0 {
			sb.WriteString(fmt.Sprintf(" sizes=%s", strings.Join(cap.Sizes, ",")))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (s *OpenAISelector) selectViaResponses(ctx context.Context, task aigo.AgentTask, engines []string, capSummary string) (aigo.Selection, error) {
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := s.Model
	if model == "" {
		model = "gpt-5-mini"
	}

	httpClient := s.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	promptBytes, err := json.Marshal(task)
	if err != nil {
		return aigo.Selection{}, err
	}

	requestBody := map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": "You route media-generation tasks to one engine from a provided allowlist. Choose exactly one engine. Use the engine capability summary to make the best match: consider media type, supported sizes, max duration, and available models. Prefer video engines when the task asks for animation, motion, clips, or duration. Prefer image engines otherwise.",
					},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": fmt.Sprintf("Available engines: %s\n\nEngine capabilities:\n%s\nTask JSON: %s", strings.Join(engines, ", "), capSummary, string(promptBytes)),
					},
				},
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "engine_selection",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"engine": map[string]any{
							"type": "string",
							"enum": engines,
						},
						"reason": map[string]any{
							"type": "string",
						},
					},
					"required":             []string{"engine", "reason"},
					"additionalProperties": false,
				},
			},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return aigo.Selection{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return aigo.Selection{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return aigo.Selection{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return aigo.Selection{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aigo.Selection{}, fmt.Errorf("openai selector failed with status %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return aigo.Selection{}, err
	}

	var selection aigo.Selection
	if err := json.Unmarshal([]byte(decoded.OutputText), &selection); err != nil {
		return aigo.Selection{}, fmt.Errorf("decode selector output: %w", err)
	}
	return selection, nil
}

func (s *OpenAISelector) selectViaChatCompletions(ctx context.Context, task aigo.AgentTask, engines []string, capSummary string) (aigo.Selection, error) {
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}

	model := strings.TrimSpace(s.Model)
	if model == "" {
		model = "qwen3.6-plus"
	}

	httpClient := s.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	promptBytes, err := json.Marshal(task)
	if err != nil {
		return aigo.Selection{}, err
	}

	system := "You route media-generation tasks to one engine from a provided allowlist. " +
		"Choose exactly one engine name from the list. " +
		"Use the engine capability summary to make the best match: consider media type, supported sizes, max duration, and available models. " +
		"Prefer video engines when the task asks for animation, motion, clips, or duration. " +
		"Prefer image engines otherwise. " +
		"Reply with a single JSON object only, keys: engine (string), reason (string). " +
		"The engine value must be exactly one of the allowed names."

	user := fmt.Sprintf("Allowed engines: %s\n\nEngine capabilities:\n%s\nTask JSON: %s", strings.Join(engines, ", "), capSummary, string(promptBytes))

	requestBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]any{
			"type": "json_object",
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return aigo.Selection{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return aigo.Selection{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return aigo.Selection{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return aigo.Selection{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aigo.Selection{}, fmt.Errorf("chat selector failed with status %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return aigo.Selection{}, err
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return aigo.Selection{}, fmt.Errorf("chat selector: empty choices/content")
	}

	raw := strings.TrimSpace(decoded.Choices[0].Message.Content)
	raw = stripMarkdownJSONFence(raw)

	var selection aigo.Selection
	if err := json.Unmarshal([]byte(raw), &selection); err != nil {
		return aigo.Selection{}, fmt.Errorf("decode chat selector JSON: %w (body=%s)", err, truncateForErr(raw, 500))
	}

	allowed := make(map[string]struct{}, len(engines))
	for _, name := range engines {
		allowed[name] = struct{}{}
	}
	if _, ok := allowed[selection.Engine]; !ok {
		return aigo.Selection{}, fmt.Errorf("model returned engine %q not in allowlist %v", selection.Engine, engines)
	}
	return selection, nil
}

func stripMarkdownJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), "json") {
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = strings.TrimSpace(s[idx+1:])
			}
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return strings.TrimSpace(s)
}

func truncateForErr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
