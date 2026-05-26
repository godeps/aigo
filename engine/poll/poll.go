package poll

import (
	"context"
	"fmt"
	"time"
)

// Fetcher checks whether an async task has completed.
// It returns the result string when done is true.
type Fetcher func(ctx context.Context) (result string, done bool, err error)

// FetchResult carries extended information from a poll fetch.
type FetchResult struct {
	Result     string  // final output (set when Done=true)
	Done       bool    // whether the task has completed
	Percent    float64 // 0~1, upstream-reported progress (0 if unavailable)
	PreviewURL string  // intermediate preview URL (e.g. video first frame)
}

// FetcherV2 checks whether an async task has completed with extended info.
type FetcherV2 func(ctx context.Context) (FetchResult, error)

// OnProgress is called after each poll attempt (optional).
type OnProgress func(attempt int, elapsed time.Duration)

// ProgressInfo carries extended progress data for OnProgressV2 callbacks.
type ProgressInfo struct {
	Attempt    int
	Elapsed    time.Duration
	Percent    float64
	PreviewURL string
}

// OnProgressV2 is the extended progress callback.
type OnProgressV2 func(ProgressInfo)

// progressCtxKey is the context key for carrying a progress callback.
type progressCtxKey struct{}

// progressV2CtxKey is the context key for the extended progress callback.
type progressV2CtxKey struct{}

// WithOnProgress attaches a progress callback to the context.
// When Poll is called without Config.OnProgress, it falls back to this.
func WithOnProgress(ctx context.Context, fn OnProgress) context.Context {
	return context.WithValue(ctx, progressCtxKey{}, fn)
}

// WithOnProgressV2 attaches an extended progress callback to the context.
func WithOnProgressV2(ctx context.Context, fn OnProgressV2) context.Context {
	return context.WithValue(ctx, progressV2CtxKey{}, fn)
}

// onProgressFromContext extracts the progress callback from context, if set.
func onProgressFromContext(ctx context.Context) OnProgress {
	fn, _ := ctx.Value(progressCtxKey{}).(OnProgress)
	return fn
}

// onProgressV2FromContext extracts the extended progress callback from context.
func onProgressV2FromContext(ctx context.Context) OnProgressV2 {
	fn, _ := ctx.Value(progressV2CtxKey{}).(OnProgressV2)
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
	return PollV2(ctx, cfg, func(ctx context.Context) (FetchResult, error) {
		result, done, err := fetch(ctx)
		return FetchResult{Result: result, Done: done}, err
	})
}

// PollV2 is like Poll but accepts a FetcherV2 that returns extended progress info.
func PollV2(ctx context.Context, cfg Config, fetch FetcherV2) (string, error) {
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

	// Resolve progress callbacks: explicit config takes priority, then context fallback.
	onProgress := cfg.OnProgress
	if onProgress == nil {
		onProgress = onProgressFromContext(ctx)
	}
	onProgressV2 := onProgressV2FromContext(ctx)

	start := time.Now()
	cur := interval
	for attempt := 1; ; attempt++ {
		fr, err := fetch(ctx)
		if err != nil {
			return "", err
		}
		if fr.Done {
			return fr.Result, nil
		}
		if cfg.MaxAttempts > 0 && attempt >= cfg.MaxAttempts {
			return "", fmt.Errorf("poll: exceeded %d attempts", cfg.MaxAttempts)
		}

		elapsed := time.Since(start)
		if onProgressV2 != nil {
			onProgressV2(ProgressInfo{
				Attempt:    attempt,
				Elapsed:    elapsed,
				Percent:    fr.Percent,
				PreviewURL: fr.PreviewURL,
			})
		} else if onProgress != nil {
			onProgress(attempt, elapsed)
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
