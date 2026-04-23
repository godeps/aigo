package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestExecuteImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/1.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "gpt-image-1",
		Kind:    KindImage,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a red balloon"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/1.png" {
		t.Fatalf("got %q", out.Value)
	}
}

func TestExecuteVideoPoll(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/video/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"t1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/video/generations/t1":
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls < 2 {
				_, _ = w.Write([]byte(`{"task_id":"t1","status":"in_progress"}`))
			} else {
				_, _ = w.Write([]byte(`{"task_id":"t1","status":"completed","url":"https://v.example.com/out.mp4"}`))
			}
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL + "/v1",
		Model:             "kling-v1",
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "walk on mars"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/out.mp4" {
		t.Fatalf("got %q", out.Value)
	}
}

func TestExecuteKlingText2Video(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/kling/v1/videos/text2video":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"kt1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/kling/v1/videos/text2video/kt1":
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls < 2 {
				_, _ = w.Write([]byte(`{"task_id":"kt1","status":"in_progress"}`))
			} else {
				_, _ = w.Write([]byte(`{"task_id":"kt1","status":"completed","url":"https://v.example.com/k.mp4"}`))
			}
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "kling-v1",
		Route:             RouteKlingText2Video,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "fly"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/k.mp4" {
		t.Fatalf("got %q", out.Value)
	}
}

func TestExecuteSpeechDataURI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte{0xff, 0xf3, 0x90, 0x00})
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "tts-1",
		Kind:    KindSpeech,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "alloy", "response_format": "mp3"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Value) < 30 || out.Value[:5] != "data:" {
		t.Fatalf("unexpected %q", out.Value)
	}
}

func TestExecuteGPTImage2OmitsResponseFormatAndStyle(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "gpt-image-2",
		Kind:    KindImage,
		APIKey:  "sk-test",
		Quality: "high",
		Style:   "vivid", // must be silently dropped for gpt-image-*
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a misty mountain at sunrise"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotPayload["response_format"]; ok {
		t.Errorf("payload must omit response_format for gpt-image-*: %#v", gotPayload)
	}
	if _, ok := gotPayload["style"]; ok {
		t.Errorf("payload must omit style for gpt-image-*: %#v", gotPayload)
	}
	if gotPayload["quality"] != "high" {
		t.Errorf("quality = %#v, want \"high\"", gotPayload["quality"])
	}
	if gotPayload["model"] != "gpt-image-2" {
		t.Errorf("model = %#v, want \"gpt-image-2\"", gotPayload["model"])
	}
	wantPrefix := "data:image/png;base64,"
	if len(out.Value) < len(wantPrefix) || out.Value[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Execute() = %q, want data URI", out.Value)
	}
}

func TestLookupRouteGPTImage2(t *testing.T) {
	t.Parallel()

	route, kind := LookupRoute("gpt-image-2")
	if route != RouteOpenAIImagesGenerations {
		t.Errorf("route = %q, want %q", route, RouteOpenAIImagesGenerations)
	}
	if kind != KindImage {
		t.Errorf("kind = %q, want %q", kind, KindImage)
	}
}
