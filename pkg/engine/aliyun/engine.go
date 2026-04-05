package aliyun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godeps/aigo/pkg/workflow"
)

const (
	defaultBaseURL      = "https://dashscope.aliyuncs.com/api/v1"
	defaultPollInterval = 15 * time.Second

	ModelQwenImage         = "qwen-image"
	ModelWanImage          = "wan2.7-image"
	ModelZImageTurbo       = "z-image-turbo"
	ModelWanTextToVideo    = "wan2.6-t2v"
	ModelWanReferenceVideo = "wan2.6-r2v"
	ModelWanVideoEdit      = "wan2.7-videoedit"
)

var (
	ErrMissingPrompt    = errors.New("aliyun: prompt not found in workflow graph")
	ErrMissingReference = errors.New("aliyun: reference media not found in workflow graph")
	ErrMissingAPIKey    = errors.New("aliyun: missing API key")
	ErrUnsupportedModel = errors.New("aliyun: unsupported model")
)

// Config configures the Alibaba Cloud Bailian engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine compiles a workflow graph into a Bailian image or video request.
type Engine struct {
	apiKey            string
	baseURL           string
	model             string
	httpClient        *http.Client
	waitForCompletion bool
	pollInterval      time.Duration
}

// New creates a Bailian execution engine.
func New(cfg Config) *Engine {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelQwenImage
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	return &Engine{
		apiKey:            cfg.APIKey,
		baseURL:           baseURL,
		model:             model,
		httpClient:        httpClient,
		waitForCompletion: cfg.WaitForCompletion,
		pollInterval:      pollInterval,
	}
}

// Execute compiles the workflow graph into the configured Bailian model request.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (string, error) {
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("aliyun: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return "", ErrMissingAPIKey
	}

	switch {
	case isQwenImageModel(e.model):
		return e.executeQwenImage(ctx, apiKey, graph)
	case isWanImageModel(e.model):
		return e.executeWanImage(ctx, apiKey, graph)
	case isWanVideoEditModel(e.model):
		return e.executeWanVideoEdit(ctx, apiKey, graph)
	case isWanReferenceVideoModel(e.model):
		return e.executeWanReferenceVideo(ctx, apiKey, graph)
	case isWanTextToVideoModel(e.model):
		return e.executeWanTextToVideo(ctx, apiKey, graph)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedModel, e.model)
	}
}

