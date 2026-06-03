package material

import (
	"context"
	"sync"
)

// MultiSearcher aggregates multiple Searcher backends and merges results.
type MultiSearcher struct {
	backends []Searcher
}

// NewMultiSearcher creates a searcher that queries multiple backends concurrently.
func NewMultiSearcher(backends ...Searcher) *MultiSearcher {
	return &MultiSearcher{backends: backends}
}

// Search queries all backends concurrently and merges results.
func (m *MultiSearcher) Search(ctx context.Context, req Request) (Result, error) {
	type outcome struct {
		result Result
		err    error
	}

	ch := make(chan outcome, len(m.backends))
	var wg sync.WaitGroup

	for _, b := range m.backends {
		wg.Add(1)
		go func(s Searcher) {
			defer wg.Done()
			r, err := s.Search(ctx, req)
			ch <- outcome{result: r, err: err}
		}(b)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var merged Result
	merged.Source = "multi"
	for o := range ch {
		if o.err != nil {
			continue
		}
		merged.Items = append(merged.Items, o.result.Items...)
		merged.Total += o.result.Total
	}

	if req.MaxResults > 0 && len(merged.Items) > req.MaxResults {
		merged.Items = merged.Items[:req.MaxResults]
	}

	return merged, nil
}
