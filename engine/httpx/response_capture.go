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

var (
	defaultResponseHeaderNames = []string{
		"x-request-id",
		"x-correlation-id",
		"x-trace-id",
		"x-dashscope-request-id",
		"retry-after",
	}
	defaultResponseHeaderPrefixes = []string{
		"x-ratelimit-",
		"ratelimit-",
	}
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
	mu             sync.Mutex
	records        []ResponseRecord
	maxRecords     int
	maxValueLen    int
	headerNames    map[string]struct{}
	headerPrefixes []string
}

// ResponseCaptureOption configures response header capture behavior.
type ResponseCaptureOption func(*ResponseCapture)

// NewResponseCapture creates an empty response header collector.
func NewResponseCapture(opts ...ResponseCaptureOption) *ResponseCapture {
	c := &ResponseCapture{
		maxRecords:     defaultMaxResponseRecords,
		maxValueLen:    defaultMaxResponseHeaderValue,
		headerNames:    makeHeaderNameSet(defaultResponseHeaderNames),
		headerPrefixes: append([]string(nil), defaultResponseHeaderPrefixes...),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.maxRecords <= 0 {
		c.maxRecords = defaultMaxResponseRecords
	}
	if c.maxValueLen <= 0 {
		c.maxValueLen = defaultMaxResponseHeaderValue
	}
	return c
}

// WithResponseCaptureLimits overrides record count and header value length
// limits. Non-positive values keep the default limit.
func WithResponseCaptureLimits(maxRecords, maxValueLen int) ResponseCaptureOption {
	return func(c *ResponseCapture) {
		if maxRecords > 0 {
			c.maxRecords = maxRecords
		}
		if maxValueLen > 0 {
			c.maxValueLen = maxValueLen
		}
	}
}

// WithResponseHeaderNames replaces the exact response header allowlist. Header
// names are matched case-insensitively.
func WithResponseHeaderNames(names ...string) ResponseCaptureOption {
	return func(c *ResponseCapture) {
		c.headerNames = makeHeaderNameSet(names)
	}
}

// WithResponseHeaderPrefixes replaces the response header prefix allowlist.
// Prefixes are matched case-insensitively.
func WithResponseHeaderPrefixes(prefixes ...string) ResponseCaptureOption {
	return func(c *ResponseCapture) {
		c.headerPrefixes = normalizeHeaderPrefixes(prefixes)
	}
}

// Records returns a snapshot of captured response records.
func (c *ResponseCapture) Records() []ResponseRecord {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ResponseRecord, 0, len(c.records))
	for _, record := range c.records {
		out = append(out, cloneResponseRecord(record))
	}
	return out
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
	if len(c.records) >= c.maxRecords {
		return
	}
	c.records = append(c.records, ResponseRecord{
		Method:     method,
		URL:        rawURL,
		StatusCode: statusCode,
		Headers:    c.cloneSafeHeaders(headers),
	})
}

func (c *ResponseCapture) cloneSafeHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		if !c.isSafeResponseHeader(k) {
			continue
		}
		out[k] = make([]string, 0, len(vals))
		for _, val := range vals {
			out[k] = append(out[k], truncateHeaderValue(val, c.maxValueLen))
		}
	}
	return out
}

func cloneResponseRecord(record ResponseRecord) ResponseRecord {
	return ResponseRecord{
		Method:     record.Method,
		URL:        record.URL,
		StatusCode: record.StatusCode,
		Headers:    cloneHeaderMap(record.Headers),
	}
}

func cloneHeaderMap(headers map[string][]string) map[string][]string {
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		out[k] = append([]string(nil), vals...)
	}
	return out
}

func truncateHeaderValue(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func (c *ResponseCapture) isSafeResponseHeader(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if _, ok := c.headerNames[name]; ok {
		return true
	}
	for _, prefix := range c.headerPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func makeHeaderNameSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func normalizeHeaderPrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.ToLower(strings.TrimSpace(prefix))
		if prefix != "" {
			out = append(out, prefix)
		}
	}
	return out
}
