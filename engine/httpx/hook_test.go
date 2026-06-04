package httpx

import (
	"net/http"
	"net/http/httptest"
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
