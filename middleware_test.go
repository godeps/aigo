package aigo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
)

func validGraph() workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	}
}

func TestWithLogging(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := NewClient()
	_ = c.RegisterEngine("s", stubEngine{result: "ok"})
	c.Use(WithLogging(&buf))

	_, err := c.Execute(context.Background(), "s", validGraph())
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `engine="s"`) {
		t.Fatalf("log missing engine name: %s", out)
	}
	if !strings.Contains(out, "start") || !strings.Contains(out, "ok") {
		t.Fatalf("log missing phases: %s", out)
	}
}

type failNEngine struct {
	failures int
	called   int
}

func (e *failNEngine) Execute(context.Context, workflow.Graph) (engine.Result, error) {
	e.called++
	if e.called <= e.failures {
		return engine.Result{}, &aigoerr.Error{
			Code:      aigoerr.CodeServerError,
			Message:   "server error",
			Retryable: true,
		}
	}
	return engine.Result{Value: "success"}, nil
}

func TestWithRetry_RecoversAfterTransientFailure(t *testing.T) {
	t.Parallel()
	fe := &failNEngine{failures: 2}
	c := NewClient()
	_ = c.RegisterEngine("r", fe)
	c.Use(WithRetry(3))

	r, err := c.Execute(context.Background(), "r", validGraph())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "success" {
		t.Fatalf("got %q", r.Value)
	}
	if fe.called != 3 {
		t.Fatalf("expected 3 calls, got %d", fe.called)
	}
}

func TestWithRetry_NonRetryableStopsImmediately(t *testing.T) {
	t.Parallel()
	c := NewClient()
	permanent := &aigoerr.Error{Code: aigoerr.CodeInvalidInput, Message: "bad", Retryable: false}
	_ = c.RegisterEngine("p", stubEngine{err: permanent})
	c.Use(WithRetry(3))

	_, err := c.Execute(context.Background(), "p", validGraph())
	if err == nil {
		t.Fatal("expected error")
	}
	// Should not retry non-retryable errors
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected aigoerr.Error, got %T", err)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	t.Parallel()
	var order []string
	mw := func(tag string) Middleware {
		return func(name string, next engine.Engine) engine.Engine {
			return middlewareFunc(func(ctx context.Context, g workflow.Graph) (engine.Result, error) {
				order = append(order, tag+"-before")
				r, err := next.Execute(ctx, g)
				order = append(order, tag+"-after")
				return r, err
			})
		}
	}

	c := NewClient()
	_ = c.RegisterEngine("s", stubEngine{result: "ok"})
	c.Use(mw("A"), mw("B"))

	_, err := c.Execute(context.Background(), "s", validGraph())
	if err != nil {
		t.Fatal(err)
	}
	// A is outermost, B is inner
	expected := "A-before,B-before,B-after,A-after"
	got := strings.Join(order, ",")
	if got != expected {
		t.Fatalf("order = %q, want %q", got, expected)
	}
}

type middlewareFunc func(context.Context, workflow.Graph) (engine.Result, error)

func (f middlewareFunc) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	return f(ctx, g)
}

// timeoutNEngine returns a wrapped context.DeadlineExceeded for the first N
// calls, then succeeds. This simulates HTTP client timeouts (not parent ctx).
type timeoutNEngine struct {
	failures int
	called   int
}

func (e *timeoutNEngine) Execute(_ context.Context, _ workflow.Graph) (engine.Result, error) {
	e.called++
	if e.called <= e.failures {
		return engine.Result{}, fmt.Errorf("aliyun: call multimodal image api: %w", context.DeadlineExceeded)
	}
	return engine.Result{Value: "recovered"}, nil
}

func TestWithRetry_RecoversAfterTimeout(t *testing.T) {
	t.Parallel()
	te := &timeoutNEngine{failures: 2}
	c := NewClient()
	_ = c.RegisterEngine("t", te)
	c.Use(WithRetry(3))

	r, err := c.Execute(context.Background(), "t", validGraph())
	if err != nil {
		t.Fatalf("expected recovery, got: %v", err)
	}
	if r.Value != "recovered" {
		t.Fatalf("got %q, want recovered", r.Value)
	}
	if te.called != 3 {
		t.Fatalf("expected 3 calls, got %d", te.called)
	}
}

func TestWithRetry_TimeoutButParentCancelled(t *testing.T) {
	t.Parallel()
	te := &timeoutNEngine{failures: 10}
	c := NewClient()
	_ = c.RegisterEngine("t", te)
	c.Use(WithRetry(5))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Execute(ctx, "t", validGraph())
	if err == nil {
		t.Fatal("expected error when parent ctx cancelled")
	}
	// Should stop after at most 1-2 calls, not retry all 5 times.
	if te.called > 2 {
		t.Fatalf("retried %d times despite cancelled ctx", te.called)
	}
}

func TestWithRetry_ProgressCallback(t *testing.T) {
	t.Parallel()
	fe := &failNEngine{failures: 2}
	c := NewClient()
	_ = c.RegisterEngine("r", fe)
	c.Use(WithRetry(3))

	var mu sync.Mutex
	var attempts []int
	ctx := poll.WithOnProgressV2(context.Background(), func(info poll.ProgressInfo) {
		mu.Lock()
		defer mu.Unlock()
		attempts = append(attempts, info.Attempt)
	})

	r, err := c.Execute(ctx, "r", validGraph())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "success" {
		t.Fatalf("got %q", r.Value)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(attempts) != 2 {
		t.Fatalf("expected 2 progress callbacks, got %d: %v", len(attempts), attempts)
	}
	if attempts[0] != 1 || attempts[1] != 2 {
		t.Fatalf("expected attempts [1,2], got %v", attempts)
	}
}

// retryAfterEngine returns an *aigoerr.Error with RetryAfter set.
type retryAfterEngine struct {
	retryAfter time.Duration
	failures   int
	called     int
}

func (e *retryAfterEngine) Execute(context.Context, workflow.Graph) (engine.Result, error) {
	e.called++
	if e.called <= e.failures {
		return engine.Result{}, &aigoerr.Error{
			Code:       aigoerr.CodeRateLimit,
			Message:    "rate limited",
			Retryable:  true,
			RetryAfter: e.retryAfter,
		}
	}
	return engine.Result{Value: "ok"}, nil
}

func TestWithRetry_RespectsRetryAfter(t *testing.T) {
	t.Parallel()
	re := &retryAfterEngine{retryAfter: 100 * time.Millisecond, failures: 1}
	c := NewClient()
	_ = c.RegisterEngine("ra", re)
	c.Use(WithRetry(2))

	start := time.Now()
	r, err := c.Execute(context.Background(), "ra", validGraph())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "ok" {
		t.Fatalf("got %q", r.Value)
	}
	// With Retry-After=100ms, delay should be ~100ms, not 1s (default exponential).
	if elapsed > 500*time.Millisecond {
		t.Fatalf("delay too long (%s), expected ~100ms from Retry-After", elapsed)
	}
}

func TestWithRetry_ExhaustedErrorIncludesDetails(t *testing.T) {
	t.Parallel()
	c := NewClient()
	_ = c.RegisterEngine("f", stubEngine{err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded)})
	c.Use(WithRetry(2))

	_, err := c.Execute(context.Background(), "f", validGraph())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "retries exhausted") {
		t.Fatalf("error should mention retries exhausted: %s", msg)
	}
	if !strings.Contains(msg, "2") {
		t.Fatalf("error should contain retry count: %s", msg)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("should unwrap to DeadlineExceeded")
	}
}
