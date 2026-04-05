package httpx

import (
	"net/http"
	"testing"
	"time"
)

func TestOrDefaultNil(t *testing.T) {
	t.Parallel()
	c := OrDefault(nil, 5*time.Second)
	if c == nil || c.Timeout != 5*time.Second {
		t.Fatalf("got %+v", c)
	}
}

func TestOrDefaultPreservesTimeout(t *testing.T) {
	t.Parallel()
	in := &http.Client{Timeout: 7 * time.Second}
	c := OrDefault(in, 99*time.Second)
	if c != in {
		t.Fatal("expected same client when Timeout > 0")
	}
}

func TestOrDefaultFillsZeroTimeout(t *testing.T) {
	t.Parallel()
	in := &http.Client{}
	c := OrDefault(in, 11*time.Second)
	if c.Timeout != 11*time.Second {
		t.Fatalf("timeout %v", c.Timeout)
	}
	if c == in {
		t.Fatal("expected cloned client")
	}
}
