package material

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockSearcher is a controllable test backend.
type mockSearcher struct {
	source string
	items  []Item
	err    error
	delay  time.Duration
}

func (m *mockSearcher) Search(ctx context.Context, req Request) (Result, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	if m.err != nil {
		return Result{}, m.err
	}
	return Result{
		Items:  m.items,
		Total:  len(m.items),
		Source: m.source,
	}, nil
}

func (m *mockSearcher) Source() string                { return m.source }
func (m *mockSearcher) SupportedMediaTypes() []string { return []string{"image"} }

func TestMultiSearcher_MergesResults(t *testing.T) {
	t.Parallel()
	b1 := &mockSearcher{source: "a", items: []Item{{ID: "1", Source: "a"}}}
	b2 := &mockSearcher{source: "b", items: []Item{{ID: "2", Source: "b"}, {ID: "3", Source: "b"}}}

	ms := NewMultiSearcher(b1, b2)
	result, err := ms.Search(context.Background(), Request{Query: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(result.Items))
	}
	if result.Total != 3 {
		t.Fatalf("total = %d, want 3", result.Total)
	}
}

func TestMultiSearcher_MaxResults(t *testing.T) {
	t.Parallel()
	b1 := &mockSearcher{source: "a", items: []Item{{ID: "1"}, {ID: "2"}, {ID: "3"}}}
	b2 := &mockSearcher{source: "b", items: []Item{{ID: "4"}, {ID: "5"}}}

	ms := NewMultiSearcher(b1, b2)
	result, err := ms.Search(context.Background(), Request{Query: "test", MaxResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("got %d items, want 3 (capped)", len(result.Items))
	}
}

func TestMultiSearcher_PartialFailure(t *testing.T) {
	t.Parallel()
	b1 := &mockSearcher{source: "good", items: []Item{{ID: "1", Source: "good"}}}
	b2 := &mockSearcher{source: "bad", err: fmt.Errorf("connection refused")}

	ms := NewMultiSearcher(b1, b2)
	mr := ms.SearchMulti(context.Background(), Request{Query: "test"})

	if len(mr.Items) != 1 {
		t.Fatalf("got %d items, want 1 (good backend only)", len(mr.Items))
	}
	if len(mr.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(mr.Errors))
	}
	if mr.Errors[0].Source != "bad" {
		t.Fatalf("error source = %q, want bad", mr.Errors[0].Source)
	}
}

func TestMultiSearcher_Timeout(t *testing.T) {
	t.Parallel()
	b1 := &mockSearcher{source: "fast", items: []Item{{ID: "1"}}}
	b2 := &mockSearcher{source: "slow", items: []Item{{ID: "2"}}, delay: 5 * time.Second}

	ms := NewMultiSearcherWithOptions([]Searcher{b1, b2}, WithTimeout(100*time.Millisecond))
	mr := ms.SearchMulti(context.Background(), Request{Query: "test"})

	if len(mr.Items) != 1 {
		t.Fatalf("got %d items, want 1 (slow timed out)", len(mr.Items))
	}
	if len(mr.Errors) != 1 {
		t.Fatalf("got %d errors, want 1 (timeout)", len(mr.Errors))
	}
}

func TestMultiSearcher_EmptyBackends(t *testing.T) {
	t.Parallel()
	ms := NewMultiSearcher()
	result, err := ms.Search(context.Background(), Request{Query: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("got %d items, want 0", len(result.Items))
	}
}
