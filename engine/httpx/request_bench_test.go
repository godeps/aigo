package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkDoJSON(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := &http.Client{}
	body := []byte(`{"prompt":"test"}`)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DoJSON(ctx, client, http.MethodPost, srv.URL, "test-key", body, "bench")
	}
}
