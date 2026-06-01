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
// (first_frame, last_frame, reference_image) but require HTTP(S) or
// oss:// URLs for video/audio inputs (video, first_clip, reference_video).
//
// Resolution strategy:
//  1. HTTP(S) and oss:// URLs are returned as-is.
//  2. data: URIs with image MIME types are returned as-is (API accepts them).
//  3. data: URIs with non-image MIME types are decoded and uploaded to OSS.
//  4. Absolute file paths: images are converted to data URIs; videos use
//     sidecar .url files or are uploaded to OSS.
//  5. Anything else returns an error.
//
// model is the DashScope model name (e.g. "wan2.7-i2v") required by the
// upload policy API — uploaded files are bound to the specified model.
func EnsureRemoteURL(ctx context.Context, rt *runtime.RT, apiKey, model, rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") ||
		strings.HasPrefix(rawURL, "oss://") {
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
		return uploadToDashScopeOSS(ctx, rt, apiKey, model, data, "ref"+ext)
	}

	if filepath.IsAbs(rawURL) {
		if sidecar, err := os.ReadFile(rawURL + ".url"); err == nil {
			src := strings.TrimSpace(string(sidecar))
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") ||
				strings.HasPrefix(src, "oss://") {
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
		return uploadToDashScopeOSS(ctx, rt, apiKey, model, data, "ref"+ext)
	}

	return "", fmt.Errorf("aliyun: unsupported URL scheme for video API (need http/https, got %q)", truncate(rawURL, 80))
}

// EnsureRemoteURLs applies EnsureRemoteURL to each URL in the slice.
func EnsureRemoteURLs(ctx context.Context, rt *runtime.RT, apiKey, model string, urls []string) ([]string, error) {
	out := make([]string, len(urls))
	for i, u := range urls {
		resolved, err := EnsureRemoteURL(ctx, rt, apiKey, model, u)
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

// uploadPolicy holds the credentials returned by the DashScope upload policy API.
type uploadPolicy struct {
	Policy             string `json:"policy"`
	Signature          string `json:"signature"`
	UploadDir          string `json:"upload_dir"`
	UploadHost         string `json:"upload_host"`
	OSSAccessKeyID     string `json:"oss_access_key_id"`
	XOSSObjectACL      string `json:"x_oss_object_acl"`
	XOSSForbidOverwrite string `json:"x_oss_forbid_overwrite"`
}

// getUploadPolicy fetches temporary OSS upload credentials from DashScope.
// The returned policy is valid for ~300 seconds.
func getUploadPolicy(ctx context.Context, rt *runtime.RT, apiKey, model string) (*uploadPolicy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		rt.BaseURL+"/uploads?action=getPolicy&model="+model, nil)
	if err != nil {
		return nil, fmt.Errorf("aliyun: build upload policy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aliyun: get upload policy: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aliyun: read upload policy response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("aliyun: upload policy failed (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Data uploadPolicy `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("aliyun: decode upload policy: %w", err)
	}
	if result.Data.UploadHost == "" || result.Data.UploadDir == "" {
		return nil, fmt.Errorf("aliyun: upload policy missing upload_host or upload_dir: %s", truncate(string(body), 200))
	}
	return &result.Data, nil
}

// uploadToDashScopeOSS uploads binary data to DashScope's temporary OSS storage
// and returns an oss:// URL (valid for 48 hours).
//
// Flow: GET /uploads?action=getPolicy → POST {upload_host} → oss://{key}
//
// When using the returned oss:// URL in API calls, the caller must add
// the header X-DashScope-OssResourceResolve: enable (handled by async.Submit).
func uploadToDashScopeOSS(ctx context.Context, rt *runtime.RT, apiKey, model string, data []byte, filename string) (string, error) {
	policy, err := getUploadPolicy(ctx, rt, apiKey, model)
	if err != nil {
		return "", err
	}

	key := policy.UploadDir + "/" + filename

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("OSSAccessKeyId", policy.OSSAccessKeyID)
	writer.WriteField("Signature", policy.Signature)
	writer.WriteField("policy", policy.Policy)
	writer.WriteField("x-oss-object-acl", policy.XOSSObjectACL)
	writer.WriteField("x-oss-forbid-overwrite", policy.XOSSForbidOverwrite)
	writer.WriteField("key", key)
	writer.WriteField("success_action_status", "200")
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("aliyun: create OSS upload form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("aliyun: write OSS upload data: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.UploadHost, body)
	if err != nil {
		return "", fmt.Errorf("aliyun: build OSS upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := rt.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aliyun: OSS upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("aliyun: OSS upload failed (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	return "oss://" + key, nil
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
// data URIs pass through, video data URIs and local files get uploaded to OSS.
func ensureRemoteMediaURLs(ctx context.Context, rt *runtime.RT, apiKey, model string, media []map[string]any) ([]map[string]any, error) {
	out := make([]map[string]any, len(media))
	for i, m := range media {
		clone := make(map[string]any, len(m))
		for k, v := range m {
			clone[k] = v
		}
		if rawURL, ok := clone["url"].(string); ok && rawURL != "" {
			resolved, err := EnsureRemoteURL(ctx, rt, apiKey, model, rawURL)
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
