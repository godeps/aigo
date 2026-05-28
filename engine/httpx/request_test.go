package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
)

func TestDoJSON_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %q", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %q", auth)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	body, err := DoJSON(context.Background(), http.DefaultClient, http.MethodPost, srv.URL, "test-key", []byte(`{"prompt":"hi"}`), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestDoJSON_Non2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	_, err := DoJSON(context.Background(), http.DefaultClient, http.MethodPost, srv.URL, "key", []byte(`{}`), "test")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	var ae *aigoerr.Error
	if ok := errors.As(err, &ae); !ok {
		t.Fatalf("expected *aigoerr.Error, got %T", err)
	}
	if ae.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", ae.StatusCode)
	}
}

func TestDoJSON_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal"))
	}))
	defer srv.Close()

	_, err := DoJSON(context.Background(), http.DefaultClient, http.MethodPost, srv.URL, "key", nil, "pfx")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDoJSON_CancelledContext(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DoJSON(ctx, http.DefaultClient, http.MethodGet, srv.URL, "key", nil, "test")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestDoJSON_InvalidURL(t *testing.T) {
	t.Parallel()
	_, err := DoJSON(context.Background(), http.DefaultClient, http.MethodGet, "://bad-url", "key", nil, "test")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestDoJSON_ConnectionRefused(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	_, err := DoJSON(context.Background(), client, http.MethodGet, "http://127.0.0.1:1", "key", nil, "test")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}
