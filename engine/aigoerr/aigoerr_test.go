package aigoerr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestFromHTTPResponse_400(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 400, Status: "400 Bad Request", Header: http.Header{}}
	e := FromHTTPResponse(resp, []byte("invalid prompt"), "test")
	if e.Code != CodeInvalidInput {
		t.Fatalf("code = %v, want CodeInvalidInput", e.Code)
	}
	if e.Retryable {
		t.Fatal("400 should not be retryable")
	}
	if e.StatusCode != 400 {
		t.Fatalf("StatusCode = %d", e.StatusCode)
	}
}

func TestFromHTTPResponse_401(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 401, Status: "401 Unauthorized", Header: http.Header{}}
	e := FromHTTPResponse(resp, []byte("bad key"), "test")
	if e.Code != CodeAuthentication {
		t.Fatalf("code = %v", e.Code)
	}
	if e.Retryable {
		t.Fatal("401 should not be retryable")
	}
}

func TestFromHTTPResponse_429(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("Retry-After", "30")
	resp := &http.Response{StatusCode: 429, Status: "429 Too Many Requests", Header: h}
	e := FromHTTPResponse(resp, []byte("rate limited"), "test")
	if e.Code != CodeRateLimit {
		t.Fatalf("code = %v", e.Code)
	}
	if !e.Retryable {
		t.Fatal("429 should be retryable")
	}
	if e.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v", e.RetryAfter)
	}
}

func TestFromHTTPResponse_500(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 500, Status: "500 Internal Server Error", Header: http.Header{}}
	e := FromHTTPResponse(resp, []byte("oops"), "test")
	if e.Code != CodeServerError {
		t.Fatalf("code = %v", e.Code)
	}
	if !e.Retryable {
		t.Fatal("500 should be retryable")
	}
}

func TestFromHTTPResponse_402(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 402, Status: "402 Payment Required", Header: http.Header{}}
	e := FromHTTPResponse(resp, []byte("no quota"), "test")
	if e.Code != CodeQuotaExceeded {
		t.Fatalf("code = %v", e.Code)
	}
	if e.Retryable {
		t.Fatal("402 should not be retryable")
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	retryable := &Error{Code: CodeRateLimit, Retryable: true, Message: "rate limit"}
	notRetryable := &Error{Code: CodeInvalidInput, Retryable: false, Message: "bad input"}

	if !IsRetryable(retryable) {
		t.Fatal("expected retryable")
	}
	if IsRetryable(notRetryable) {
		t.Fatal("expected not retryable")
	}
	if IsRetryable(errors.New("plain error")) {
		t.Fatal("plain errors should not be retryable")
	}
	// wrapped
	wrapped := fmt.Errorf("outer: %w", retryable)
	if !IsRetryable(wrapped) {
		t.Fatal("wrapped retryable should be retryable")
	}
}

func TestIsRetryable_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	if !IsRetryable(context.DeadlineExceeded) {
		t.Fatal("DeadlineExceeded should be retryable")
	}
}

func TestIsRetryable_WrappedDeadlineExceeded(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("aliyun: call multimodal image api: %w", context.DeadlineExceeded)
	if !IsRetryable(err) {
		t.Fatal("wrapped DeadlineExceeded should be retryable")
	}
}

func TestIsRetryable_ContextCanceled(t *testing.T) {
	t.Parallel()
	if IsRetryable(context.Canceled) {
		t.Fatal("Canceled should NOT be retryable")
	}
	wrapped := fmt.Errorf("canceled: %w", context.Canceled)
	if IsRetryable(wrapped) {
		t.Fatal("wrapped Canceled should NOT be retryable")
	}
}

type mockNetError struct{ timeout bool }

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

func TestIsRetryable_NetTimeout(t *testing.T) {
	t.Parallel()
	timeoutErr := &mockNetError{timeout: true}
	if !IsRetryable(timeoutErr) {
		t.Fatal("net.Error with Timeout()=true should be retryable")
	}
	nonTimeoutErr := &mockNetError{timeout: false}
	if IsRetryable(nonTimeoutErr) {
		t.Fatal("net.Error with Timeout()=false should NOT be retryable")
	}
}

func TestIsRetryable_UnexpectedEOF(t *testing.T) {
	t.Parallel()
	if !IsRetryable(io.ErrUnexpectedEOF) {
		t.Fatal("ErrUnexpectedEOF should be retryable")
	}
	wrapped := fmt.Errorf("read response: %w", io.ErrUnexpectedEOF)
	if !IsRetryable(wrapped) {
		t.Fatal("wrapped ErrUnexpectedEOF should be retryable")
	}
}

func TestIsRetryable_AigoErrorTakesPriority(t *testing.T) {
	t.Parallel()
	// An *Error with Retryable=false wrapping DeadlineExceeded: the
	// structured classification should win over the native error check.
	ae := &Error{
		Code:      CodeInvalidInput,
		Message:   "bad input",
		Retryable: false,
		Err:       context.DeadlineExceeded,
	}
	if IsRetryable(ae) {
		t.Fatal("*Error{Retryable:false} should NOT be retryable even if wrapping DeadlineExceeded")
	}
}

func TestIsTransientNative(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, false},
		{"unexpected_eof", io.ErrUnexpectedEOF, true},
		{"net_timeout", &mockNetError{timeout: true}, true},
		{"net_non_timeout", &mockNetError{timeout: false}, false},
		{"plain", errors.New("plain"), false},
		{"wrapped_deadline", fmt.Errorf("outer: %w", context.DeadlineExceeded), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransientNative(tt.err); got != tt.want {
				t.Fatalf("IsTransientNative(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestGetCode(t *testing.T) {
	t.Parallel()
	e := &Error{Code: CodeAuthentication, Message: "auth"}
	code, ok := GetCode(e)
	if !ok || code != CodeAuthentication {
		t.Fatalf("code=%v, ok=%v", code, ok)
	}
	_, ok = GetCode(errors.New("plain"))
	if ok {
		t.Fatal("plain error should not have code")
	}
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	e := &Error{Code: CodeServerError, Message: "server", Err: inner}
	if !errors.Is(e, inner) {
		t.Fatal("should unwrap to inner")
	}
}

func TestError_ErrorString(t *testing.T) {
	t.Parallel()
	e := &Error{Code: CodeRateLimit, StatusCode: 429, Message: "test: rate limited", Retryable: true}
	s := e.Error()
	if s == "" {
		t.Fatal("empty error string")
	}
	eWithInner := &Error{Code: CodeServerError, StatusCode: 500, Message: "test: server", Retryable: true, Err: errors.New("boom")}
	s2 := eWithInner.Error()
	if s2 == "" {
		t.Fatal("empty error string with inner")
	}
}
