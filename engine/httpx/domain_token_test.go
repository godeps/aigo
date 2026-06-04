package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDomainTokenTransportInjectsBearer(t *testing.T) {
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

func TestDomainTokenTransportDoesNotOverrideAuthorization(t *testing.T) {
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

func TestDomainTokenTransportSkipsUnmatchedDomain(t *testing.T) {
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