func (e *Engine) executeQwenImage(ctx context.Context, apiKey string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{
		"prompt": prompt,
	}
	if negativePrompt, ok := extractStringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := map[string]any{
		"size": extractSize(graph, "1024*1024"),
	}
	if n, ok := extractIntOption(graph, "n"); ok {
		parameters["n"] = n
	}
	if watermark, ok := extractBoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if promptExtend, ok := extractBoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}
	if seed, ok := extractIntOption(graph, "seed"); ok {
		parameters["seed"] = seed
	}

	payload := map[string]any{
		"model":      e.model,
		"input":      input,
		"parameters": parameters,
	}

	result, err := e.submitAsyncTask(ctx, apiKey, "/services/aigc/text2image/image-synthesis", payload, taskResultExtractor{
		urlFields: [][]string{{"results", "url"}, {"result_url"}},
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

func (e *Engine) executeWanImage(ctx context.Context, apiKey string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	content := []map[string]any{
		{"text": prompt},
	}
	for _, imageURL := range extractImageURLs(graph) {
		content = append(content, map[string]any{"image": imageURL})
	}

	input := map[string]any{
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	parameters := map[string]any{}
	if size, ok := extractStringOption(graph, "size"); ok {
		parameters["size"] = size
	}
	if size, ok := extractWidthHeightSize(graph); ok {
		parameters["size"] = size
	}
	if watermark, ok := extractBoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if promptExtend, ok := extractBoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}
	if thinkingMode, ok := extractBoolOption(graph, "thinking_mode"); ok {
		parameters["thinking_mode"] = thinkingMode
	}
	if n, ok := extractIntOption(graph, "n"); ok {
		parameters["n"] = n
	}

	payload := map[string]any{
		"model": e.model,
		"input": input,
	}
	if len(parameters) > 0 {
		payload["parameters"] = parameters
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal wan image request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/services/aigc/multimodal-generation/generation", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build wan image request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: call wan image api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read wan image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("aliyun: wan image request failed with status %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Type  string `json:"type"`
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("aliyun: decode wan image response: %w", err)
	}
	for _, choice := range decoded.Output.Choices {
		for _, item := range choice.Message.Content {
			if item.Image != "" {
				return item.Image, nil
			}
		}
	}

	return "", errors.New("aliyun: wan image response did not contain an image URL")
}

func (e *Engine) executeWanTextToVideo(ctx context.Context, apiKey string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	input := map[string]any{"prompt": prompt}
	if negativePrompt, ok := extractStringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := buildWanVideoParameters(graph, false)
	payload := map[string]any{
		"model":      e.model,
		"input":      input,
		"parameters": parameters,
	}

	return e.submitAsyncTask(ctx, apiKey, "/services/aigc/video-generation/video-synthesis", payload, taskResultExtractor{
		urlFields: [][]string{{"video_url"}},
	})
}

func (e *Engine) executeWanReferenceVideo(ctx context.Context, apiKey string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	referenceURLs := extractMediaURLs(graph)
	if len(referenceURLs) == 0 {
		return "", ErrMissingReference
	}

	input := map[string]any{
		"prompt":         prompt,
		"reference_urls": referenceURLs,
	}
	if negativePrompt, ok := extractStringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := buildWanVideoParameters(graph, false)
	payload := map[string]any{
		"model":      e.model,
		"input":      input,
		"parameters": parameters,
	}

	return e.submitAsyncTask(ctx, apiKey, "/services/aigc/video-generation/video-synthesis", payload, taskResultExtractor{
		urlFields: [][]string{{"video_url"}},
	})
}

func (e *Engine) executeWanVideoEdit(ctx context.Context, apiKey string, graph workflow.Graph) (string, error) {
	prompt, err := extractPrompt(graph)
	if err != nil {
		return "", err
	}

	media := extractVideoEditMedia(graph)
	if len(media) == 0 {
		return "", ErrMissingReference
	}

	input := map[string]any{
		"prompt": prompt,
		"media":  media,
	}
	if negativePrompt, ok := extractStringOption(graph, "negative_prompt"); ok {
		input["negative_prompt"] = negativePrompt
	}

	parameters := buildWanVideoParameters(graph, true)
	payload := map[string]any{
		"model":      e.model,
		"input":      input,
		"parameters": parameters,
	}

	return e.submitAsyncTask(ctx, apiKey, "/services/aigc/video-generation/video-synthesis", payload, taskResultExtractor{
		urlFields: [][]string{{"video_url"}},
	})
}

type taskResultExtractor struct {
	urlFields [][]string
}

func (e *Engine) submitAsyncTask(ctx context.Context, apiKey string, path string, payload map[string]any, extractor taskResultExtractor) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal async request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build async request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: create async task: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read task creation response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("aliyun: async task creation failed with status %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	taskID, err := parseTaskID(respBody)
	if err != nil {
		return "", err
	}
	if !e.waitForCompletion {
		return taskID, nil
	}

	return e.waitForTask(ctx, apiKey, taskID, extractor)
}

func (e *Engine) waitForTask(ctx context.Context, apiKey string, taskID string, extractor taskResultExtractor) (string, error) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		url, done, err := e.fetchTask(ctx, apiKey, taskID, extractor)
		if err != nil {
			return "", err
		}
		if done {
			if url == "" {
				return taskID, nil
			}
			return url, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("aliyun: wait for task %q: %w", taskID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (e *Engine) fetchTask(ctx context.Context, apiKey string, taskID string, extractor taskResultExtractor) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/tasks/"+taskID, nil)
	if err != nil {
		return "", false, fmt.Errorf("aliyun: build task query request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("aliyun: query task: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("aliyun: read task query response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("aliyun: task query failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false, fmt.Errorf("aliyun: decode task query response: %w", err)
	}

	output, _ := result["output"].(map[string]any)
	status, _ := output["task_status"].(string)
	switch status {
	case "PENDING", "RUNNING", "":
		return "", false, nil
	case "FAILED", "CANCELED", "UNKNOWN":
		return "", true, fmt.Errorf("aliyun: task %s finished with status %s", taskID, status)
	case "SUCCEEDED":
		for _, path := range extractor.urlFields {
			if url, ok := nestedString(output, path...); ok && url != "" {
				return url, true, nil
			}
		}
		return "", true, nil
	default:
		return "", false, nil
	}
}

func parseTaskID(body []byte) (string, error) {
	var decoded struct {
		Output struct {
			TaskID string `json:"task_id"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("aliyun: decode task creation response: %w", err)
	}
	if decoded.Output.TaskID == "" {
		return "", errors.New("aliyun: task creation response did not include task_id")
	}
	return decoded.Output.TaskID, nil
}

func buildWanVideoParameters(graph workflow.Graph, preferResolution bool) map[string]any {
	parameters := map[string]any{}

	if preferResolution {
		if resolution, ok := extractResolution(graph); ok {
			parameters["resolution"] = resolution
		}
	} else {
		if size, ok := extractStringOption(graph, "size"); ok {
			parameters["size"] = size
		} else if size, ok := extractWidthHeightSize(graph); ok {
			parameters["size"] = size
		}
	}

	if preferResolution {
		if size, exists := parameters["resolution"]; !exists {
			if resolution, ok := deriveResolutionFromGraph(graph); ok {
				parameters["resolution"] = resolution
			}
		} else if _, ok := size.(string); !ok {
			delete(parameters, "resolution")
		}
	}

	if duration, ok := extractIntOption(graph, "duration"); ok {
		parameters["duration"] = duration
	}
	if watermark, ok := extractBoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if audio, ok := extractBoolOption(graph, "audio"); ok && !preferResolution {
		parameters["audio"] = audio
	}
	if shotType, ok := extractStringOption(graph, "shot_type"); ok && !preferResolution {
		parameters["shot_type"] = shotType
	}
	if promptExtend, ok := extractBoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}

	if len(parameters) == 0 {
		if preferResolution {
			parameters["resolution"] = "720P"
		} else {
			parameters["size"] = "1280*720"
		}
	}

	return parameters
}

func extractPrompt(graph workflow.Graph) (string, error) {
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
		if value, ok := extractStringOption(graph, key); ok && strings.TrimSpace(value) != "" {
			return value, nil
		}
	}

	return "", ErrMissingPrompt
}

func extractStringOption(graph workflow.Graph, keys ...string) (string, bool) {
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

func extractIntOption(graph workflow.Graph, keys ...string) (int, bool) {
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

func extractBoolOption(graph workflow.Graph, keys ...string) (bool, bool) {
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

func extractSize(graph workflow.Graph, fallback string) string {
	if size, ok := extractStringOption(graph, "size"); ok {
		return size
	}
	if size, ok := extractWidthHeightSize(graph); ok {
		return size
	}
	return fallback
}

func extractWidthHeightSize(graph workflow.Graph) (string, bool) {
	for _, ref := range graph.FindByClassType("EmptyLatentImage") {
		width, okW := ref.Node.IntInput("width")
		height, okH := ref.Node.IntInput("height")
		if okW && okH {
			return fmt.Sprintf("%d*%d", width, height), true
		}
	}
	return "", false
}

func extractResolution(graph workflow.Graph) (string, bool) {
	if resolution, ok := extractStringOption(graph, "resolution"); ok {
		return resolution, true
	}
	return deriveResolutionFromGraph(graph)
}

func deriveResolutionFromGraph(graph workflow.Graph) (string, bool) {
	if size, ok := extractStringOption(graph, "size"); ok {
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

func extractImageURLs(graph workflow.Graph) []string {
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

func extractMediaURLs(graph workflow.Graph) []string {
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

func extractVideoEditMedia(graph workflow.Graph) []map[string]any {
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

func nestedString(value any, path ...string) (string, bool) {
	current := value
	for _, key := range path {
		for {
			list, ok := current.([]any)
			if !ok {
				break
			}
			if len(list) == 0 {
				return "", false
			}
			current = list[0]
		}

		object, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := object[key]
		if !ok {
			return "", false
		}
		current = next
	}

	for {
		list, ok := current.([]any)
		if !ok {
			break
		}
		if len(list) == 0 {
			return "", false
		}
		current = list[0]
	}

	text, ok := current.(string)
	return text, ok
}

func isQwenImageModel(model string) bool {
	return strings.HasPrefix(model, "qwen-image")
}

func isWanImageModel(model string) bool {
	return (strings.Contains(model, "image") || strings.HasPrefix(model, "z-image")) && !strings.Contains(model, "video")
}

func isWanTextToVideoModel(model string) bool {
	return strings.Contains(model, "-t2v")
}

func isWanReferenceVideoModel(model string) bool {
	return strings.Contains(model, "-r2v")
}

func isWanVideoEditModel(model string) bool {
	return strings.Contains(model, "videoedit")
}
