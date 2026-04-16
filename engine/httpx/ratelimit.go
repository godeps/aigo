package httpx

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitTransport wraps an http.RoundTripper with a token-bucket rate limiter.
type RateLimitTransport struct {
	Base    http.RoundTripper
	Limiter *rate.Limiter
}

// RoundTrip implements http.RoundTripper. It waits for rate limit clearance
// before forwarding the request to the base transport.
func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Limiter != nil {
		if err := t.Limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewRateLimitedClient creates an *http.Client with a token-bucket rate limiter.
// rps is requests per second; burst is the maximum burst size.
func NewRateLimitedClient(rps float64, burst int, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if burst <= 0 {
		burst = 1
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &RateLimitTransport{
			Limiter: rate.NewLimiter(rate.Limit(rps), burst),
		},
	}
}
