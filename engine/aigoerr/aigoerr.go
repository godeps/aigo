// Package aigoerr provides structured, classifiable errors for aigo engines.
// Agents can use IsRetryable and GetCode to make informed retry decisions.
package aigoerr

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Code classifies an error for agent retry logic.
type Code int

const (
	CodeUnknown        Code = iota
	CodeInvalidInput        // 400
	CodeAuthentication      // 401, 403
	CodeQuotaExceeded       // 402
	CodeRateLimit           // 429
	CodeServerError         // 5xx
	CodeTimeout             // context deadline / timeout
	CodeUnavailable         // engine unreachable
)

// Error is a structured error carrying classification metadata.
type Error struct {
	Code       Code
	StatusCode int
	Message    string
	Retryable  bool
	RetryAfter time.Duration
	Err        error // wrapped original
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s (code=%d, retryable=%t): %v", e.Message, e.StatusCode, e.Retryable, e.Err)
	}
	return fmt.Sprintf("%s (code=%d, retryable=%t)", e.Message, e.StatusCode, e.Retryable)
}

func (e *Error) Unwrap() error { return e.Err }

// FromHTTPResponse creates a classified Error from an HTTP response.
// prefix is prepended to the message (e.g., "newapi", "openai").
func FromHTTPResponse(resp *http.Response, body []byte, prefix string) *Error {
	code, retryable := classifyStatus(resp.StatusCode)
	msg := fmt.Sprintf("%s: status %s: %s", prefix, resp.Status, strings.TrimSpace(string(body)))

	var retryAfter time.Duration
	if code == CodeRateLimit {
		retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		retryable = true
	}

	return &Error{
		Code:       code,
		StatusCode: resp.StatusCode,
		Message:    msg,
		Retryable:  retryable,
		RetryAfter: retryAfter,
	}
}

// IsRetryable checks the error chain for a retryable *Error.
func IsRetryable(err error) bool {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Retryable
	}
	return false
}

// GetCode extracts the error Code from the chain, if present.
func GetCode(err error) (Code, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code, true
	}
	return CodeUnknown, false
}

func classifyStatus(status int) (Code, bool) {
	switch {
	case status == 400:
		return CodeInvalidInput, false
	case status == 401 || status == 403:
		return CodeAuthentication, false
	case status == 402:
		return CodeQuotaExceeded, false
	case status == 429:
		return CodeRateLimit, true
	case status >= 500:
		return CodeServerError, true
	default:
		return CodeUnknown, false
	}
}

func parseRetryAfter(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
