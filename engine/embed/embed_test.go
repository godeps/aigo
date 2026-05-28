package embed

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
)

func TestTextRequest(t *testing.T) {
	t.Parallel()
	req := TextRequest("hello world", "RETRIEVAL_QUERY")
	if req.Type != ContentText {
		t.Errorf("expected ContentText, got %d", req.Type)
	}
	if req.Content != "hello world" {
		t.Errorf("expected 'hello world', got %v", req.Content)
	}
	if req.TaskType != "RETRIEVAL_QUERY" {
		t.Errorf("expected RETRIEVAL_QUERY, got %s", req.TaskType)
	}
}

func TestImageRequest(t *testing.T) {
	t.Parallel()
	data := []byte{0xFF, 0xD8, 0xFF}
	req := ImageRequest(data, "RETRIEVAL_DOCUMENT")
	if req.Type != ContentImage {
		t.Errorf("expected ContentImage, got %d", req.Type)
	}
	got := req.Content.([]byte)
	if len(got) != 3 {
		t.Errorf("expected 3 bytes, got %d", len(got))
	}
}

func TestVideoRequest(t *testing.T) {
	t.Parallel()
	data := []byte{0x00, 0x00, 0x00}
	req := VideoRequest(data, "")
	if req.Type != ContentVideo {
		t.Errorf("expected ContentVideo, got %d", req.Type)
	}
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(5)
	ctx := context.Background()

	// Should allow 5 requests immediately
	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("request %d should not be rate limited: %v", i, err)
		}
	}
}

func TestRateLimiter_ContextCancel(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(1)
	ctx := context.Background()

	// First request passes
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}

	// Second request with cancelled context should fail
	ctx2, cancel := context.WithCancel(context.Background())
	cancel()
	if err := rl.Wait(ctx2); err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestRateLimiter_PrunesOldTimestamps(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(2)
	// Inject timestamps older than 1 minute so the pruning loop executes.
	old := time.Now().Add(-2 * time.Minute)
	rl.timestamps = []time.Time{old, old}

	ctx := context.Background()
	// Old entries are pruned, so both new requests should succeed immediately.
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("expected no error after pruning, got %v", err)
	}
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("expected no error after pruning, got %v", err)
	}
}

func TestRateLimiter_WaitsWhenFull(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(1)
	// Set a timestamp that expires in ~10ms so the time.After path fires quickly.
	almostExpired := time.Now().Add(-time.Minute + 10*time.Millisecond)
	rl.timestamps = []time.Time{almostExpired}

	ctx := context.Background()
	err := rl.Wait(ctx)
	if err != nil {
		t.Fatalf("expected nil after short wait, got %v", err)
	}
}

// --------------- Retry tests ---------------

func TestRetry_ImmediateSuccess(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Retry(func() error {
		calls++
		return nil
	}, 3, time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Retry(func() error {
		calls++
		if calls < 3 {
			return &aigoerr.Error{Retryable: true, Message: "transient"}
		}
		return nil
	}, 5, time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	t.Parallel()
	calls := 0
	permanent := &aigoerr.Error{Retryable: false, Message: "permanent"}
	err := Retry(func() error {
		calls++
		return permanent
	}, 3, time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for non-retryable), got %d", calls)
	}
}

func TestRetry_MaxRetriesExhausted(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Retry(func() error {
		calls++
		return &aigoerr.Error{Retryable: true, Message: "always fails"}
	}, 2, time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// initial attempt + 2 retries = 3 total calls
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_ZeroRetries(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Retry(func() error {
		calls++
		return &aigoerr.Error{Retryable: true, Message: "fail"}
	}, 0, time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}
