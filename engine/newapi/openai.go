package newapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/engine/newapi/internal/poll"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

func (e *Engine) runOpenAIImageGenerations(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}

	gptImage := isGPTImageModel(e.model)
	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
		"size":   graph.ExtractImageSizeOpenAI(g, imageSizesForModel(e.model)...),
		"n":      1,
	}
	if gptImage {
		// gpt-image-* always returns b64_json and rejects response_format/style.
		e.applyGPTImageOptions(payload)
	} else {
		payload["response_format"] = "url"
		if e.style != "" {
			payload["style"] = e.style
		}
	}
	if quality := e.imageQuality(g); quality != "" {
		payload["quality"] = quality
	}
	if n, ok := graph.IntOption(g, "n"); ok && n >= 1 && n <= 10 {
		payload["n"] = n
	}
	_ = graph.MergeJSONObject(g, payload, "extra_body", "openai_image_extra")

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal image request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/images/generations"), apiKey, body, "application/json")
	if err != nil {
		return "", err
	}
	return decodeOpenAIImageData(respBody, e.b64ImageMIME())
}

func (e *Engine) runOpenAIImageEdits(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	imgBytes, err := graph.ImageBytesForEdits(g, e.httpClient, e.allowRemoteMediaFetch)
	if err != nil {
		return "", wrapGraphErr(err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", e.model)
	_ = w.WriteField("prompt", prompt)
	if isGPTImageModel(e.model) {
		// gpt-image-* always returns b64_json; response_format is not accepted.
		if e.background != "" {
			_ = w.WriteField("background", e.background)
		}
		if e.outputFormat != "" {
			_ = w.WriteField("output_format", e.outputFormat)
		}
		if e.moderation != "" {
			_ = w.WriteField("moderation", e.moderation)
		}
		if e.outputCompression > 0 {
			_ = w.WriteField("output_compression", fmt.Sprintf("%d", e.outputCompression))
		}
	} else {
		_ = w.WriteField("response_format", "url")
	}
	if s := graph.ExtractImageSizeOpenAI(g, imageSizesForModel(e.model)...); s != "" {
		_ = w.WriteField("size", s)
	}
	if n, ok := graph.IntOption(g, "n"); ok && n >= 1 && n <= 10 {
		_ = w.WriteField("n", fmt.Sprintf("%d", n))
	}
	if quality := e.imageQuality(g); quality != "" {
		_ = w.WriteField("quality", quality)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="image.png"`)
	h.Set("Content-Type", "image/png")
	part, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(imgBytes); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/images/edits"), apiKey, buf.Bytes(), w.FormDataContentType())
	if err != nil {
		return "", err
	}
	return decodeOpenAIImageData(respBody, e.b64ImageMIME())
}

func (e *Engine) imageQuality(g workflow.Graph) string {
	if quality, ok := graph.StringOptionFromClassTypes(g, []string{"ImageOptions"}, "quality"); ok {
		return strings.TrimSpace(quality)
	}
	return strings.TrimSpace(e.quality)
}

func (e *Engine) runOpenAIVideoGenerations(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	payload, err := e.buildStandardVideoPayload(g)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal video create: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/video/generations"), apiKey, body, "application/json")
	if err != nil {
		return "", err
	}

	var created struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("newapi: decode video create: %w", err)
	}
	if strings.TrimSpace(created.TaskID) == "" {
		return "", fmt.Errorf("newapi: video create missing task_id: %s", strings.TrimSpace(string(respBody)))
	}
	if !e.waitVideo {
		return created.TaskID, nil
	}
	return e.pollVideoGET(ctx, apiKey, func(id string) string {
		return e.apiURL("/v1/video/generations/" + id)
	}, created.TaskID)
}

func (e *Engine) buildStandardVideoPayload(g workflow.Graph) (map[string]any, error) {
	prompt, err := graph.ExtractPrompt(g)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
	}
	if img, ok := graph.FirstImageURL(g); ok {
		payload["image"] = img
	}
	if d, ok := graph.ExtractVideoDuration(g); ok {
		payload["duration"] = resolve.ClampDuration(d, 0, 60)
	}
	if w, h, ok := graph.ExtractVideoDimensions(g); ok {
		payload["width"] = w
		payload["height"] = h
	}
	if fps, ok := graph.IntOptionPreferVideoOptions(g, "fps"); ok && fps > 0 {
		payload["fps"] = fps
	}
	if seed, ok := graph.IntOptionPreferVideoOptions(g, "seed"); ok {
		payload["seed"] = seed
	}
	if n, ok := graph.IntOptionPreferVideoOptions(g, "n"); ok && n > 0 {
		payload["n"] = n
	}
	payload["response_format"] = "url"
	meta := map[string]any{}
	if neg, ok := graph.ExtractNegativePrompt(g); ok {
		meta["negative_prompt"] = neg
	}
	if len(meta) > 0 {
		payload["metadata"] = meta
	}
	_ = graph.MergeJSONObject(g, payload, "extra_body", "video_extra")
	return payload, nil
}

func (e *Engine) pollVideoGET(ctx context.Context, apiKey string, urlForID func(string) string, taskID string) (string, error) {
	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlForID(taskID), nil)
		if err != nil {
			return "", false, fmt.Errorf("newapi: build video get: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := e.httpClient.Do(req)
		if err != nil {
			return "", false, fmt.Errorf("newapi: video get: %w", err)
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("newapi: read video get: %w", rerr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", false, aigoerr.FromHTTPResponse(resp, body, "newapi")
		}
		url, done, perr := poll.ParseVideoJSON(body)
		if perr != nil {
			return "", false, perr
		}
		if done {
			return url, true, nil
		}
		return "", false, nil
	})
}

func (e *Engine) runOpenAISpeech(ctx context.Context, apiKey string, g workflow.Graph) (string, error) {
	text, err := graph.ExtractPrompt(g)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	voice, ok := graph.ExtractSpeechVoice(g)
	if !ok || strings.TrimSpace(voice) == "" {
		return "", ErrMissingVoice
	}
	format := graph.ExtractSpeechResponseFormat(g)
	payload := map[string]any{
		"model":           e.model,
		"input":           text,
		"voice":           strings.TrimSpace(voice),
		"response_format": format,
	}
	if speed, ok := graph.ExtractSpeechSpeed(g); ok {
		payload["speed"] = speed
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("newapi: marshal speech: %w", err)
	}

	audioBytes, err := e.doRequest(ctx, http.MethodPost, e.apiURL("/v1/audio/speech"), apiKey, body, "application/json")
	if err != nil {
		return "", err
	}
	mime := speechMIME(format)
	b64 := base64.StdEncoding.EncodeToString(audioBytes)
	return "data:" + mime + ";base64," + b64, nil
}

func (e *Engine) runOpenAIWhisper(ctx context.Context, apiKey string, g workflow.Graph, path string) (string, error) {
	fn, audio, err := graph.AudioBytesForWhisper(g, e.httpClient, e.allowRemoteMediaFetch)
	if err != nil {
		return "", wrapGraphErr(err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", e.model)
	if lang, ok := graph.StringOption(g, "language"); ok {
		_ = w.WriteField("language", lang)
	}
	if p, ok := graph.StringOption(g, "whisper_prompt"); ok {
		_ = w.WriteField("prompt", p)
	}
	rf := "json"
	if v, ok := graph.StringOption(g, "response_format"); ok {
		rf = v
	}
	_ = w.WriteField("response_format", rf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, fn))
	h.Set("Content-Type", "application/octet-stream")
	part, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.apiURL(path), apiKey, buf.Bytes(), w.FormDataContentType())
	if err != nil {
		return "", err
	}
	if rf == "text" || rf == "srt" || rf == "vtt" {
		return strings.TrimSpace(string(respBody)), nil
	}
	return string(respBody), nil
}

func (e *Engine) doRequest(ctx context.Context, method, url, apiKey string, body []byte, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("newapi: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("newapi: http %s: %w", method, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("newapi: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "newapi")
	}
	return out, nil
}

func decodeOpenAIImageData(respBody []byte, b64MIME string) (string, error) {
	var decoded struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("newapi: decode image response: %w", err)
	}
	if len(decoded.Data) == 0 {
		return "", errors.New("newapi: image response had no data")
	}
	if decoded.Data[0].URL != "" {
		return decoded.Data[0].URL, nil
	}
	if decoded.Data[0].B64JSON != "" {
		mime := b64MIME
		if mime == "" {
			mime = "image/png"
		}
		return "data:" + mime + ";base64," + decoded.Data[0].B64JSON, nil
	}
	return "", errors.New("newapi: image response had no url or b64_json")
}

// applyGPTImageOptions writes gpt-image-* specific fields into the JSON payload,
// omitting any field that is empty/zero.
func (e *Engine) applyGPTImageOptions(payload map[string]any) {
	if e.background != "" {
		payload["background"] = e.background
	}
	if e.outputFormat != "" {
		payload["output_format"] = e.outputFormat
	}
	if e.moderation != "" {
		payload["moderation"] = e.moderation
	}
	if e.outputCompression > 0 {
		payload["output_compression"] = e.outputCompression
	}
}

// b64ImageMIME returns the data: URI MIME for b64_json image responses,
// derived from the configured OutputFormat. Defaults to image/png.
func (e *Engine) b64ImageMIME() string {
	return imageMIMEFromOutputFormat(e.outputFormat)
}

// imageMIMEFromOutputFormat maps OpenAI image `output_format` values to MIME types.
func imageMIMEFromOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "png", "":
		return "image/png"
	}
	return "image/png"
}

// isGPTImageModel reports whether the model belongs to the gpt-image-* family,
// which has a different request/response contract than DALL-E models.
func isGPTImageModel(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "gpt-image-")
}

func speechMIME(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/mpeg"
	}
}
