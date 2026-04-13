package aigo

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/workflow"
)

// WithLogging returns middleware that logs engine calls to the given writer.
func WithLogging(w io.Writer) Middleware {
	return func(name string, next engine.Engine) engine.Engine {
		return &loggingEngine{name: name, next: next, w: w}
	}
}

type loggingEngine struct {
	name string
	next engine.Engine
	w    io.Writer
}

func (e *loggingEngine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	start := time.Now()
	fmt.Fprintf(e.w, "[aigo] engine=%q start\n", e.name)
	r, err := e.next.Execute(ctx, graph)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(e.w, "[aigo] engine=%q error=%v elapsed=%s\n", e.name, err, elapsed)
	} else {
		fmt.Fprintf(e.w, "[aigo] engine=%q ok kind=%d elapsed=%s\n", e.name, r.Kind, elapsed)
	}
	return r, err
}

// WithRetry returns middleware that retries on retryable errors up to maxRetries times.
func WithRetry(maxRetries int) Middleware {
	return func(name string, next engine.Engine) engine.Engine {
		return &retryEngine{next: next, maxRetries: maxRetries}
	}
}

type retryEngine struct {
	next       engine.Engine
	maxRetries int
}

func (e *retryEngine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		r, err := e.next.Execute(ctx, graph)
		if err == nil {
			return r, nil
		}
		lastErr = err
		if !aigoerr.IsRetryable(err) {
			return r, err
		}
		if ctx.Err() != nil {
			return engine.Result{}, ctx.Err()
		}
		// Exponential backoff: 1s, 2s, 4s, ...
		if attempt < e.maxRetries {
			delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return engine.Result{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return engine.Result{}, lastErr
}
