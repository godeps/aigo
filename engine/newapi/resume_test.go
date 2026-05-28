package newapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResumeOpenAIVideo(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls < 2 {
			w.Write([]byte(`{"task_id":"vid1","status":"in_progress"}`))
		} else {
			w.Write([]byte(`{"task_id":"vid1","status":"completed","url":"https://cdn.example.com/out.mp4"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "test-video",
		Route:             RouteOpenAIVideoGenerations,
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	out, err := eng.Resume(context.Background(), "vid1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/out.mp4" {
		t.Errorf("got %q", out.Value)
	}
}

func TestResumeUnsupportedRoute(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test",
		Route:   RouteOpenAIImagesGenerations,
		APIKey:  "sk-test",
	})

	_, err := eng.Resume(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for unsupported route")
	}
}

func TestResumeMissingBaseURL(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		Model:  "test",
		APIKey: "sk-test",
	})

	_, err := eng.Resume(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
	if err != ErrMissingBaseURL {
		t.Errorf("got %v, want ErrMissingBaseURL", err)
	}
}

func TestResumeMissingAPIKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("NEWAPI_API_KEY", "")

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test",
		Route:   RouteOpenAIVideoGenerations,
	})

	_, err := eng.Resume(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
