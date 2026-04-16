package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

type mockEngine struct {
	calls atomic.Int32
}

func (m *mockEngine) Execute(_ context.Context, _ workflow.Graph) (Result, error) {
	m.calls.Add(1)
	return Result{Value: "result", Kind: OutputURL}, nil
}

func TestCache_HitAndMiss(t *testing.T) {
	t.Parallel()
	mock := &mockEngine{}
	c := WithCache(mock, time.Minute, 10)

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}

	// First call — cache miss.
	r1, err := c.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Value != "result" {
		t.Errorf("got %q", r1.Value)
	}
	if mock.calls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls.Load())
	}

	// Second call — cache hit.
	r2, err := c.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Value != "result" {
		t.Errorf("got %q", r2.Value)
	}
	if mock.calls.Load() != 1 {
		t.Errorf("expected still 1 call (cached), got %d", mock.calls.Load())
	}
}

func TestCache_DifferentGraphs(t *testing.T) {
	t.Parallel()
	mock := &mockEngine{}
	c := WithCache(mock, time.Minute, 10)

	g1 := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a"}},
	}
	g2 := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "b"}},
	}

	c.Execute(context.Background(), g1)
	c.Execute(context.Background(), g2)

	if mock.calls.Load() != 2 {
		t.Errorf("expected 2 calls for different graphs, got %d", mock.calls.Load())
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	mock := &mockEngine{}
	c := WithCache(mock, 10*time.Millisecond, 10)

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	c.Execute(context.Background(), g)
	time.Sleep(20 * time.Millisecond) // wait for TTL
	c.Execute(context.Background(), g)

	if mock.calls.Load() != 2 {
		t.Errorf("expected 2 calls after TTL, got %d", mock.calls.Load())
	}
}

func TestCache_LRUEviction(t *testing.T) {
	t.Parallel()
	mock := &mockEngine{}
	c := WithCache(mock, time.Minute, 2) // max 2 entries

	for i := 0; i < 3; i++ {
		g := workflow.Graph{
			"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": string(rune('a' + i))}},
		}
		c.Execute(context.Background(), g)
	}

	if c.Len() > 2 {
		t.Errorf("expected at most 2 entries, got %d", c.Len())
	}
}

func TestCache_Clear(t *testing.T) {
	t.Parallel()
	mock := &mockEngine{}
	c := WithCache(mock, time.Minute, 10)

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	c.Execute(context.Background(), g)
	if c.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", c.Len())
	}

	c.Clear()
	if c.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", c.Len())
	}
}
