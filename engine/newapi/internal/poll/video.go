package poll

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VideoTask 与 New API 统一视频轮询、Kling 等 JSON 形态一致。
type VideoTask struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	Error  *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// ParseVideoJSON 解析任务 JSON：若已完成则 Done=true；若失败则 Err 非空。
func ParseVideoJSON(body []byte) (url string, done bool, err error) {
	var decoded VideoTask
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", false, fmt.Errorf("poll: decode video task: %w", err)
	}
	switch decoded.Status {
	case "queued", "in_progress", "":
		return "", false, nil
	case "completed":
		if u := strings.TrimSpace(decoded.URL); u != "" {
			return u, true, nil
		}
		return "", true, fmt.Errorf("poll: video completed but no url")
	case "failed":
		msg := "failed"
		if decoded.Error != nil && decoded.Error.Message != "" {
			msg = decoded.Error.Message
		}
		return "", true, fmt.Errorf("poll: video task failed: %s", msg)
	default:
		return "", false, nil
	}
}
