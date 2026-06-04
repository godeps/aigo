package httpx

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// ResponseCaptureHook records response headers when the request context has a
// ResponseCapture attached.
type ResponseCaptureHook struct{}

// AfterResponse implements ResponseHook.
func (ResponseCaptureHook) AfterResponse(resp *http.Response) error {
	if resp == nil || resp.Request == nil {
		return nil
	}
	captureResponse(resp.Request.Context(), resp.Request.URL.String(), resp.StatusCode, resp.Header)
	return nil
}

// ResponseRecord captures response headers observed by a wrapped HTTP client.
type ResponseRecord struct {
	URL        string              `json:"url"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
}

// ResponseCapture stores response header records for requests sharing a context.
type ResponseCapture struct {
	mu      sync.Mutex
	records []ResponseRecord
}

// NewResponseCapture creates an empty response header collector.
func NewResponseCapture() *ResponseCapture {
	return &ResponseCapture{}
}

// Records returns a snapshot of captured response records.
func (c *ResponseCapture) Records() []ResponseRecord {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ResponseRecord(nil), c.records...)
}

type responseCaptureKey struct{}

// WithResponseCapture attaches a response header collector to ctx.
func WithResponseCapture(ctx context.Context, c *ResponseCapture) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, responseCaptureKey{}, c)
}

// ResponseCaptureFromContext returns the collector attached to ctx, if any.
func ResponseCaptureFromContext(ctx context.Context) *ResponseCapture {
	c, _ := ctx.Value(responseCaptureKey{}).(*ResponseCapture)
	return c
}

func captureResponse(ctx context.Context, rawURL string, statusCode int, headers http.Header) {
	c := ResponseCaptureFromContext(ctx)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, ResponseRecord{
		URL:        rawURL,
		StatusCode: statusCode,
		Headers:    cloneSafeHeaders(headers),
	})
}

func cloneSafeHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		if strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		out[k] = append([]string(nil), vals...)
	}
	return out
}
