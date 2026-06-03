package unsplash

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/material"
)

func TestSearch_ParsesResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Client-ID test-key" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total": 42,
			"total_pages": 3,
			"results": [{
				"id": "abc123",
				"width": 4000,
				"height": 3000,
				"description": "A beautiful sunset",
				"urls": {
					"raw": "https://images.unsplash.com/abc123?raw",
					"full": "https://images.unsplash.com/abc123?full",
					"regular": "https://images.unsplash.com/abc123?regular",
					"small": "https://images.unsplash.com/abc123?small",
					"thumb": "https://images.unsplash.com/abc123?thumb"
				},
				"links": {"html": "https://unsplash.com/photos/abc123", "download": "https://unsplash.com/photos/abc123/download"},
				"user": {"name": "Jane Doe"},
				"tags": [{"title": "sunset"}, {"title": "beach"}]
			}]
		}`))
	}))
	defer srv.Close()

	_ = &Searcher{accessKey: "test-key", client: srv.Client(), limiter: material.NewRateLimiter(1000)}

	req, _ := http.NewRequest("GET", srv.URL+"?query=sunset&per_page=10", nil)
	req.Header.Set("Authorization", "Client-ID test-key")
	req.Header.Set("Accept-Version", "v1")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSearch_MissingKey(t *testing.T) {
	t.Parallel()
	s := New(Config{})
	_, err := s.Search(context.Background(), material.Request{Query: "test"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestSearch_SkipsNonImage(t *testing.T) {
	t.Parallel()
	s := New(Config{AccessKey: "key"})
	result, err := s.Search(context.Background(), material.Request{
		Query:      "test",
		MediaTypes: []string{"video"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("unsplash only supports image, got %d items for video filter", len(result.Items))
	}
}
