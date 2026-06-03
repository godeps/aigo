package material

import (
	"context"
	"sync"
	"time"
)

// MultiSearcher aggregates multiple Searcher backends and merges results.
type MultiSearcher struct {
	backends []Searcher
	timeout  time.Duration
}

// MultiOption configures MultiSearcher behavior.
type MultiOption func(*MultiSearcher)

// WithTimeout sets a per-backend timeout. If a backend doesn't respond
// within this duration, its results are skipped. Default: 15s.
func WithTimeout(d time.Duration) MultiOption {
	return func(m *MultiSearcher) { m.timeout = d }
}

// NewMultiSearcher creates a searcher that queries multiple backends concurrently.
func NewMultiSearcher(backends ...Searcher) *MultiSearcher {
	return &MultiSearcher{backends: backends, timeout: 15 * time.Second}
}

// NewMultiSearcherWithOptions creates a MultiSearcher with custom options.
func NewMultiSearcherWithOptions(backends []Searcher, opts ...MultiOption) *MultiSearcher {
	m := &MultiSearcher{backends: backends, timeout: 15 * time.Second}
	for _, o := range opts {
		o(m)
	}
	return m
}

// BackendError records a single backend failure during multi-search.
type BackendError struct {
	Source string
	Err    error
}

func (e BackendError) Error() string {
	return e.Source + ": " + e.Err.Error()
}

// MultiResult extends Result with partial failure information.
type MultiResult struct {
	Result
	Errors []BackendError
}

// Search queries all backends concurrently and merges results.
// Partial failures are tolerated — failed backends are recorded in MultiResult.Errors.
func (m *MultiSearcher) Search(ctx context.Context, req Request) (Result, error) {
	mr := m.SearchMulti(ctx, req)
	return mr.Result, nil
}

// SearchMulti is like Search but returns MultiResult with error details
// and a composite NextToken for pagination across backends.
func (m *MultiSearcher) SearchMulti(ctx context.Context, req Request) MultiResult {
	type outcome struct {
		source string
		result Result
		err    error
	}

	// Decode per-backend pagination state from composite NextToken.
	pageState := DecodePagination(req.NextToken)

	ch := make(chan outcome, len(m.backends))
	var wg sync.WaitGroup

	for _, b := range m.backends {
		wg.Add(1)
		go func(s Searcher) {
			defer wg.Done()

			searchCtx := ctx
			if m.timeout > 0 {
				var cancel context.CancelFunc
				searchCtx, cancel = context.WithTimeout(ctx, m.timeout)
				defer cancel()
			}

			// Inject per-backend NextToken from decoded state.
			backendReq := req
			source := ""
			if d, ok := s.(Describer); ok {
				source = d.Source()
			}
			if source != "" && pageState != nil {
				if tok, ok := pageState[source]; ok {
					backendReq.NextToken = tok
				}
			}

			r, err := s.Search(searchCtx, backendReq)
			if r.Source == "" {
				r.Source = source
			}
			ch <- outcome{source: r.Source, result: r, err: err}
		}(b)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var mr MultiResult
	mr.Source = "multi"
	nextState := PaginationState{}

	for o := range ch {
		if o.err != nil {
			mr.Errors = append(mr.Errors, BackendError{Source: o.source, Err: o.err})
			continue
		}
		mr.Items = append(mr.Items, o.result.Items...)
		mr.Total += o.result.Total
		if o.result.NextToken != "" {
			nextState[o.source] = o.result.NextToken
		}
	}

	if len(nextState) > 0 {
		mr.NextToken = EncodePagination(nextState)
	}

	if req.MaxResults > 0 && len(mr.Items) > req.MaxResults {
		mr.Items = mr.Items[:req.MaxResults]
	}

	return mr
}
