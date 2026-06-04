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
// Returning an error makes the client call fail after the response is received;
// observer hooks should return nil.
type ResponseHook interface {
	AfterResponse(*http.Response) error
}

// HookOption configures request/response hooks for HookTransport.
type HookOption func(*hookOptions)

type hookOptions struct {
	requestHooks  []RequestHook
	responseHooks []ResponseHook
}

// WithRequestHooks appends hooks that run before the underlying transport.
func WithRequestHooks(hooks ...RequestHook) HookOption {
	return func(o *hookOptions) {
		for _, hook := range hooks {
			if hook != nil {
				o.requestHooks = append(o.requestHooks, hook)
			}
		}
	}
}

// WithResponseHooks appends hooks that run after the underlying transport
// returns a response.
func WithResponseHooks(hooks ...ResponseHook) HookOption {
	return func(o *hookOptions) {
		for _, hook := range hooks {
			if hook != nil {
				o.responseHooks = append(o.responseHooks, hook)
			}
		}
	}
}

// HookTransport applies request and response hooks around an underlying
// RoundTripper.
type HookTransport struct {
	Base          http.RoundTripper
	RequestHooks  []RequestHook
	ResponseHooks []ResponseHook
}

func (t *HookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	for _, hook := range t.RequestHooks {
		req, err = hook.BeforeRequest(req)
		if err != nil {
			return nil, err
		}
		if req == nil {
			return nil, ErrNilHookRequest
		}
	}

	resp, err := roundTripperOrDefault(t.Base).RoundTrip(req)
	if resp != nil {
		for _, hook := range t.ResponseHooks {
			if hookErr := hook.AfterResponse(resp); hookErr != nil && err == nil {
				err = hookErr
			}
		}
	}
	return resp, err
}

// WithHTTPHooks returns a shallow clone of client whose transport applies the
// supplied request/response hooks.
func WithHTTPHooks(client *http.Client, opts ...HookOption) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	var cfg hookOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	out := *client
	out.Transport = &HookTransport{
		Base:          client.Transport,
		RequestHooks:  append([]RequestHook(nil), cfg.requestHooks...),
		ResponseHooks: append([]ResponseHook(nil), cfg.responseHooks...),
	}
	return &out
}

// AppendHTTPHooks returns a shallow clone of client with hooks appended. If the
// client already uses HookTransport, hooks are appended to the existing
// transport instead of nesting another HookTransport.
func AppendHTTPHooks(client *http.Client, opts ...HookOption) *http.Client {
	if client == nil {
		return WithHTTPHooks(nil, opts...)
	}
	if _, ok := client.Transport.(*HookTransport); !ok {
		return WithHTTPHooks(client, opts...)
	}
	var cfg hookOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	out := *client
	existing := client.Transport.(*HookTransport)
	out.Transport = &HookTransport{
		Base:          existing.Base,
		RequestHooks:  append(append([]RequestHook(nil), existing.RequestHooks...), cfg.requestHooks...),
		ResponseHooks: append(append([]ResponseHook(nil), existing.ResponseHooks...), cfg.responseHooks...),
	}
	return &out
}

func roundTripperOrDefault(rt http.RoundTripper) http.RoundTripper {
	if rt != nil {
		return rt
	}
	return http.DefaultTransport
}
