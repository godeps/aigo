package audiogen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

func TestIsTTSModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"qwen-tts", true},
		{"qwen3-tts", true},
		{"qwen3-tts-v2", true},
		{"QWEN-TTS", true},
		{"Qwen-Tts-Flash", true},
		{"  qwen-tts  ", true},
		{"cosyvoice-v1", false},
		{"gpt-4o", false},
		{"", false},
		{"qwen-asr", false},
		{"tts-only", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			got := IsTTSModel(tt.model)
			if got != tt.want {
				t.Fatalf("IsTTSModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestPreviewAudioMIME(t *testing.T) {
	t.Parallel()
	tests := []struct {
		format string
		want   string
	}{
		{"mp3", "audio/mpeg"},
		{"MP3", "audio/mpeg"},
		{"opus", "audio/opus"},
		{"pcm", "audio/pcm"},
		{"wav", "audio/wav"},
		{"", "audio/wav"},
		{"unknown", "audio/wav"},
		{"  mp3  ", "audio/mpeg"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got := previewAudioMIME(tt.format)
			if got != tt.want {
				t.Fatalf("previewAudioMIME(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func ttsGraph(text, voice string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": text}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": voice}},
	}
}

func ttsGraphWithOptions(text, voice string, opts map[string]any) workflow.Graph {
	audioInputs := map[string]any{"voice": voice}
	for k, v := range opts {
		audioInputs[k] = v
	}
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": text}},
		"2": {ClassType: "AudioOptions", Inputs: audioInputs},
	}
}

func TestRunTTS_Success(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.URL.Path != "/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth header")
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/audio.mp3"}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunTTS(context.Background(), rt, "test-key", "qwen3-tts", ttsGraph("hello world", "Cherry"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://example.com/audio.mp3" {
		t.Fatalf("result = %q, want URL", result)
	}

	if gotPayload["model"] != "qwen3-tts" {
		t.Fatalf("model = %v, want qwen3-tts", gotPayload["model"])
	}
	input, _ := gotPayload["input"].(map[string]any)
	if input["text"] != "hello world" {
		t.Fatalf("input.text = %v, want %q", input["text"], "hello world")
	}
	if input["voice"] != "Cherry" {
		t.Fatalf("input.voice = %v, want Cherry", input["voice"])
	}
}

func TestRunTTS_Base64Fallback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"data":"AQIDBA=="}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", ttsGraph("test", "Ethan"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "data:audio/wav;base64,AQIDBA==" {
		t.Fatalf("result = %q, want data URI", result)
	}
}

func TestRunTTS_MissingPrompt(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "Cherry"}},
	}
	_, err := RunTTS(context.Background(), nil, "", "", g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestRunTTS_MissingVoice(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}
	_, err := RunTTS(context.Background(), nil, "", "", g)
	if err != ierr.ErrMissingVoice {
		t.Fatalf("expected ErrMissingVoice, got %v", err)
	}
}

func TestRunTTS_EmptyVoice(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "  "}},
	}
	_, err := RunTTS(context.Background(), nil, "", "", g)
	if err != ierr.ErrMissingVoice {
		t.Fatalf("expected ErrMissingVoice, got %v", err)
	}
}

func TestRunTTS_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"code":"BadRequest","message":"invalid"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", ttsGraph("test", "Cherry"))
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
}

func TestRunTTS_InvalidVoiceHint(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"code":"InvalidParameter","message":"voice is not valid"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", ttsGraph("test", "BadVoice"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "supported voices") {
		t.Fatalf("error %q should contain supported voices hint", err.Error())
	}
}

func TestRunTTS_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":"InternalError","message":"something went wrong"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", ttsGraph("test", "Cherry"))
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "InternalError") {
		t.Fatalf("error %q should contain error code", err.Error())
	}
}

func TestRunTTS_EmptyAudioResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", ttsGraph("test", "Cherry"))
	if err == nil {
		t.Fatal("expected error for empty audio response")
	}
	if !strings.Contains(err.Error(), "did not contain audio") {
		t.Fatalf("error = %q, want 'did not contain audio'", err.Error())
	}
}

func TestRunTTS_WithLanguageType(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/audio.mp3"}}}`))
	}))
	defer srv.Close()

	g := ttsGraphWithOptions("hello", "Cherry", map[string]any{"language_type": "zh"})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["language_type"] != "zh" {
		t.Fatalf("input.language_type = %v, want zh", input["language_type"])
	}
}

func TestRunTTS_WithInstructions(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/audio.mp3"}}}`))
	}))
	defer srv.Close()

	g := ttsGraphWithOptions("hello", "Cherry", map[string]any{"instructions": "speak slowly"})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params, _ := gotPayload["parameters"].(map[string]any)
	if params == nil {
		t.Fatal("expected parameters in request")
	}
	if params["instructions"] != "speak slowly" {
		t.Fatalf("parameters.instructions = %v, want %q", params["instructions"], "speak slowly")
	}
}

func TestRunTTS_WithOptimizeInstructions(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/audio.mp3"}}}`))
	}))
	defer srv.Close()

	g := ttsGraphWithOptions("hello", "Cherry", map[string]any{"optimize_instructions": true})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunTTS(context.Background(), rt, "key", "qwen3-tts", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params, _ := gotPayload["parameters"].(map[string]any)
	if params == nil {
		t.Fatal("expected parameters in request")
	}
	if params["optimize_instructions"] != true {
		t.Fatalf("parameters.optimize_instructions = %v, want true", params["optimize_instructions"])
	}
}
