package httpx

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RetryTransport wraps an http.RoundTripper and retries on 429/5xx responses.
// Only GET requests (and other idempotent methods) are retried by default.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int           // default: 3
	BaseDelay  time.Duration // default: 1s
	MaxDelay   time.Duration // default: 30s
}

// RoundTrip implements http.RoundTripper with automatic retry logic.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	maxRetries := t.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseDelay := t.BaseDelay
	if baseDelay <= 0 {
		baseDelay = time.Second
	}
	maxDelay := t.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	// Only retry idempotent methods.
	if !isIdempotent(req.Method) {
		return base.RoundTrip(req)
	}

	var resp *http.Response
	var err error
	delay := baseDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait with jitter before retry.
			jitter := time.Duration(rand.Int63n(int64(delay) / 2))
			wait := delay + jitter

			// Check for Retry-After header from previous response.
			if resp != nil {
				if ra := parseRetryAfterHeader(resp.Header.Get("Retry-After")); ra > 0 {
					wait = ra
				}
			}

			timer := time.NewTimer(wait)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}

			// Grow delay with exponential backoff.
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}

		resp, err = base.RoundTrip(req)
		if err != nil {
			continue // network error, retry
		}
		if !shouldRetry(resp.StatusCode) {
			return resp, nil
		}
		// Drain and close the body so the connection can be reused.
		resp.Body.Close()
	}

	return resp, err
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func parseRetryAfterHeader(val string) time.Duration {
	if val == "" {
		return 0
	}
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// NewRetryClient creates an *http.Client with automatic retry for 429/5xx.
func NewRetryClient(maxRetries int, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &RetryTransport{
			MaxRetries: maxRetries,
		},
	}
}
