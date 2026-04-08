package embed

import (
	"math"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
)

// Retry calls fn with exponential back-off on retryable errors.
func Retry(fn func() error, maxRetries int, initialDelay time.Duration) error {
	delay := initialDelay
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !aigoerr.IsRetryable(lastErr) || attempt == maxRetries {
			return lastErr
		}
		wait := time.Duration(math.Min(float64(delay), float64(60*time.Second)))
		time.Sleep(wait)
		delay *= 2
	}
	return lastErr
}
