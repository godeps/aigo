package httpx

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// DomainToken maps a host pattern to a bearer token.
type DomainToken struct {
	Domain string
	Token  string
}

// DomainTokenTransport injects Authorization: Bearer tokens for requests whose
// URL host matches a configured domain token. Explicit Authorization headers
// always win.
type DomainTokenTransport struct {
	Base   http.RoundTripper
	Tokens []DomainToken
}

func (t *DomainTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if token, ok := resolveDomainToken(req.URL, t.Tokens); ok && req.Header.Get("Authorization") == "" {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		req = clone
	}

	resp, err := roundTripperOrDefault(t.Base).RoundTrip(req)
	if resp != nil {
		captureResponse(req.Context(), req.URL.String(), resp.StatusCode, resp.Header)
	}
	return resp, err
}

// WithDomainTokens returns a shallow clone of client whose transport injects
// domain-scoped bearer tokens and captures response headers when a context
// collector is attached. Empty tokens still install the capturing transport.
func WithDomainTokens(client *http.Client, tokens []DomainToken) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	out := *client
	out.Transport = &DomainTokenTransport{
		Base:   client.Transport,
		Tokens: append([]DomainToken(nil), tokens...),
	}
	return &out
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

func roundTripperOrDefault(rt http.RoundTripper) http.RoundTripper {
	if rt != nil {
		return rt
	}
	return http.DefaultTransport
}

func resolveDomainToken(u *url.URL, tokens []DomainToken) (string, bool) {
	if u == nil || u.Host == "" {
		return "", false
	}
	host := strings.ToLower(u.Host)
	for _, dt := range tokens {
		if strings.TrimSpace(dt.Token) == "" {
			continue
		}
		if matchDomain(dt.Domain, host) {
			return dt.Token, true
		}
	}
	return "", false
}

func matchDomain(pattern, host string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	host = strings.ToLower(strings.TrimSpace(host))
	if pattern == "" || host == "" {
		return false
	}
	if pattern == host {
		return true
	}

	hostName, hostPort := splitHostPort(host)
	if strings.Contains(pattern, "/") {
		return matchCIDR(pattern, hostName)
	}
	if strings.Contains(pattern, "*") {
		return matchWildcard(pattern, hostName)
	}

	patternName, patternPort := splitHostPort(pattern)
	if patternPort != "" {
		return patternPort == hostPort && equalHostOrIP(patternName, hostName)
	}
	return equalHostOrIP(patternName, hostName)
}

func splitHostPort(hostport string) (host, port string) {
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		return h, p
	}
	return strings.Trim(hostport, "[]"), ""
}

func equalHostOrIP(a, b string) bool {
	if a == b {
		return true
	}
	aIP := net.ParseIP(a)
	bIP := net.ParseIP(b)
	if aIP != nil && bIP != nil {
		return aIP.Equal(bIP)
	}
	return false
}

func matchCIDR(cidr, host string) bool {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && network.Contains(ip)
}

func matchWildcard(pattern, host string) bool {
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	if strings.HasSuffix(pattern, ".*") {
		return strings.HasPrefix(host, pattern[:len(pattern)-1])
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == host
	}
	if !strings.HasPrefix(host, parts[0]) {
		return false
	}
	host = host[len(parts[0]):]
	last := parts[len(parts)-1]
	if !strings.HasSuffix(host, last) {
		return false
	}
	host = host[:len(host)-len(last)]
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(host, part)
		if idx < 0 {
			return false
		}
		host = host[idx+len(part):]
	}
	return true
}
