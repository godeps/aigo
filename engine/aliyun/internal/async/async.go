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
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/engine/poll"
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
		return "", aigoerr.FromHTTPResponse(resp, respBody, "aliyun")
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

// Wait polls an already-submitted task to completion. Exported for engine.Resumer support.
func Wait(ctx context.Context, rt *runtime.RT, apiKey, taskID string, ex URLExtractor) (string, error) {
	return wait(ctx, rt, apiKey, taskID, ex)
}

func wait(ctx context.Context, rt *runtime.RT, apiKey, taskID string, ex URLExtractor) (string, error) {
	return poll.Poll(ctx, poll.Config{
		Interval:    rt.PollInterval,
		Backoff:     1.5,
		MaxInterval: 60 * time.Second,
	}, func(ctx context.Context) (string, bool, error) {
		url, done, err := fetch(ctx, rt, apiKey, taskID, ex)
		if err != nil {
			return "", false, fmt.Errorf("aliyun: wait for task %q: %w", taskID, err)
		}
		if done {
			if url == "" {
				return taskID, true, nil
			}
			return url, true, nil
		}
		return "", false, nil
	})
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
		return "", false, aigoerr.FromHTTPResponse(resp, body, "aliyun")
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
		// Extract error details from task output if available.
		errMsg := ""
		if code, _ := output["code"].(string); code != "" {
			errMsg += " code=" + code
		}
		if msg, _ := output["message"].(string); msg != "" {
			errMsg += " message=" + msg
		}
		return "", true, fmt.Errorf("aliyun: task %s finished with status %s%s", taskID, status, errMsg)
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
