package qwenvl

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

func TestExecute_TextOnly(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"This is a description."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe something"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "This is a description." {
		t.Fatalf("Value = %q, want %q", result.Value, "This is a description.")
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("Kind = %v, want OutputPlainText", result.Kind)
	}

	msgs, ok := gotPayload["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %v", gotPayload["messages"])
	}
	msg := msgs[0].(map[string]any)
	if content, ok := msg["content"].(string); !ok || content != "describe something" {
		t.Fatalf("content = %v, want plain string", msg["content"])
	}
	if gotPayload["model"] != defaultModel {
		t.Fatalf("model = %v, want %q", gotPayload["model"], defaultModel)
	}
}

func TestExecute_WithImage(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"The image shows a cat."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this image"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/cat.jpg"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "The image shows a cat." {
		t.Fatalf("Value = %q", result.Value)
	}

	msgs := gotPayload["messages"].([]any)
	msg := msgs[0].(map[string]any)
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("expected array content for image request, got %T", msg["content"])
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(parts))
	}
	imgPart := parts[0].(map[string]any)
	if imgPart["type"] != "image_url" {
		t.Fatalf("first part type = %v, want image_url", imgPart["type"])
	}
	textPart := parts[1].(map[string]any)
	if textPart["type"] != "text" {
		t.Fatalf("second part type = %v, want text", textPart["type"])
	}
}

func TestExecute_WithVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"The video shows a person walking."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this video"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/video.mp4"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "The video shows a person walking." {
		t.Fatalf("Value = %q", result.Value)
	}

	msgs := gotPayload["messages"].([]any)
	msg := msgs[0].(map[string]any)
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("expected array content for video request, got %T", msg["content"])
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(parts))
	}
	videoPart := parts[0].(map[string]any)
	if videoPart["type"] != "video_url" {
		t.Fatalf("first part type = %v, want video_url", videoPart["type"])
	}
	videoURL := videoPart["video_url"].(map[string]any)
	if videoURL["url"] != "https://example.com/video.mp4" {
		t.Fatalf("video_url.url = %v", videoURL["url"])
	}
	textPart := parts[1].(map[string]any)
	if textPart["type"] != "text" {
		t.Fatalf("second part type = %v, want text", textPart["type"])
	}
}

func TestExecute_WithImageAndVideo(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Analysis complete."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "compare the image and video"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/clip.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/frame.jpg"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Value != "Analysis complete." {
		t.Fatalf("Value = %q", result.Value)
	}

	msgs := gotPayload["messages"].([]any)
	msg := msgs[0].(map[string]any)
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("expected array content, got %T", msg["content"])
	}
	// video first, then image, then text
	if len(parts) != 3 {
		t.Fatalf("expected 3 content parts, got %d", len(parts))
	}
	if parts[0].(map[string]any)["type"] != "video_url" {
		t.Fatalf("part[0] type = %v, want video_url", parts[0].(map[string]any)["type"])
	}
	if parts[1].(map[string]any)["type"] != "image_url" {
		t.Fatalf("part[1] type = %v, want image_url", parts[1].(map[string]any)["type"])
	}
	if parts[2].(map[string]any)["type"] != "text" {
		t.Fatalf("part[2] type = %v, want text", parts[2].(map[string]any)["type"])
	}
}

func TestExecute_MissingKey(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")

	eng := New(Config{})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestExecute_MissingPrompt(t *testing.T) {
	t.Parallel()

	eng := New(Config{APIKey: "key"})
	g := workflow.Graph{
		"1": {ClassType: "Something", Inputs: map[string]any{}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	eng := New(Config{})
	cap := eng.Capabilities()
	if len(cap.MediaTypes) != 3 {
		t.Fatalf("expected 3 media types, got %v", cap.MediaTypes)
	}
	want := map[string]bool{"text": true, "image": true, "video": true}
	for _, mt := range cap.MediaTypes {
		if !want[mt] {
			t.Errorf("unexpected media type %q", mt)
		}
	}
	if !cap.SupportsSync {
		t.Error("expected SupportsSync = true")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	m := ModelsByCapability()
	if len(m["text"]) != 3 {
		t.Errorf("expected 3 text models, got %d", len(m["text"]))
	}
	if len(m["image"]) != 3 {
		t.Errorf("expected 3 image models, got %d", len(m["image"]))
	}
	if len(m["video"]) != 3 {
		t.Errorf("expected 3 video models, got %d", len(m["video"]))
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) != 4 {
		t.Errorf("expected 4 config fields, got %d", len(fields))
	}
	if fields[0].Key != "apiKey" {
		t.Errorf("first field key = %q, want apiKey", fields[0].Key)
	}
	if fields[0].EnvVar != "DASHSCOPE_API_KEY" {
		t.Errorf("first field envVar = %q, want DASHSCOPE_API_KEY", fields[0].EnvVar)
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "qwenvl" {
		t.Errorf("provider name = %q, want qwenvl", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("expected at least one config")
	}
	if p.Configs[0].EnvVars[0] != "DASHSCOPE_API_KEY" {
		t.Errorf("envVar = %q, want DASHSCOPE_API_KEY", p.Configs[0].EnvVars[0])
	}
}

func TestExecute_JSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"JSON response text."}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{APIKey: "sk-test", BaseURL: srv.URL, Model: "qwen-vl-max"})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe"}},
	}
	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "JSON response text." {
		t.Errorf("got %q, want %q", result.Value, "JSON response text.")
	}
}

func TestExecute_JSONEmptyChoices(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	eng := New(Config{APIKey: "sk-test", BaseURL: srv.URL, Model: "qwen-vl-max"})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestExecute_JSONEmptyContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  "}}]}`))
	}))
	defer srv.Close()

	eng := New(Config{APIKey: "sk-test", BaseURL: srv.URL, Model: "qwen-vl-max"})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestExecute_ExtraBody(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "sk-test", BaseURL: server.URL, Model: "qwen-vl-max"})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe"}},
		"2": {ClassType: "Options", Inputs: map[string]any{
			"extra_body": `{"temperature": 0.7, "top_p": 0.9}`,
		}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}

	if gotPayload["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", gotPayload["temperature"])
	}
	if gotPayload["top_p"] != 0.9 {
		t.Errorf("top_p = %v, want 0.9", gotPayload["top_p"])
	}
}

func TestExecute_Streaming(t *testing.T) {
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
			`{"choices":[{"delta":{"content":"这是"}}]}`,
			`{"choices":[{"delta":{"content":"一段"}}]}`,
			`{"choices":[{"delta":{"content":"视频描述。"}}]}`,
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
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "qwen-vl-max",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this video"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/v.mp4"}},
	}
	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "这是一段视频描述。" {
		t.Errorf("got %q, want %q", result.Value, "这是一段视频描述。")
	}
	if result.Kind != engine.OutputPlainText {
		t.Errorf("kind = %v, want PlainText", result.Kind)
	}
}
