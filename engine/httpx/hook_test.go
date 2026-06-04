package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testRequestHook struct {
	header string
	value  string
}

func (h testRequestHook) BeforeRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set(h.header, h.value)
	return clone, nil
}

type testResponseHook struct {
	status int
	header string
}

func (h *testResponseHook) AfterResponse(resp *http.Response) error {
	h.status = resp.StatusCode
	h.header = resp.Header.Get("X-Hooked")
	return nil
}

func TestWithHTTPHooksAllowsCustomRequestAndResponseHooks(t *testing.T) {
	t.Parallel()

	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.Header().Set("X-Hooked", "yes")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	respHook := &testResponseHook{}
	client := WithHTTPHooks(
		srv.Client(),
		WithRequestHooks(testRequestHook{header: "X-Custom", value: "from-hook"}),
		WithResponseHooks(respHook),
	)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotHeader != "from-hook" {
		t.Fatalf("X-Custom = %q, want from-hook", gotHeader)
	}
	if respHook.status != http.StatusCreated {
		t.Fatalf("captured status = %d, want %d", respHook.status, http.StatusCreated)
	}
	if respHook.header != "yes" {
		t.Fatalf("captured header = %q, want yes", respHook.header)
	}
}

func TestAppendHTTPHooksDoesNotNestHookTransport(t *testing.T) {
	t.Parallel()

	client := WithHTTPHooks(nil, WithRequestHooks(testRequestHook{header: "X-One", value: "1"}))
	client = AppendHTTPHooks(client, WithRequestHooks(testRequestHook{header: "X-Two", value: "2"}))

	transport, ok := client.Transport.(*HookTransport)
	if !ok {
		t.Fatalf("transport = %T, want *HookTransport", client.Transport)
	}
	if _, nested := transport.Base.(*HookTransport); nested {
		t.Fatal("expected AppendHTTPHooks to avoid nested HookTransport")
	}
	if len(transport.RequestHooks) != 2 {
		t.Fatalf("request hook count = %d, want 2", len(transport.RequestHooks))
	}
}

func TestResponseCaptureFiltersSensitiveHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-DashScope-Request-Id", "req-123")
		w.Header().Set("X-RateLimit-Remaining", "9")
		w.Header().Set("Set-Cookie", "sid=secret")
		w.Header().Set("X-Api-Key", "secret")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	capture := NewResponseCapture()
	ctx := WithResponseCapture(context.Background(), capture)
	client := WithHTTPHooks(srv.Client(), WithResponseHooks(ResponseCaptureHook{}))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("captured records = %d, want 1", len(records))
	}
	headers := records[0].Headers
	if records[0].Method != http.MethodGet {
		t.Fatalf("captured method = %q, want GET", records[0].Method)
	}
	if got := headers["X-Dashscope-Request-Id"]; len(got) != 1 || got[0] != "req-123" {
		t.Fatalf("captured request id = %v", got)
	}
	if got := headers["X-Ratelimit-Remaining"]; len(got) != 1 || got[0] != "9" {
		t.Fatalf("captured ratelimit = %v", got)
	}
	for _, name := range []string{"Set-Cookie", "X-Api-Key", "Content-Type"} {
		if _, ok := headers[name]; ok {
			t.Fatalf("sensitive or unlisted header %q was captured: %v", name, headers[name])
		}
	}
}

func TestResponseCaptureLimitsRecordsAndHeaderValueLength(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("a", defaultMaxResponseHeaderValue+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", longValue)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	capture := NewResponseCapture()
	ctx := WithResponseCapture(context.Background(), capture)
	client := WithHTTPHooks(srv.Client(), WithResponseHooks(ResponseCaptureHook{}))
	for i := 0; i < defaultMaxResponseRecords+5; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	records := capture.Records()
	if len(records) != defaultMaxResponseRecords {
		t.Fatalf("captured records = %d, want %d", len(records), defaultMaxResponseRecords)
	}
	got := records[0].Headers["X-Request-Id"]
	if len(got) != 1 || len(got[0]) != defaultMaxResponseHeaderValue {
		t.Fatalf("captured X-Request-Id length = %v, want %d", got, defaultMaxResponseHeaderValue)
	}
}
