package newapi

import (
	"net/http"
	"net/http/httptest"
)

// newTestServer creates a test HTTP server with the given handler.
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
