package vidgen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
)

// EnsureRemoteURL guarantees that url is usable by DashScope servers.
//
// DashScope video APIs accept base64 data URIs for image inputs
// (first_frame, last_frame, reference_image) but require HTTP(S) URLs
// for video inputs (video, first_clip, reference_video).
//
// Resolution strategy:
//  1. HTTP(S) URLs are returned as-is.
//  2. data: URIs with image MIME types are returned as-is (API accepts them).
//  3. data: URIs with non-image MIME types are decoded and uploaded.
//  4. Absolute file paths: images are converted to data URIs; videos use
//     sidecar .url files or are uploaded.
//  5. Anything else returns an error.
func EnsureRemoteURL(ctx context.Context, rt *runtime.RT, apiKey, rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL, nil
	}

	if strings.HasPrefix(rawURL, "data:") {
		if isImageDataURI(rawURL) {
			return rawURL, nil
		}
		data, ext, err := decodeDataURI(rawURL)
		if err != nil {
			return "", fmt.Errorf("aliyun: decode data URI for upload: %w", err)
		}
		return uploadToDashScope(ctx, rt, apiKey, data, "ref"+ext)
	}

	if filepath.IsAbs(rawURL) {
		if sidecar, err := os.ReadFile(rawURL + ".url"); err == nil {
			src := strings.TrimSpace(string(sidecar))
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				return src, nil
			}
		}
		if isImageExt(filepath.Ext(rawURL)) {
			return fileToDataURI(rawURL)
		}
		data, err := os.ReadFile(rawURL)
		if err != nil {
			return "", fmt.Errorf("aliyun: read local file for upload: %w (path=%s)", err, rawURL)
		}
		ext := filepath.Ext(rawURL)
		if ext == "" {
			ext = ".mp4"
		}
		return uploadToDashScope(ctx, rt, apiKey, data, "ref"+ext)
	}

	return "", fmt.Errorf("aliyun: unsupported URL scheme for video API (need http/https, got %q)", truncate(rawURL, 80))
}

// EnsureRemoteURLs applies EnsureRemoteURL to each URL in the slice, returning
// on the first error.
func EnsureRemoteURLs(ctx context.Context, rt *runtime.RT, apiKey string, urls []string) ([]string, error) {
	out := make([]string, len(urls))
	for i, u := range urls {
		resolved, err := EnsureRemoteURL(ctx, rt, apiKey, u)
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

// uploadToDashScope uploads binary data to DashScope's file storage and returns
// the accessible HTTP URL. It uses the /compatible-mode/v1/files endpoint.
func uploadToDashScope(ctx context.Context, rt *runtime.RT, apiKey string, data []byte, filename string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("aliyun: create upload form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("aliyun: write upload data: %w", err)
	}
	if err := writer.WriteField("purpose", "file-extract"); err != nil {
		return "", fmt.Errorf("aliyun: write upload purpose: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		rt.BaseURL+"/compatible-mode/v1/files", body)
	if err != nil {
		return "", fmt.Errorf("aliyun: build upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: upload file: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aliyun: read upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("aliyun: upload failed (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var result struct {
		ID       string `json:"id"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("aliyun: decode upload response: %w", err)
	}

	if result.URL != "" {
		return result.URL, nil
	}

	return "", fmt.Errorf("aliyun: upload response did not contain a downloadable URL (id=%s); "+
		"video inputs require an HTTP(S) URL — provide a publicly accessible URL instead of a local file: %s",
		result.ID, truncate(string(respBody), 200))
}

// isVideoMediaType reports whether a media type string refers to a video input
// that requires an HTTP(S) URL (data URIs are not accepted for video inputs).
func isVideoMediaType(mediaType string) bool {
	switch mediaType {
	case "video", "first_clip", "reference_video":
		return true
	}
	return false
}

// ensureRemoteMediaURLs resolves "url" fields in a VideoEditMedia slice.
// EnsureRemoteURL handles the image-vs-video distinction internally: image
// data URIs pass through, video data URIs and local files get uploaded.
func ensureRemoteMediaURLs(ctx context.Context, rt *runtime.RT, apiKey string, media []map[string]any) ([]map[string]any, error) {
	out := make([]map[string]any, len(media))
	for i, m := range media {
		clone := make(map[string]any, len(m))
		for k, v := range m {
			clone[k] = v
		}
		if rawURL, ok := clone["url"].(string); ok && rawURL != "" {
			resolved, err := EnsureRemoteURL(ctx, rt, apiKey, rawURL)
			if err != nil {
				return nil, err
			}
			clone["url"] = resolved
		}
		out[i] = clone
	}
	return out, nil
}

// isImageDataURI reports whether the data URI has an image/* MIME type.
func isImageDataURI(uri string) bool {
	return strings.HasPrefix(uri, "data:image/")
}

// isImageExt reports whether the file extension belongs to a supported image format.
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".bmp": true,
}

func isImageExt(ext string) bool {
	return imageExts[strings.ToLower(ext)]
}

// fileToDataURI reads a local file and returns a base64-encoded data URI.
func fileToDataURI(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("aliyun: read local image for data URI: %w (path=%s)", err, path)
	}
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

func decodeDataURI(dataURI string) ([]byte, string, error) {
	comma := strings.IndexByte(dataURI, ',')
	if comma < 0 {
		return nil, "", fmt.Errorf("invalid data URI: no comma separator")
	}
	header := dataURI[:comma]
	payload := dataURI[comma+1:]

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", fmt.Errorf("base64 decode: %w", err)
		}
	}

	ext := ".bin"
	if colon := strings.IndexByte(header, ':'); colon >= 0 {
		mimeType := header[colon+1:]
		if semi := strings.IndexByte(mimeType, ';'); semi >= 0 {
			mimeType = mimeType[:semi]
		}
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
		}
	}
	return data, ext, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
