package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/godeps/aigo/engine/aigoerr"
)

// DoJSON sends a JSON request and returns the response body.
// It sets Authorization (Bearer) and Content-Type headers automatically.
// Non-2xx responses are converted to *aigoerr.Error.
func DoJSON(ctx context.Context, hc *http.Client, method, url, apiKey string, body []byte, prefix string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", prefix, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: http %s: %w", prefix, method, err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read body: %w", prefix, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, prefix)
	}
	return out, nil
}
