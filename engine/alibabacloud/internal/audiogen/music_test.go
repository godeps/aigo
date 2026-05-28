package audiogen

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

func musicGraph(opts map[string]any) workflow.Graph {
	return workflow.Graph{
		"opt": {ClassType: "Options", Inputs: opts},
	}
}

func musicPromptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func TestRunMusic_PromptOnly(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth header")
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.URL.Path != "/services/audio/music/generation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/song.mp3"}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunMusic(context.Background(), rt, "test-key", "fun-music-v1", musicPromptGraph("a happy pop song"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://example.com/song.mp3" {
		t.Fatalf("result = %q, want URL", result)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["prompt"] != "a happy pop song" {
		t.Fatalf("input.prompt = %v, want %q", input["prompt"], "a happy pop song")
	}
	if gotPayload["model"] != "fun-music-v1" {
		t.Fatalf("model = %v, want fun-music-v1", gotPayload["model"])
	}
}

func TestRunMusic_LyricsOverPrompt(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/song.mp3"}}}`))
	}))
	defer srv.Close()

	g := workflow.Graph{
		"1":   {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "style hint"}},
		"opt": {ClassType: "Options", Inputs: map[string]any{"lyrics": "la la la"}},
	}
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunMusic(context.Background(), rt, "key", "fun-music-v1", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["lyrics"] != "la la la" {
		t.Fatalf("input.lyrics = %v, want %q", input["lyrics"], "la la la")
	}
	if input["prompt"] != "style hint" {
		t.Fatalf("input.prompt = %v, want %q", input["prompt"], "style hint")
	}
}

func TestRunMusic_WithOptions(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/song.wav"}}}`))
	}))
	defer srv.Close()

	g := musicGraph(map[string]any{
		"prompt":               "rock music",
		"gender":               "male",
		"format":               "wav",
		"enable_aigc_watermark": true,
	})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunMusic(context.Background(), rt, "key", "fun-music-v1", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["gender"] != "male" {
		t.Fatalf("input.gender = %v, want male", input["gender"])
	}
	if input["format"] != "wav" {
		t.Fatalf("input.format = %v, want wav", input["format"])
	}
	if input["enable_aigc_watermark"] != true {
		t.Fatalf("input.enable_aigc_watermark = %v, want true", input["enable_aigc_watermark"])
	}
}

func TestRunMusic_DataURIFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"data":"AQID"}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunMusic(context.Background(), rt, "key", "fun-music-v1", musicPromptGraph("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "data:audio/mpeg;base64,AQID" {
		t.Fatalf("result = %q, want data URI", result)
	}
}

func TestRunMusic_MissingInput(t *testing.T) {
	g := workflow.Graph{"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{}}}
	_, err := RunMusic(context.Background(), nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunMusic_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":"InvalidParameter","message":"bad lyrics"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunMusic(context.Background(), rt, "key", "fun-music-v1", musicPromptGraph("test"))
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestRunMusic_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"code":"BadRequest","message":"invalid"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunMusic(context.Background(), rt, "key", "fun-music-v1", musicPromptGraph("test"))
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
}
