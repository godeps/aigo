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

// ---------------------------------------------------------------------------
// Submit tests
// ---------------------------------------------------------------------------

func TestSubmit_NoWait_ReturnsTaskID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-DashScope-Async") != "enable" {
			t.Errorf("missing X-DashScope-Async header")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"task_id": "task-123"},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        srv.Client(),
		WaitForCompletion: false,
	}
	ctx := context.Background()
	taskID, err := Submit(ctx, rt, "test-key", "/api/gen", map[string]any{"prompt": "hello"}, URLExtractor{})
	if err != nil {
		t.Fatalf("Submit err = %v", err)
	}
	if taskID != "task-123" {
		t.Errorf("Submit taskID = %q, want task-123", taskID)
	}
}

func TestSubmit_WaitForCompletion_PollsUntilSucceeded(t *testing.T) {
	t.Parallel()
	var attempt int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{"task_id": "task-poll"},
			})
			return
		}
		// GET — polling
		attempt++
		if attempt < 3 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{"task_status": "RUNNING"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "SUCCEEDED",
				"video_url":   "https://video-result",
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        srv.Client(),
		WaitForCompletion: true,
		PollInterval:      time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url, err := Submit(ctx, rt, "k", "/api/gen", map[string]any{}, URLExtractor{
		URLFields: [][]string{{"video_url"}},
	})
	if err != nil {
		t.Fatalf("Submit err = %v", err)
	}
	if url != "https://video-result" {
		t.Errorf("Submit url = %q, want https://video-result", url)
	}
}

func TestSubmit_CreationHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"InvalidParameter","message":"bad input"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
	_, err := Submit(context.Background(), rt, "k", "/api/gen", map[string]any{}, URLExtractor{})
	if err == nil {
		t.Fatal("Submit should fail on 400")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error", err)
	}
	if ae.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", ae.StatusCode)
	}
}

func TestSubmit_CreationMissingTaskID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"output": map[string]any{}})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
	_, err := Submit(context.Background(), rt, "k", "/api/gen", map[string]any{}, URLExtractor{})
	if err == nil {
		t.Fatal("Submit should fail when task_id is missing")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Errorf("err = %v, should mention task_id", err)
	}
}

func TestSubmit_CreationInvalidJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
	_, err := Submit(context.Background(), rt, "k", "/api/gen", map[string]any{}, URLExtractor{})
	if err == nil {
		t.Fatal("Submit should fail on invalid JSON response")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("err = %v, should mention decode", err)
	}
}

// ---------------------------------------------------------------------------
// Wait / fetch — additional status tests
// ---------------------------------------------------------------------------

func TestWait_CanceledStatusReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "CANCELED",
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

	_, err := Wait(ctx, rt, "k", "task-c", URLExtractor{})
	if err == nil {
		t.Fatal("Wait should fail on CANCELED status")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error", err)
	}
	if !strings.Contains(ae.Message, "CANCELED") {
		t.Errorf("message = %q, should mention CANCELED", ae.Message)
	}
}

func TestWait_UnknownStatusReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "UNKNOWN",
				"code":        "InternalError",
				"message":     "something went wrong",
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

	_, err := Wait(ctx, rt, "k", "task-u", URLExtractor{})
	if err == nil {
		t.Fatal("Wait should fail on UNKNOWN status")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error", err)
	}
	if !strings.Contains(ae.Message, "InternalError") || !strings.Contains(ae.Message, "something went wrong") {
		t.Errorf("message = %q, should include code and message details", ae.Message)
	}
}

func TestWait_PendingThenSucceeded(t *testing.T) {
	t.Parallel()
	var attempt int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{"task_status": "PENDING"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "SUCCEEDED",
				"image_url":   "https://img",
			},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url, err := Wait(ctx, rt, "k", "task-p", URLExtractor{URLFields: [][]string{{"image_url"}}})
	if err != nil {
		t.Fatalf("Wait err = %v", err)
	}
	if url != "https://img" {
		t.Errorf("url = %q, want https://img", url)
	}
}

func TestWait_PollingHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Wait(ctx, rt, "k", "task-err", URLExtractor{})
	if err == nil {
		t.Fatal("Wait should fail on HTTP 500 polling response")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error in chain", err)
	}
	if ae.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", ae.StatusCode)
	}
}

func TestWait_ContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"task_status": "RUNNING"},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: 50 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := Wait(ctx, rt, "k", "task-ctx", URLExtractor{})
	if err == nil {
		t.Fatal("Wait should fail on context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestWait_FailedNoExtras(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "FAILED",
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

	_, err := Wait(ctx, rt, "k", "task-fnx", URLExtractor{})
	if err == nil {
		t.Fatal("Wait should fail on FAILED status")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error", err)
	}
	if !strings.Contains(ae.Message, "FAILED") {
		t.Errorf("message = %q, should mention FAILED", ae.Message)
	}
}

func TestWait_MultipleExtractorPaths(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_status": "SUCCEEDED",
				"fallback":    "https://fallback-url",
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

	url, err := Wait(ctx, rt, "k", "task-multi", URLExtractor{
		URLFields: [][]string{
			{"primary_url"},   // missing
			{"fallback"},      // present
		},
	})
	if err != nil {
		t.Fatalf("Wait err = %v", err)
	}
	if url != "https://fallback-url" {
		t.Errorf("url = %q, want https://fallback-url", url)
	}
}

// ---------------------------------------------------------------------------
// nestedString — additional edge cases
// ---------------------------------------------------------------------------

func TestNestedString_TerminalArrayUnwrap(t *testing.T) {
	t.Parallel()
	v := map[string]any{
		"results": []any{"https://url-in-array"},
	}
	got, ok := nestedString(v, "results")
	if !ok || got != "https://url-in-array" {
		t.Fatalf("nestedString(terminal array string) = %q, %v; want https://url-in-array, true", got, ok)
	}
}

func TestNestedString_TerminalEmptyArrayReturnsFalse(t *testing.T) {
	t.Parallel()
	v := map[string]any{
		"results": []any{},
	}
	if got, ok := nestedString(v, "results"); ok {
		t.Fatalf("nestedString(terminal empty array) = %q, true; want false", got)
	}
}

func TestNestedString_NestedArrayOfArrays(t *testing.T) {
	t.Parallel()
	v := map[string]any{
		"data": []any{
			[]any{
				map[string]any{"url": "https://nested-array"},
			},
		},
	}
	got, ok := nestedString(v, "data", "url")
	if !ok || got != "https://nested-array" {
		t.Fatalf("nestedString(nested arrays) = %q, %v; want https://nested-array, true", got, ok)
	}
}

func TestNestedString_EmptyPath(t *testing.T) {
	t.Parallel()
	got, ok := nestedString("direct-string")
	if !ok || got != "direct-string" {
		t.Fatalf("nestedString(empty path) = %q, %v; want direct-string, true", got, ok)
	}
}
