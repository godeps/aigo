package poll

import (
	"context"
	"fmt"
	"time"
)

// Fetcher checks whether an async task has completed.
// It returns the result string when done is true.
type Fetcher func(ctx context.Context) (result string, done bool, err error)

// OnProgress is called after each poll attempt (optional).
type OnProgress func(attempt int, elapsed time.Duration)

// progressCtxKey is the context key for carrying a progress callback.
type progressCtxKey struct{}

// WithOnProgress attaches a progress callback to the context.
// When Poll is called without Config.OnProgress, it falls back to this.
func WithOnProgress(ctx context.Context, fn OnProgress) context.Context {
	return context.WithValue(ctx, progressCtxKey{}, fn)
}

// onProgressFromContext extracts the progress callback from context, if set.
func onProgressFromContext(ctx context.Context) OnProgress {
	fn, _ := ctx.Value(progressCtxKey{}).(OnProgress)
	return fn
}

// Config controls polling behavior.
type Config struct {
	Interval    time.Duration // base polling interval
	MaxAttempts int           // 0 means unlimited
	Backoff     float64       // multiplier per attempt; 1.0 = fixed interval
	MaxInterval time.Duration // upper bound for backoff growth
	OnProgress  OnProgress    // optional progress callback
}

// Poll calls fetch repeatedly until it returns done or an error.
// It calls fetch immediately on the first iteration (no initial wait).
func Poll(ctx context.Context, cfg Config, fetch Fetcher) (string, error) {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	backoff := cfg.Backoff
	if backoff < 1.0 {
		backoff = 1.0
	}
	maxInterval := cfg.MaxInterval
	if maxInterval <= 0 {
		maxInterval = 60 * time.Second
	}

	// Resolve progress callback: explicit config takes priority, then context fallback.
	onProgress := cfg.OnProgress
	if onProgress == nil {
		onProgress = onProgressFromContext(ctx)
	}

	start := time.Now()
	cur := interval
	for attempt := 1; ; attempt++ {
		result, done, err := fetch(ctx)
		if err != nil {
			return "", err
		}
		if done {
			return result, nil
		}
		if cfg.MaxAttempts > 0 && attempt >= cfg.MaxAttempts {
			return "", fmt.Errorf("poll: exceeded %d attempts", cfg.MaxAttempts)
		}
		if onProgress != nil {
			onProgress(attempt, time.Since(start))
		}

		timer := time.NewTimer(cur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}

		// grow interval
		if backoff > 1.0 {
			cur = time.Duration(float64(cur) * backoff)
			if cur > maxInterval {
				cur = maxInterval
			}
		}
	}
}
