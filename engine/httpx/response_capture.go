package httpx

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

const (
	defaultMaxResponseRecords     = 32
	defaultMaxResponseHeaderValue = 1024
)

// ResponseCaptureHook records response headers when the request context has a
// ResponseCapture attached.
type ResponseCaptureHook struct{}

// AfterResponse implements ResponseHook.
func (ResponseCaptureHook) AfterResponse(resp *http.Response) error {
	if resp == nil || resp.Request == nil {
		return nil
	}
	captureResponse(resp.Request.Context(), resp.Request.Method, resp.Request.URL.String(), resp.StatusCode, resp.Header)
	return nil
}

// ResponseRecord captures response headers observed by a wrapped HTTP client.
type ResponseRecord struct {
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
}

// ResponseCapture stores response header records for requests sharing a context.
type ResponseCapture struct {
	mu          sync.Mutex
	records     []ResponseRecord
	maxRecords  int
	maxValueLen int
}

// NewResponseCapture creates an empty response header collector.
func NewResponseCapture() *ResponseCapture {
	return &ResponseCapture{
		maxRecords:  defaultMaxResponseRecords,
		maxValueLen: defaultMaxResponseHeaderValue,
	}
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

func captureResponse(ctx context.Context, method, rawURL string, statusCode int, headers http.Header) {
	c := ResponseCaptureFromContext(ctx)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	maxRecords := c.maxRecords
	if maxRecords <= 0 {
		maxRecords = defaultMaxResponseRecords
	}
	if len(c.records) >= maxRecords {
		return
	}
	maxValueLen := c.maxValueLen
	if maxValueLen <= 0 {
		maxValueLen = defaultMaxResponseHeaderValue
	}
	c.records = append(c.records, ResponseRecord{
		Method:     method,
		URL:        rawURL,
		StatusCode: statusCode,
		Headers:    cloneSafeHeaders(headers, maxValueLen),
	})
}

func cloneSafeHeaders(headers http.Header, maxValueLen int) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		if !isSafeResponseHeader(k) {
			continue
		}
		out[k] = make([]string, 0, len(vals))
		for _, val := range vals {
			out[k] = append(out[k], truncateHeaderValue(val, maxValueLen))
		}
	}
	return out
}

func truncateHeaderValue(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func isSafeResponseHeader(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "x-request-id",
		"x-correlation-id",
		"x-trace-id",
		"x-dashscope-request-id",
		"retry-after":
		return true
	}
	return strings.HasPrefix(name, "x-ratelimit-") ||
		strings.HasPrefix(name, "ratelimit-")
}
