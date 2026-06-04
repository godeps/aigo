package httpx

import (
	"errors"
	"net/http"
)

var ErrNilHookRequest = errors.New("httpx: request hook returned nil request")

// RequestHook can inspect or replace an outbound HTTP request before it is
// sent. Implementations must not mutate the input request in place unless they
// own it; clone the request when changing headers, URL, or body.
type RequestHook interface {
	BeforeRequest(*http.Request) (*http.Request, error)
}

// ResponseHook can inspect a response returned by the wrapped transport.
type ResponseHook interface {
	AfterResponse(*http.Response) error
}

// HookTransport applies request and response hooks around an underlying
// RoundTripper.
type HookTransport struct {
	Base  http.RoundTripper
	Hooks []any
}

func (t *HookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	for _, hook := range t.Hooks {
		if h, ok := hook.(RequestHook); ok {
			req, err = h.BeforeRequest(req)
			if err != nil {
				return nil, err
			}
			if req == nil {
				return nil, ErrNilHookRequest
			}
		}
	}

	resp, err := roundTripperOrDefault(t.Base).RoundTrip(req)
	if resp != nil {
		for _, hook := range t.Hooks {
			if h, ok := hook.(ResponseHook); ok {
				if hookErr := h.AfterResponse(resp); hookErr != nil && err == nil {
					err = hookErr
				}
			}
		}
	}
	return resp, err
}

// WithHTTPHooks returns a shallow clone of client whose transport applies the
// supplied request/response hooks. Nil hooks are ignored.
func WithHTTPHooks(client *http.Client, hooks ...any) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	filtered := make([]any, 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	out := *client
	out.Transport = &HookTransport{
		Base:  client.Transport,
		Hooks: filtered,
	}
	return &out
}

func roundTripperOrDefault(rt http.RoundTripper) http.RoundTripper {
	if rt != nil {
		return rt
	}
	return http.DefaultTransport
}
