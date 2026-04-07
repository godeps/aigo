package ark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestExecuteTextToVideo(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/contents/generations/tasks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-test-001"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/contents/generations/tasks/cgt-test-001":
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls < 2 {
				_, _ = w.Write([]byte(`{"id":"cgt-test-001","status":"running"}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"cgt-test-001","status":"succeeded","content":{"video_url":"https://v.example.com/seedance.mp4"}}`))
			}
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "A cat playing piano"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/seedance.mp4" {
		t.Fatalf("got %q", out.Value)
	}
}

func TestExecuteNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/contents/generations/tasks" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cgt-nowait-002"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "Sunset over ocean"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "cgt-nowait-002" {
		t.Fatalf("expected task id, got %q", out.Value)
	}
}

func TestExecuteFailed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-fail-003"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-fail-003","status":"failed","error":{"code":"content_filter","message":"content blocked"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "content blocked") {
		t.Fatalf("expected content blocked error, got: %v", err)
	}
}

func TestExecuteImageToVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-i2v-004"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-i2v-004","status":"succeeded","content":{"video_url":"https://v.example.com/i2v.mp4"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "animate this image"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{
			"url":  "https://example.com/photo.jpg",
			"role": "first_frame",
		}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/i2v.mp4" {
		t.Fatalf("got %q", out.Value)
	}

	// verify payload structure
	contentArr, ok := gotPayload["content"].([]any)
	if !ok || len(contentArr) < 2 {
		t.Fatalf("expected at least 2 content items, got %v", gotPayload["content"])
	}
	imgItem, ok := contentArr[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", contentArr[1])
	}
	if imgItem["type"] != "image_url" {
		t.Fatalf("expected image_url type, got %v", imgItem["type"])
	}
	if imgItem["role"] != "first_frame" {
		t.Fatalf("expected first_frame role, got %v", imgItem["role"])
	}
}

func TestExecuteMultiModal(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-mm-005"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-mm-005","status":"succeeded","content":{"video_url":"https://v.example.com/mm.mp4"}}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "multi-modal prompt"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{
			"url":  "https://example.com/ref.jpg",
			"role": "reference_image",
		}},
		"3": {ClassType: "LoadVideo", Inputs: map[string]any{
			"url": "https://example.com/ref.mp4",
		}},
		"4": {ClassType: "LoadAudio", Inputs: map[string]any{
			"url": "https://example.com/bgm.mp3",
		}},
		"5": {ClassType: "VideoOptions", Inputs: map[string]any{
			"duration":       10,
			"generate_audio": true,
		}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://v.example.com/mm.mp4" {
		t.Fatalf("got %q", out.Value)
	}

	contentArr, ok := gotPayload["content"].([]any)
	if !ok {
		t.Fatalf("expected content array, got %T", gotPayload["content"])
	}
	// text + image + video + audio = 4
	if len(contentArr) != 4 {
		t.Fatalf("expected 4 content items, got %d", len(contentArr))
	}
}

func TestExecuteExpired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-exp-006"}`))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cgt-exp-006","status":"expired"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "doubao-seedance-2-0-260128",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for expired task")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}
}

func TestMissingContent(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "test-model",
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1280, "height": 720}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
	if !strings.Contains(err.Error(), "no content") {
		t.Fatalf("expected missing content error, got: %v", err)
	}
}
