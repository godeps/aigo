package embed

import (
	"context"
	"testing"
)

func TestTextRequest(t *testing.T) {
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
	data := []byte{0x00, 0x00, 0x00}
	req := VideoRequest(data, "")
	if req.Type != ContentVideo {
		t.Errorf("expected ContentVideo, got %d", req.Type)
	}
}

func TestRateLimiter(t *testing.T) {
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
