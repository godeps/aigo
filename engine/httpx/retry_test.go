package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryTransport_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 3, BaseDelay: time.Millisecond},
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

func TestRetryTransport_RetriesOn500(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 3, BaseDelay: time.Millisecond},
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 after retries, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestRetryTransport_RetriesOn429(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 2, BaseDelay: time.Millisecond},
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 after 429 retry, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_NoRetryForPOST(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 3, BaseDelay: time.Millisecond},
	}
	resp, err := client.Post(srv.URL, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := count.Load(); got != 1 {
		t.Errorf("POST should not retry, but got %d attempts", got)
	}
}

func TestRetryTransport_RespectsContextCancel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 3, BaseDelay: time.Millisecond},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetryTransport_ExhaustsRetries(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &RetryTransport{MaxRetries: 2, BaseDelay: time.Millisecond},
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("expected 500 after exhausting retries, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 total attempts, got %d", got)
	}
}

func TestNewRetryClient(t *testing.T) {
	t.Parallel()
	c := NewRetryClient(2, 10*time.Second)
	if c.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", c.Timeout)
	}
	if _, ok := c.Transport.(*RetryTransport); !ok {
		t.Error("expected RetryTransport")
	}
}
