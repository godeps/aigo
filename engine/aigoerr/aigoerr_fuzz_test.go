package aigoerr

import (
	"net/http"
	"testing"
)

func FuzzFromHTTPResponse(f *testing.F) {
	// Seed with common status codes and body patterns
	for _, tc := range []struct {
		status int
		body   string
	}{
		{200, "ok"},
		{400, `{"error": "bad request"}`},
		{401, "unauthorized"},
		{429, `{"error": "rate limited"}`},
		{500, "internal error"},
		{502, ""},
	} {
		f.Add(tc.status, tc.body)
	}
	f.Fuzz(func(t *testing.T, status int, body string) {
		if status < 100 || status > 599 {
			return // invalid HTTP status
		}
		resp := &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{},
		}
		// Should never panic
		e := FromHTTPResponse(resp, []byte(body), "fuzz")
		if e == nil {
			t.Error("FromHTTPResponse returned nil")
		}
	})
}
