package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimitTransport_AllowsRequests(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RateLimitTransport{
			Limiter: rate.NewLimiter(100, 10), // generous limit
		},
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRateLimitTransport_Throttles(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// 2 requests per second, burst 1
	client := &http.Client{
		Transport: &RateLimitTransport{
			Limiter: rate.NewLimiter(2, 1),
		},
	}

	start := time.Now()
	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	elapsed := time.Since(start)

	if count.Load() != 3 {
		t.Errorf("expected 3 requests, got %d", count.Load())
	}
	// 3 requests at 2 rps (burst 1): first immediate, 2nd waits ~500ms, 3rd waits ~500ms
	if elapsed < 800*time.Millisecond {
		t.Errorf("expected throttling (≥800ms), got %v", elapsed)
	}
}

func TestRateLimitTransport_RespectsContext(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Very slow rate, but cancel context immediately.
	client := &http.Client{
		Transport: &RateLimitTransport{
			Limiter: rate.NewLimiter(0.001, 0), // essentially blocked
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error from rate-limited + cancelled context")
	}
}

func TestNewRateLimitedClient(t *testing.T) {
	t.Parallel()
	c := NewRateLimitedClient(10, 5, 30*time.Second)
	if c.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", c.Timeout)
	}
	rt, ok := c.Transport.(*RateLimitTransport)
	if !ok {
		t.Fatal("expected RateLimitTransport")
	}
	if rt.Limiter == nil {
		t.Error("expected non-nil limiter")
	}
}
