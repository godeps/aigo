package pixabay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/material"
)

func TestDoRequest_ParsesJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total": 100,
			"totalHits": 20,
			"hits": [{
				"id": 1234,
				"pageURL": "https://pixabay.com/photos/1234/",
				"tags": "nature, forest, green",
				"webformatURL": "https://pixabay.com/get/1234_640.jpg",
				"largeImageURL": "https://pixabay.com/get/1234_1280.jpg",
				"imageWidth": 5000,
				"imageHeight": 3333,
				"imageSize": 2048000,
				"user": "Photographer1"
			}]
		}`))
	}))
	defer srv.Close()

	s := &Searcher{apiKey: "test-key", client: srv.Client(), limiter: material.NewRateLimiter(1000)}
	body, err := s.doRequest(context.Background(), srv.URL+"?key=test-key&q=nature")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}

func TestDoRequest_ServerError_Retries(t *testing.T) {
	t.Parallel()
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			w.Write([]byte(`internal error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"total":0,"totalHits":0,"hits":[]}`))
	}))
	defer srv.Close()

	s := &Searcher{apiKey: "key", client: srv.Client(), limiter: material.NewRateLimiter(1000)}
	body, err := s.doRequest(context.Background(), srv.URL+"?key=key&q=test")
	if err != nil {
		t.Fatalf("should retry on 500 and succeed, got: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
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

func TestSearch_VideoResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total": 5,
			"totalHits": 5,
			"hits": [{
				"id": 999,
				"pageURL": "https://pixabay.com/videos/999/",
				"tags": "ocean, waves",
				"picture_id": "https://i.vimeocdn.com/video/999.jpg",
				"duration": 30,
				"user": "VideoMaker",
				"videos": {
					"large": {"url": "https://videos.pixabay.com/999_large.mp4", "width": 1920, "height": 1080, "size": 50000000},
					"medium": {"url": "https://videos.pixabay.com/999_med.mp4", "width": 1280, "height": 720, "size": 25000000}
				}
			}]
		}`))
	}))
	defer srv.Close()

	s := &Searcher{apiKey: "key", client: srv.Client(), limiter: material.NewRateLimiter(1000)}
	body, err := s.doRequest(context.Background(), srv.URL+"?key=key&q=ocean")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}
