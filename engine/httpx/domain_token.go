package httpx

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// DomainToken maps a host pattern to a bearer token.
type DomainToken struct {
	Domain string
	Token  string
}

// DomainTokenHook injects Authorization: Bearer tokens for requests whose URL
// host matches a configured domain token. Explicit Authorization headers always
// win.
type DomainTokenHook struct {
	Tokens []DomainToken
}

// BeforeRequest implements RequestHook.
func (h DomainTokenHook) BeforeRequest(req *http.Request) (*http.Request, error) {
	if token, ok := resolveDomainToken(req.URL, h.Tokens); ok && req.Header.Get("Authorization") == "" {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		return clone, nil
	}
	return req, nil
}

// WithDomainTokens returns a shallow clone of client whose transport injects
// domain-scoped bearer tokens and captures response headers when a context
// collector is attached. Empty tokens still install the capturing transport.
func WithDomainTokens(client *http.Client, tokens []DomainToken) *http.Client {
	return WithHTTPHooks(client, DomainTokenHook{Tokens: append([]DomainToken(nil), tokens...)}, ResponseCaptureHook{})
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
