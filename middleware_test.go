package aigo

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/workflow"
)

func validGraph() workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "t"}},
	}
}

func TestWithLogging(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := NewClient()
	_ = c.RegisterEngine("s", stubEngine{result: "ok"})
	c.Use(WithLogging(&buf))

	_, err := c.Execute(context.Background(), "s", validGraph())
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `engine="s"`) {
		t.Fatalf("log missing engine name: %s", out)
	}
	if !strings.Contains(out, "start") || !strings.Contains(out, "ok") {
		t.Fatalf("log missing phases: %s", out)
	}
}

type failNEngine struct {
	failures int
	called   int
}

func (e *failNEngine) Execute(context.Context, workflow.Graph) (engine.Result, error) {
	e.called++
	if e.called <= e.failures {
		return engine.Result{}, &aigoerr.Error{
			Code:      aigoerr.CodeServerError,
			Message:   "server error",
			Retryable: true,
		}
	}
	return engine.Result{Value: "success"}, nil
}

func TestWithRetry_RecoversAfterTransientFailure(t *testing.T) {
	t.Parallel()
	fe := &failNEngine{failures: 2}
	c := NewClient()
	_ = c.RegisterEngine("r", fe)
	c.Use(WithRetry(3))

	r, err := c.Execute(context.Background(), "r", validGraph())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "success" {
		t.Fatalf("got %q", r.Value)
	}
	if fe.called != 3 {
		t.Fatalf("expected 3 calls, got %d", fe.called)
	}
}

func TestWithRetry_NonRetryableStopsImmediately(t *testing.T) {
	t.Parallel()
	c := NewClient()
	permanent := &aigoerr.Error{Code: aigoerr.CodeInvalidInput, Message: "bad", Retryable: false}
	_ = c.RegisterEngine("p", stubEngine{err: permanent})
	c.Use(WithRetry(3))

	_, err := c.Execute(context.Background(), "p", validGraph())
	if err == nil {
		t.Fatal("expected error")
	}
	// Should not retry non-retryable errors
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected aigoerr.Error, got %T", err)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	t.Parallel()
	var order []string
	mw := func(tag string) Middleware {
		return func(name string, next engine.Engine) engine.Engine {
			return middlewareFunc(func(ctx context.Context, g workflow.Graph) (engine.Result, error) {
				order = append(order, tag+"-before")
				r, err := next.Execute(ctx, g)
				order = append(order, tag+"-after")
				return r, err
			})
		}
	}

	c := NewClient()
	_ = c.RegisterEngine("s", stubEngine{result: "ok"})
	c.Use(mw("A"), mw("B"))

	_, err := c.Execute(context.Background(), "s", validGraph())
	if err != nil {
		t.Fatal(err)
	}
	// A is outermost, B is inner
	expected := "A-before,B-before,B-after,A-after"
	got := strings.Join(order, ",")
	if got != expected {
		t.Fatalf("order = %q, want %q", got, expected)
	}
}

type middlewareFunc func(context.Context, workflow.Graph) (engine.Result, error)

func (f middlewareFunc) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	return f(ctx, g)
}
