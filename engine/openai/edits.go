package openai

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
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

// hasImageSource reports whether the graph contains any image source that should
// route the request to /v1/images/edits instead of /v1/images/generations.
func hasImageSource(g workflow.Graph) bool {
	if v, ok := resolve.StringOption(g, "image_b64", "image_base64"); ok && strings.TrimSpace(v) != "" {
		return true
	}
	if v, ok := resolve.StringOption(g, "image_path", "filename"); ok && strings.TrimSpace(v) != "" {
		return true
	}
	if v, ok := resolve.StringOption(g, "image_url", "edit_image_url"); ok && strings.TrimSpace(v) != "" {
		return true
	}
	for _, ref := range g.FindByClassType("LoadImage") {
		if v, ok := ref.Node.StringInput("image"); ok && strings.TrimSpace(v) != "" {
			return true
		}
		if v, ok := ref.Node.StringInput("url"); ok && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// loadImageBytes resolves the source image bytes from the graph. Order:
// image_b64 → image_path → image_url → LoadImage{image|url}.
func (e *Engine) loadImageBytes(ctx context.Context, g workflow.Graph) ([]byte, error) {
	if s, ok := resolve.StringOption(g, "image_b64", "image_base64"); ok && strings.TrimSpace(s) != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	}
	if p, ok := resolve.StringOption(g, "image_path", "filename"); ok && strings.TrimSpace(p) != "" {
		return os.ReadFile(p)
	}
	if u, ok := resolve.StringOption(g, "image_url", "edit_image_url"); ok && strings.TrimSpace(u) != "" {
		return e.fetchImageURL(ctx, u)
	}
	for _, ref := range g.FindByClassType("LoadImage") {
		if p, ok := ref.Node.StringInput("image"); ok && strings.TrimSpace(p) != "" {
			return os.ReadFile(p)
		}
		if u, ok := ref.Node.StringInput("url"); ok && strings.TrimSpace(u) != "" {
			return e.fetchImageURL(ctx, u)
		}
	}
	return nil, errors.New("openai: no image source found in graph")
}

func (e *Engine) fetchImageURL(ctx context.Context, u string) ([]byte, error) {
	if !e.allowRemoteMediaFetch {
		return nil, ErrRemoteMediaDisabled
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai: fetch image %s: status %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// executeEdits posts a multipart request to /v1/images/edits and decodes the response.
func (e *Engine) executeEdits(ctx context.Context, apiKey string, req Request, g workflow.Graph) (engine.Result, error) {
	imgBytes, err := e.loadImageBytes(ctx, g)
	if err != nil {
		return engine.Result{}, err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", req.Model)
	_ = w.WriteField("prompt", req.Prompt)
	if req.Size != "" {
		_ = w.WriteField("size", req.Size)
	}
	_ = w.WriteField("n", "1")
	if isGPTImageModel(req.Model) {
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
		if req.Quality != "" {
			_ = w.WriteField("quality", req.Quality)
		}
	} else {
		_ = w.WriteField("response_format", "url")
		if req.Quality != "" {
			_ = w.WriteField("quality", req.Quality)
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="image.png"`)
	h.Set("Content-Type", "image/png")
	part, err := w.CreatePart(h)
	if err != nil {
		return engine.Result{}, err
	}
	if _, err := part.Write(imgBytes); err != nil {
		return engine.Result{}, err
	}
	if err := w.Close(); err != nil {
		return engine.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/images/edits", &buf)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: build edits request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: edits request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openai: read edits response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "openai")
	}

	var decoded struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return engine.Result{}, fmt.Errorf("openai: decode edits response: %w", err)
	}
	if len(decoded.Data) == 0 {
		return engine.Result{}, errors.New("openai: edits response did not contain images")
	}
	if decoded.Data[0].URL != "" {
		return engine.Result{Value: decoded.Data[0].URL, Kind: engine.OutputURL}, nil
	}
	if decoded.Data[0].B64JSON != "" {
		return engine.Result{Value: "data:" + e.b64ImageMIME() + ";base64," + decoded.Data[0].B64JSON, Kind: engine.OutputDataURI}, nil
	}
	return engine.Result{}, errors.New("openai: edits response missing url and b64_json")
}
