// Package async 封装百炼异步任务创建与轮询（图生图、文生视频等共用）。
package async

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
)

// URLExtractor 描述从 task output 中取结果 URL 的路径。
type URLExtractor struct {
	URLFields [][]string
}

// Submit 创建异步任务并在配置为等待时轮询至完成。
func Submit(ctx context.Context, rt *runtime.RT, apiKey, path string, payload map[string]any, ex URLExtractor) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("aliyun: marshal async request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aliyun: build async request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := rt.HTTPClient.Do(req)
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
	if !rt.WaitForCompletion {
		return taskID, nil
	}

	return wait(ctx, rt, apiKey, taskID, ex)
}

func wait(ctx context.Context, rt *runtime.RT, apiKey, taskID string, ex URLExtractor) (string, error) {
	ticker := time.NewTicker(rt.PollInterval)
	defer ticker.Stop()

	for {
		url, done, err := fetch(ctx, rt, apiKey, taskID, ex)
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

func fetch(ctx context.Context, rt *runtime.RT, apiKey, taskID string, ex URLExtractor) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rt.BaseURL+"/tasks/"+taskID, nil)
	if err != nil {
		return "", false, fmt.Errorf("aliyun: build task query request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := rt.HTTPClient.Do(req)
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
		for _, path := range ex.URLFields {
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
