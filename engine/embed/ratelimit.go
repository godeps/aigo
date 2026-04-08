package embed

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a sliding-window rate limiter.
// Shared by all embedding backends that need request throttling.
type RateLimiter struct {
	mu         sync.Mutex
	max        int
	timestamps []time.Time
}

// NewRateLimiter creates a rate limiter allowing max requests per minute.
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return &RateLimiter{max: maxPerMinute}
}

// Wait blocks until the caller is allowed to proceed.
// Respects context cancellation.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Prune timestamps older than 1 minute.
	cutoff := now.Add(-time.Minute)
	i := 0
	for i < len(rl.timestamps) && rl.timestamps[i].Before(cutoff) {
		i++
	}
	rl.timestamps = rl.timestamps[i:]

	if len(rl.timestamps) >= rl.max {
		sleepFor := time.Minute - now.Sub(rl.timestamps[0])
		if sleepFor > 0 {
			rl.mu.Unlock()
			select {
			case <-time.After(sleepFor):
			case <-ctx.Done():
				rl.mu.Lock()
				return ctx.Err()
			}
			rl.mu.Lock()
		}
	}

	rl.timestamps = append(rl.timestamps, time.Now())
	return nil
}
