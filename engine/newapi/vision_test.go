package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestRunChatCompletions_TextOnly(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Hello world."}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe something"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "Hello world." {
		t.Errorf("got %q, want %q", out.Value, "Hello world.")
	}
	if out.Kind != engine.OutputPlainText {
		t.Errorf("kind = %v, want PlainText", out.Kind)
	}

	msgs, ok := gotPayload["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatalf("messages = %v, want non-empty []any", gotPayload["messages"])
	}
	msg, ok := msgs[0].(map[string]any)
	if !ok {
		t.Fatalf("msg[0] = %T, want map[string]any", msgs[0])
	}
	if content, ok := msg["content"].(string); !ok || content != "describe something" {
		t.Fatalf("content = %v, want plain string", msg["content"])
	}
}

func TestRunChatCompletions_WithVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"A person is walking."}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-vl-max",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this video"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/video.mp4"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "A person is walking." {
		t.Errorf("got %q", out.Value)
	}

	msgs, ok := gotPayload["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatalf("messages = %v, want non-empty []any", gotPayload["messages"])
	}
	msg, ok := msgs[0].(map[string]any)
	if !ok {
		t.Fatalf("msg[0] = %T, want map[string]any", msgs[0])
	}
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("expected array content, got %T", msg["content"])
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	videoPart, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatalf("parts[0] = %T, want map[string]any", parts[0])
	}
	if videoPart["type"] != "video_url" {
		t.Fatalf("part[0] type = %v, want video_url", videoPart["type"])
	}
	videoURL, ok := videoPart["video_url"].(map[string]any)
	if !ok {
		t.Fatalf("video_url = %T, want map[string]any", videoPart["video_url"])
	}
	if videoURL["url"] != "https://example.com/video.mp4" {
		t.Fatalf("video_url = %v", videoURL["url"])
	}
	textPart, ok := parts[1].(map[string]any)
	if !ok {
		t.Fatalf("parts[1] = %T, want map[string]any", parts[1])
	}
	if textPart["type"] != "text" {
		t.Fatalf("part[1] type = %v, want text", textPart["type"])
	}
}

func TestRunChatCompletions_WithImageAndVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Analysis done."}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-vl-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "compare"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/clip.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/frame.jpg"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "Analysis done." {
		t.Errorf("got %q", out.Value)
	}

	msgs, ok := gotPayload["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatalf("messages = %v, want non-empty []any", gotPayload["messages"])
	}
	msg, ok := msgs[0].(map[string]any)
	if !ok {
		t.Fatalf("msg[0] = %T, want map[string]any", msgs[0])
	}
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("content = %T, want []any", msg["content"])
	}
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	p0, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatalf("parts[0] = %T, want map[string]any", parts[0])
	}
	if p0["type"] != "video_url" {
		t.Fatalf("part[0] type = %v, want video_url", p0["type"])
	}
	p1, ok := parts[1].(map[string]any)
	if !ok {
		t.Fatalf("parts[1] = %T, want map[string]any", parts[1])
	}
	if p1["type"] != "image_url" {
		t.Fatalf("part[1] type = %v, want image_url", p1["type"])
	}
	p2, ok := parts[2].(map[string]any)
	if !ok {
		t.Fatalf("parts[2] = %T, want map[string]any", parts[2])
	}
	if p2["type"] != "text" {
		t.Fatalf("part[2] type = %v, want text", p2["type"])
	}
}

func TestRunChatCompletions_LookupRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model   string
		wantCap string
	}{
		{"qwen3.6-plus", "video_understanding"},
		{"qwen-vl-max", "video_understanding"},
		{"qwen-vl-plus", "video_understanding"},
		{"qwen-vl-max-latest", "video_understanding"},
		{"qwen-vl-plus-latest", "video_understanding"},
	}
	for _, tt := range tests {
		route, kind, cap := LookupRoute(tt.model, "")
		if route != RouteChatCompletions {
			t.Errorf("%s: route = %v, want RouteChatCompletions", tt.model, route)
		}
		if kind != KindVision {
			t.Errorf("%s: kind = %v, want KindVision", tt.model, kind)
		}
		if cap != tt.wantCap {
			t.Errorf("%s: cap = %q, want %q", tt.model, cap, tt.wantCap)
		}
	}
}

func TestRunChatCompletions_Streaming(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if payload["stream"] != true {
			t.Errorf("expected stream=true, got %v", payload["stream"])
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":" world"}}]}`,
			`{"choices":[{"delta":{"content":"."}}]}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
			if flusher != nil {
				flusher.Flush()
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-vl-max",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "Hello world." {
		t.Errorf("got %q, want %q", out.Value, "Hello world.")
	}
}

func TestRunChatCompletions_InferVL(t *testing.T) {
	t.Parallel()

	route, kind, cap := InferFromModelName("some-custom-vl-model")
	if route != RouteChatCompletions {
		t.Errorf("route = %v, want RouteChatCompletions", route)
	}
	if kind != KindVision {
		t.Errorf("kind = %v, want KindVision", kind)
	}
	if cap != "video_understanding" {
		t.Errorf("cap = %q, want video_understanding", cap)
	}
}
