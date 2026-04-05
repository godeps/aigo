package newapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/workflow"
)

func (e *Engine) runSoraVideo(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", e.model)
	_ = w.WriteField("prompt", prompt)
	if img, ok := graph.FirstImageURL(g); ok {
		_ = w.WriteField("image", img)
	}
	if d, ok := graph.ExtractVideoDuration(g); ok {
		_ = w.WriteField("duration", fmt.Sprintf("%g", d))
	}
	if vw, vh, ok := graph.ExtractVideoDimensions(g); ok {
		_ = w.WriteField("width", fmt.Sprintf("%d", vw))
		_ = w.WriteField("height", fmt.Sprintf("%d", vh))
	}
	if fps, ok := graph.IntOptionPreferVideoOptions(g, "fps"); ok && fps > 0 {
		_ = w.WriteField("fps", fmt.Sprintf("%d", fps))
	}
	if seed, ok := graph.IntOptionPreferVideoOptions(g, "seed"); ok {
		_ = w.WriteField("seed", fmt.Sprintf("%d", seed))
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/videos"), apiKey, buf.Bytes(), w.FormDataContentType())
	if err != nil {
		return "", err
	}
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("newapi: decode sora create: %w", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("newapi: sora create missing id: %s", strings.TrimSpace(string(respBody)))
	}
	if !e.waitVideo {
		return created.ID, nil
	}
	return e.pollSora(ctx, apiKey, created.ID)
}

func (e *Engine) pollSora(ctx context.Context, apiKey, id string) (string, error) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.apiURL("/v1/videos/"+id), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("newapi: sora get: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", rerr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("newapi: sora get status %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		var st struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &st); err != nil {
			return "", fmt.Errorf("newapi: sora decode: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(st.Status)) {
		case "completed", "succeeded", "success":
			return e.fetchSoraContent(ctx, apiKey, id)
		case "failed", "error", "canceled", "cancelled":
			msg := st.Status
			if st.Error != nil && st.Error.Message != "" {
				msg = st.Error.Message
			}
			return "", fmt.Errorf("newapi: sora task failed: %s", msg)
		default:
			// queued, in_progress, ...
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("newapi: sora wait %q: %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (e *Engine) fetchSoraContent(ctx context.Context, apiKey, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.apiURL("/v1/videos/"+id+"/content"), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("newapi: sora content: %w", err)
	}
	defer resp.Body.Close()
	videoBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("newapi: sora content status %s: %s", resp.Status, strings.TrimSpace(string(videoBytes)))
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "video/mp4"
	}
	b64 := base64.StdEncoding.EncodeToString(videoBytes)
	return "data:" + ct + ";base64," + b64, nil
}
