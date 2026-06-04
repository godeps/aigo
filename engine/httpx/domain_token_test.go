package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDomainTokenHookInjectsBearer(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("X-DashScope-Request-Id", "req-123")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := WithDomainTokens(srv.Client(), []DomainToken{{Domain: "127.0.0.1", Token: "tok"}})
	capture := NewResponseCapture()
	ctx := WithResponseCapture(context.Background(), capture)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer tok" {
		t.Fatalf("Authorization = %q, want Bearer tok", gotAuth)
	}
	records := capture.Records()
	if len(records) != 1 {
		t.Fatalf("captured records = %d, want 1", len(records))
	}
	if got := records[0].Headers["X-Dashscope-Request-Id"]; len(got) != 1 || got[0] != "req-123" {
		t.Fatalf("captured dashscope request id = %v", got)
	}
	if records[0].StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", records[0].StatusCode, http.StatusAccepted)
	}
}

func TestDomainTokenHookDoesNotOverrideAuthorization(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := WithDomainTokens(srv.Client(), []DomainToken{{Domain: "127.0.0.1", Token: "tok"}})
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer explicit")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer explicit" {
		t.Fatalf("Authorization = %q, want explicit header", gotAuth)
	}
}

func TestDomainTokenHookSkipsUnmatchedDomain(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := WithDomainTokens(srv.Client(), []DomainToken{{Domain: "example.com", Token: "tok"}})
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}

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
		testRequestHook{header: "X-Custom", value: "from-hook"},
		respHook,
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
