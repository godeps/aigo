package poll

import (
	"encoding/json"
	"fmt"
	"strings"

	sdkpoll "github.com/godeps/aigo/engine/poll"
)

// VideoTask 与 New API 统一视频轮询、Kling 等 JSON 形态一致。
type VideoTask struct {
	Status      string       `json:"status"`
	URL         string       `json:"url"`
	TaskMetrics *TaskMetrics `json:"task_metrics,omitempty"`
	Error       *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// TaskMetrics carries upstream progress information from DashScope-style APIs.
type TaskMetrics struct {
	Total     int `json:"TOTAL"`
	Succeeded int `json:"SUCCEEDED"`
	Failed    int `json:"FAILED"`
}

// ParseVideoJSON 解析任务 JSON：若已完成则 Done=true；若失败则 Err 非空。
func ParseVideoJSON(body []byte) (url string, done bool, err error) {
	r := ParseVideoJSONV2(body)
	return r.Result, r.Done, r.Err
}

// VideoParseResult holds the extended result from ParseVideoJSONV2.
type VideoParseResult struct {
	Result  string
	Done    bool
	Err     error
	Percent float64
}

// ParseVideoJSONV2 parses task JSON with extended progress info.
func ParseVideoJSONV2(body []byte) VideoParseResult {
	var decoded VideoTask
	if err := json.Unmarshal(body, &decoded); err != nil {
		return VideoParseResult{Err: fmt.Errorf("poll: decode video task: %w", err)}
	}

	var percent float64
	if decoded.TaskMetrics != nil && decoded.TaskMetrics.Total > 0 {
		percent = float64(decoded.TaskMetrics.Succeeded) / float64(decoded.TaskMetrics.Total)
	}

	switch decoded.Status {
	case "queued", "in_progress", "":
		return VideoParseResult{Percent: percent}
	case "completed":
		if u := strings.TrimSpace(decoded.URL); u != "" {
			return VideoParseResult{Result: u, Done: true, Percent: 1.0}
		}
		return VideoParseResult{Done: true, Err: fmt.Errorf("poll: video completed but no url")}
	case "failed":
		msg := "failed"
		if decoded.Error != nil && decoded.Error.Message != "" {
			msg = decoded.Error.Message
		}
		return VideoParseResult{Done: true, Err: fmt.Errorf("poll: video task failed: %s", msg)}
	default:
		return VideoParseResult{Percent: percent}
	}
}

// ToFetchResult converts a VideoParseResult to a poll.FetchResult for use with PollV2.
func (r VideoParseResult) ToFetchResult() (sdkpoll.FetchResult, error) {
	return sdkpoll.FetchResult{
		Result:  r.Result,
		Done:    r.Done,
		Percent: r.Percent,
	}, r.Err
}
