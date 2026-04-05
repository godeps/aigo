package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/workflow"
)

const (
	jimengSubmitAction = "CVSync2AsyncSubmitTask"
	jimengGetAction    = "CVSync2AsyncGetResult"
)

func (e *Engine) runJimengVideo(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	reqKey, ok := graph.StringOption(g, "req_key", "jimeng_req_key")
	if !ok || strings.TrimSpace(reqKey) == "" {
		return "", ErrMissingJimengReqKey
	}
	submitBody := map[string]any{}
	_ = graph.MergeJSONObject(g, submitBody, "jimeng_submit_extra")
	submitBody["req_key"] = strings.TrimSpace(reqKey)
	submitBody["prompt"] = prompt
	if b64s, ok := graph.StringOption(g, "binary_data_base64"); ok && b64s != "" {
		submitBody["binary_data_base64"] = []string{b64s}
	}

	subBody, err := jsonBody(submitBody)
	if err != nil {
		return "", fmt.Errorf("newapi: jimeng submit marshal: %w", err)
	}
	subURL := jimengURL(e.apiURL("/jimeng/"), jimengSubmitAction, e.jimengVer)
	subRaw, err := e.doRequest(ctx, http.MethodPost, subURL, apiKey, subBody, "application/json")
	if err != nil {
		return "", err
	}
	taskID, jerr := jimengParseTaskID(subRaw)
	if jerr != nil {
		return "", jerr
	}
	if !e.waitVideo {
		return taskID, nil
	}
	return e.pollJimeng(ctx, apiKey, g, taskID, reqKey)
}

func jimengURL(base, action, version string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + "?Action=" + url.QueryEscape(action) + "&Version=" + url.QueryEscape(version)
	}
	q := u.Query()
	q.Set("Action", action)
	q.Set("Version", version)
	u.RawQuery = q.Encode()
	return u.String()
}

func jsonBody(v any) ([]byte, error) {
	return json.Marshal(v)
}

type jimengEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func jimengParseTaskID(body []byte) (string, error) {
	var env jimengEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("newapi: jimeng submit decode: %w", err)
	}
	if env.Code != 0 {
		return "", fmt.Errorf("newapi: jimeng submit code=%d msg=%s", env.Code, env.Message)
	}
	var dataObj map[string]any
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &dataObj); err != nil {
			return "", fmt.Errorf("newapi: jimeng data: %w", err)
		}
	}
	id := deepFindTaskID(dataObj)
	if id == "" {
		return "", fmt.Errorf("newapi: jimeng submit missing task id in data: %s", strings.TrimSpace(string(env.Data)))
	}
	return id, nil
}

func deepFindTaskID(m map[string]any) string {
	if m == nil {
		return ""
	}
	for _, k := range []string{"task_id", "TaskId", "taskId", "Id", "id"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	for _, v := range m {
		switch t := v.(type) {
		case map[string]any:
			if s := deepFindTaskID(t); s != "" {
				return s
			}
		case []any:
			for _, it := range t {
				if mm, ok := it.(map[string]any); ok {
					if s := deepFindTaskID(mm); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func (e *Engine) pollJimeng(ctx context.Context, apiKey string, g workflow.Graph, taskID, reqKey string) (string, error) {
	getURL := jimengURL(e.apiURL("/jimeng/"), jimengGetAction, e.jimengVer)
	getBody := map[string]any{}
	_ = graph.MergeJSONObject(g, getBody, "jimeng_get_extra")
	getBody["req_key"] = strings.TrimSpace(reqKey)
	getBody["task_id"] = taskID
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()
	for {
		gb, jerr := jsonBody(getBody)
		if jerr != nil {
			return "", fmt.Errorf("newapi: jimeng get marshal: %w", jerr)
		}
		raw, err := e.doRequest(ctx, http.MethodPost, getURL, apiKey, gb, "application/json")
		if err != nil {
			return "", err
		}
		url, done, perr := jimengParseResultURL(raw)
		if perr != nil {
			return "", perr
		}
		if done {
			return url, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("newapi: jimeng wait %q: %w", taskID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func jimengParseResultURL(body []byte) (mediaURL string, done bool, err error) {
	var env jimengEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", false, fmt.Errorf("newapi: jimeng get decode: %w", err)
	}
	if env.Code != 0 {
		if jimengNonZeroContinuePoll(env.Code, env.Message) {
			return "", false, nil
		}
		return "", true, fmt.Errorf("newapi: jimeng get code=%d msg=%s", env.Code, env.Message)
	}
	var dataObj map[string]any
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &dataObj); err != nil {
			return "", false, fmt.Errorf("newapi: jimeng get data: %w", err)
		}
	}
	if u := deepFindHTTPURL(dataObj); u != "" {
		return u, true, nil
	}
	// code==0 但尚无 URL：任务可能仍在处理
	return "", false, nil
}

// jimengNonZeroContinuePoll 在轮询 GetResult 时，部分网关用非 0 code 或文案表示「仍在处理」而非终态失败。
func jimengNonZeroContinuePoll(code int, msg string) bool {
	switch code {
	case 429, 503, 504:
		return true
	}
	m := strings.ToLower(strings.TrimSpace(msg))
	for _, k := range []string{
		"process", "running", "pending", "queue", "wait", "async",
		"处理", "进行", "排队", "等待", "未就绪", "not ready", "submitted",
	} {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}

func deepFindHTTPURL(m map[string]any) string {
	if m == nil {
		return ""
	}
	for k, v := range m {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "http") &&
			(strings.Contains(strings.ToLower(k), "url") || strings.HasSuffix(strings.ToLower(s), ".mp4")) {
			return s
		}
	}
	for _, v := range m {
		switch t := v.(type) {
		case map[string]any:
			if u := deepFindHTTPURL(t); u != "" {
				return u
			}
		case []any:
			for _, it := range t {
				if mm, ok := it.(map[string]any); ok {
					if u := deepFindHTTPURL(mm); u != "" {
						return u
					}
				}
			}
		}
	}
	return ""
}
