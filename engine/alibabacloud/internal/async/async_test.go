package async

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
)

func TestNestedString_TopLevelString(t *testing.T) {
	got, ok := nestedString(map[string]any{"url": "https://x"}, "url")
	if !ok || got != "https://x" {
		t.Fatalf("nestedString(top) = %q, %v", got, ok)
	}
}

func TestNestedString_NestedPath(t *testing.T) {
	v := map[string]any{
		"output": map[string]any{
			"results": map[string]any{"video_url": "https://v"},
		},
	}
	got, ok := nestedString(v, "output", "results", "video_url")
	if !ok || got != "https://v" {
		t.Fatalf("nestedString(nested) = %q, %v", got, ok)
	}
}

func TestNestedString_AutoArrayFirstElement(t *testing.T) {
	v := map[string]any{
		"results": []any{
			map[string]any{"url": "https://first"},
			map[string]any{"url": "https://second"},
		},
	}
	got, ok := nestedString(v, "results", "url")
	if !ok || got != "https://first" {
		t.Fatalf("nestedString(array auto-first) = %q, %v; want first element", got, ok)
	}
}

func TestNestedString_EmptyArrayReturnsFalse(t *testing.T) {
	v := map[string]any{"results": []any{}}
	if got, ok := nestedString(v, "results", "url"); ok {
		t.Fatalf("nestedString(empty array) = %q, true; want false", got)
	}
}

func TestNestedString_MissingKeyReturnsFalse(t *testing.T) {
	if got, ok := nestedString(map[string]any{"a": "b"}, "missing"); ok {
		t.Fatalf("nestedString(missing) = %q, true; want false", got)
	}
}

func TestNestedString_NonObjectMidPathReturnsFalse(t *testing.T) {
	v := map[string]any{"a": "scalar"}
	if got, ok := nestedString(v, "a", "b"); ok {
		t.Fatalf("nestedString(scalar mid) = %q, true; want false (cannot traverse into string)", got)
	}
}

func TestNestedString_TerminalNonStringReturnsFalse(t *testing.T) {
	v := map[string]any{"count": 42}
	if got, ok := nestedString(v, "count"); ok {
		t.Fatalf("nestedString(int terminal) = %q, true; want false", got)
	}
}

func TestNestedString_DeepNesting(t *testing.T) {
	v := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "deep",
				},
			},
		},
	}
	got, ok := nestedString(v, "a", "b", "c", "d")
	if !ok || got != "deep" {
		t.Fatalf("nestedString(deep) = %q, %v", got, ok)
	}
}

// TestWait_SucceededWithoutURL verifies the bug fix: when the task SUCCEEDS
// but no extractor path matches, we MUST return a classified error rather
// than silently returning the taskID as the URL (data corruption risk).
func TestWait_SucceededWithoutURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "SUCCEEDED",
				// No matching URL field.
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url, err := Wait(ctx, rt, "k", "task-xyz", URLExtractor{URLFields: [][]string{{"results", "url"}}})
	if err == nil {
		t.Fatalf("Wait succeeded but no URL — expected error, got url=%q", url)
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("Wait err = %v, want *aigoerr.Error", err)
	}
	if ae.Retryable {
		t.Errorf("expected non-retryable error, got Retryable=true")
	}
	if !strings.Contains(ae.Message, "task-xyz") {
		t.Errorf("error message should reference taskID; got %q", ae.Message)
	}
}

// TestWait_FailedReturnsClassifiedError verifies FAILED task status produces
// an aigoerr.Error (not bare fmt.Errorf) so agents can route retries.
func TestWait_FailedReturnsClassifiedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "FAILED",
				"code":        "InvalidParameter",
				"message":     "image url cannot be reached",
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Wait(ctx, rt, "k", "task-fail", URLExtractor{URLFields: [][]string{{"results", "url"}}})
	if err == nil {
		t.Fatalf("Wait should have failed for FAILED task, got nil")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("Wait err = %v, want *aigoerr.Error in chain", err)
	}
	if ae.Retryable {
		t.Errorf("FAILED task should be non-retryable")
	}
	if !strings.Contains(ae.Message, "InvalidParameter") || !strings.Contains(ae.Message, "image url cannot be reached") {
		t.Errorf("error should include code+message details, got %q", ae.Message)
	}
}

// TestWait_SucceededWithURLReturnsURL is the happy path — verifies the fix
// did not break the normal SUCCEEDED-with-URL flow.
func TestWait_SucceededWithURLReturnsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "SUCCEEDED",
				"results":     []any{map[string]any{"url": "https://result"}},
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url, err := Wait(ctx, rt, "k", "task-ok", URLExtractor{URLFields: [][]string{{"results", "url"}}})
	if err != nil {
		t.Fatalf("Wait err = %v, want success", err)
	}
	if url != "https://result" {
		t.Errorf("Wait url = %q, want https://result", url)
	}
}
