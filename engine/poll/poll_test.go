package poll

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoll_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	result, err := Poll(context.Background(), Config{Interval: time.Millisecond}, func(ctx context.Context) (string, bool, error) {
		return "ok", true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("got %q, want %q", result, "ok")
	}
}

func TestPoll_SuccessAfterRetries(t *testing.T) {
	t.Parallel()

	var calls int32
	result, err := Poll(context.Background(), Config{Interval: time.Millisecond}, func(ctx context.Context) (string, bool, error) {
		n := atomic.AddInt32(&calls, 1)
		if n >= 3 {
			return "done", true, nil
		}
		return "", false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Errorf("got %q, want %q", result, "done")
	}
	if c := atomic.LoadInt32(&calls); c != 3 {
		t.Errorf("expected 3 calls, got %d", c)
	}
}

func TestPoll_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	_, err := Poll(ctx, Config{Interval: time.Millisecond}, func(ctx context.Context) (string, bool, error) {
		if atomic.AddInt32(&calls, 1) >= 2 {
			cancel()
		}
		return "", false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestPoll_MaxAttempts(t *testing.T) {
	t.Parallel()

	var calls int32
	_, err := Poll(context.Background(), Config{Interval: time.Millisecond, MaxAttempts: 3}, func(ctx context.Context) (string, bool, error) {
		atomic.AddInt32(&calls, 1)
		return "", false, nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if c := atomic.LoadInt32(&calls); c != 3 {
		t.Errorf("expected 3 calls, got %d", c)
	}
}

func TestPoll_FetcherError(t *testing.T) {
	t.Parallel()

	fetchErr := errors.New("fetch failed")
	var calls int32
	_, err := Poll(context.Background(), Config{Interval: time.Millisecond}, func(ctx context.Context) (string, bool, error) {
		if atomic.AddInt32(&calls, 1) >= 2 {
			return "", false, fetchErr
		}
		return "", false, nil
	})
	if !errors.Is(err, fetchErr) {
		t.Fatalf("expected fetchErr, got %v", err)
	}
}

func TestPoll_OnProgress(t *testing.T) {
	t.Parallel()

	var progressCalls int32
	var calls int32
	_, _ = Poll(context.Background(), Config{
		Interval:    time.Millisecond,
		MaxAttempts: 4,
		OnProgress: func(attempt int, elapsed time.Duration) {
			atomic.AddInt32(&progressCalls, 1)
			if attempt < 1 {
				t.Errorf("attempt should be >= 1, got %d", attempt)
			}
			if elapsed < 0 {
				t.Errorf("elapsed should be >= 0, got %v", elapsed)
			}
		},
	}, func(ctx context.Context) (string, bool, error) {
		atomic.AddInt32(&calls, 1)
		return "", false, nil
	})

	// OnProgress is called after each non-final, non-done attempt (before waiting).
	// With MaxAttempts=4, attempts 1,2,3 call OnProgress (attempt 4 hits the limit and exits).
	if c := atomic.LoadInt32(&progressCalls); c != 3 {
		t.Errorf("expected 3 progress calls, got %d", c)
	}
}

func TestPoll_Backoff(t *testing.T) {
	t.Parallel()

	var timestamps []time.Time
	var calls int32
	_, _ = Poll(context.Background(), Config{
		Interval:    5 * time.Millisecond,
		Backoff:     2.0,
		MaxInterval: 100 * time.Millisecond,
		MaxAttempts: 4,
	}, func(ctx context.Context) (string, bool, error) {
		timestamps = append(timestamps, time.Now())
		atomic.AddInt32(&calls, 1)
		return "", false, nil
	})

	if len(timestamps) < 4 {
		t.Fatalf("expected at least 4 timestamps, got %d", len(timestamps))
	}

	// Verify intervals grow: gap[1] >= gap[0] (with tolerance)
	gap0 := timestamps[2].Sub(timestamps[1])
	gap1 := timestamps[3].Sub(timestamps[2])
	if gap1 < gap0/2 {
		t.Errorf("expected growing intervals: gap0=%v, gap1=%v", gap0, gap1)
	}
}
