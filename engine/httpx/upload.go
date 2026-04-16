package httpx

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/godeps/aigo/engine/aigoerr"
)

// UploadFile uploads a file via multipart/form-data.
// fieldName is the form field name for the file; extra adds additional string fields.
func UploadFile(ctx context.Context, hc *http.Client, url, apiKey, fieldName, filePath string, extra map[string]string, prefix string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %w", prefix, err)
	}
	defer f.Close()

	return UploadReader(ctx, hc, url, apiKey, fieldName, filepath.Base(filePath), f, extra, prefix)
}

// UploadReader uploads data from an io.Reader via multipart/form-data.
func UploadReader(ctx context.Context, hc *http.Client, url, apiKey, fieldName, fileName string, r io.Reader, extra map[string]string, prefix string) ([]byte, error) {
	pr, pw := io.Pipe()

	writer := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()

		// Add extra fields first.
		for k, v := range extra {
			if err := writer.WriteField(k, v); err != nil {
				pw.CloseWithError(err)
				return
			}
		}

		// Add the file field.
		part, err := writer.CreateFormFile(fieldName, fileName)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, r); err != nil {
			pw.CloseWithError(err)
			return
		}
		writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return nil, fmt.Errorf("%s: build upload request: %w", prefix, err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: upload request: %w", prefix, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read upload response: %w", prefix, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, body, prefix)
	}
	return body, nil
}
