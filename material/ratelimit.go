package material

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RateLimiter provides simple token-bucket rate limiting for search backends.
type RateLimiter struct {
	ticker   *time.Ticker
	tokens   chan struct{}
}

// NewRateLimiter creates a rate limiter that allows rpm requests per minute.
func NewRateLimiter(rpm int) *RateLimiter {
	if rpm <= 0 {
		rpm = 60
	}
	interval := time.Minute / time.Duration(rpm)
	rl := &RateLimiter{
		ticker: time.NewTicker(interval),
		tokens: make(chan struct{}, rpm),
	}
	// Pre-fill one token so the first request goes through immediately.
	rl.tokens <- struct{}{}
	go func() {
		for range rl.ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()
	return rl
}

// Wait blocks until a token is available or context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop releases the rate limiter resources.
func (rl *RateLimiter) Stop() {
	rl.ticker.Stop()
}

// Retry executes fn with exponential backoff and jitter.
// Retries up to maxAttempts times on error. Returns the last error if all attempts fail.
func Retry(fn func() error, maxAttempts int, baseDelay time.Duration) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < maxAttempts-1 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(i)))
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			time.Sleep(delay + jitter)
		}
	}
	return err
}

// RetryWithContext executes fn with exponential backoff, respecting context cancellation.
func RetryWithContext(ctx context.Context, fn func() error, maxAttempts int, baseDelay time.Duration) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < maxAttempts-1 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(i)))
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			select {
			case <-time.After(delay + jitter):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return err
}
