package poll

import (
	"context"
	"testing"
	"time"
)

func BenchmarkPoll_FixedInterval(b *testing.B) {
	// Poll with a fetcher that succeeds on first attempt
	// Use very small interval
	for i := 0; i < b.N; i++ {
		Poll(context.Background(), Config{Interval: time.Microsecond}, func(ctx context.Context) (string, bool, error) {
			return "done", true, nil
		})
	}
}

func BenchmarkPoll_WithBackoff(b *testing.B) {
	// Poll with backoff, fetcher succeeds after 3 attempts
	for i := 0; i < b.N; i++ {
		attempt := 0
		Poll(context.Background(), Config{
			Interval: time.Microsecond,
			Backoff:  2.0,
		}, func(ctx context.Context) (string, bool, error) {
			attempt++
			return "done", attempt >= 3, nil
		})
	}
}
