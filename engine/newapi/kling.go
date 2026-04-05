package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// runKlingVideo basePath 为 /kling/v1/videos/text2video 或 /kling/v1/videos/image2video
func (e *Engine) runKlingVideo(ctx context.Context, apiKey string, g workflow.Graph, basePath string) (string, error) {
	payload, err := e.buildStandardVideoPayload(g)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal kling create: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL(basePath), apiKey, body, "application/json")
	if err != nil {
		return "", err
	}
	var created struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("newapi: decode kling create: %w", err)
	}
	if strings.TrimSpace(created.TaskID) == "" {
		return "", fmt.Errorf("newapi: kling create missing task_id: %s", strings.TrimSpace(string(respBody)))
	}
	if !e.waitVideo {
		return created.TaskID, nil
	}
	return e.pollVideoGET(ctx, apiKey, func(id string) string {
		return e.apiURL(basePath + "/" + id)
	}, created.TaskID)
}
