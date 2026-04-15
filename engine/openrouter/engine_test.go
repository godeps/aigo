package openrouter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func graphWithPrompt(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"prompt": prompt}},
	}
}

func graphWithVoice(text, voice string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"prompt": text, "voice": voice}},
	}
}

func graphWithAudio(url string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"audio_url": url}},
	}
}

// --- Image tests ---

func TestRunImageGeneration(t *testing.T) {
	tests := []struct {
		name     string
		graph    workflow.Graph
		respBody string
		wantURL  string
		wantErr  bool
	}{
		{
			name:  "success with image_url block",
			graph: graphWithPrompt("a cat"),
			respBody: `{"choices":[{"message":{"content":[
				{"type":"text","text":"Here is an image"},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}
			]}}]}`,
			wantURL: "data:image/png;base64,abc123",
		},
		{
			name:     "empty choices",
			graph:    graphWithPrompt("a cat"),
			respBody: `{"choices":[]}`,
			wantErr:  true,
		},
		{
			name:     "api error",
			graph:    graphWithPrompt("a cat"),
			respBody: `{"error":{"code":"invalid_model","message":"model not found"}}`,
			wantErr:  true,
		},
		{
			name:    "missing prompt",
			graph:   workflow.Graph{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					json.NewDecoder(r.Body).Decode(&payload)
					if payload["model"] != ModelGPT5Image {
						t.Fatalf("expected model %s, got %v", ModelGPT5Image, payload["model"])
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Fatal("missing auth header")
					}
					// Verify modalities.
					mods, _ := payload["modalities"].([]any)
					if len(mods) != 2 {
						t.Fatalf("expected 2 modalities, got %d", len(mods))
					}
					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{
				httpClient: http.DefaultClient,
				model:      ModelGPT5Image,
			}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runImageGeneration(context.Background(), e, "test-key", ModelGPT5Image, tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantURL {
				t.Fatalf("expected %q, got %q", tt.wantURL, result)
			}
		})
	}
}

func TestRunImageGeneration_RequestFormat(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,x"}}]}}]}`))
	}))
	defer srv.Close()

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient}
	_, err := runImageGeneration(context.Background(), e, "key", ModelGPT5Image, graphWithPrompt("a dog"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify messages format.
	messages, _ := gotPayload["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]any)
	if msg["role"] != "user" {
		t.Fatalf("expected role user, got %v", msg["role"])
	}
}

// --- TTS tests ---

func TestRunTTS(t *testing.T) {
	tests := []struct {
		name     string
		graph    workflow.Graph
		respBody string
		wantErr  bool
	}{
		{
			name:     "success",
			graph:    graphWithVoice("Hello world", "alloy"),
			respBody: `{"choices":[{"message":{"audio":{"data":"dGVzdA==","id":"audio_123"}}}]}`,
		},
		{
			name:    "missing voice",
			graph:   graphWithPrompt("Hello world"),
			wantErr: true,
		},
		{
			name:     "no audio in response",
			graph:    graphWithVoice("Hello world", "alloy"),
			respBody: `{"choices":[{"message":{"content":"text only"}}]}`,
			wantErr:  true,
		},
		{
			name:     "api error",
			graph:    graphWithVoice("Hello", "alloy"),
			respBody: `{"error":{"code":"rate_limit","message":"too many requests"}}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{httpClient: http.DefaultClient}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runTTS(context.Background(), e, "key", ModelGPTAudio, tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == "" {
				t.Fatal("expected non-empty result")
			}
			// Verify data URI format.
			if result != "data:audio/wav;base64,dGVzdA==" {
				t.Fatalf("unexpected result: %s", result)
			}
		})
	}
}

func TestRunTTS_VoiceAndFormat(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"audio":{"data":"dGVzdA=="}}}]}`))
	}))
	defer srv.Close()

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt":          "Hi",
			"voice":           "nova",
			"response_format": "mp3",
		}},
	}

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient}
	_, err := runTTS(context.Background(), e, "key", ModelGPTAudio, graph)
	if err != nil {
		t.Fatal(err)
	}

	audio, _ := gotPayload["audio"].(map[string]any)
	if audio["voice"] != "nova" {
		t.Fatalf("expected voice nova, got %v", audio["voice"])
	}
	if audio["format"] != "mp3" {
		t.Fatalf("expected format mp3, got %v", audio["format"])
	}
}

// --- ASR tests ---

func TestRunASR(t *testing.T) {
	// Serve a tiny audio file for the ASR handler to fetch.
	audioData := []byte("fake-wav-data")
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(audioData)
	}))
	defer audioSrv.Close()

	tests := []struct {
		name     string
		graph    workflow.Graph
		respBody string
		wantText string
		wantErr  bool
	}{
		{
			name:     "success string content",
			graph:    graphWithAudio(audioSrv.URL + "/test.wav"),
			respBody: `{"choices":[{"message":{"content":"hello world"}}]}`,
			wantText: "hello world",
		},
		{
			name:  "success array content",
			graph: graphWithAudio(audioSrv.URL + "/test.wav"),
			respBody: `{"choices":[{"message":{"content":[
				{"type":"text","text":"transcribed text"}
			]}}]}`,
			wantText: "transcribed text",
		},
		{
			name:    "missing audio url",
			graph:   workflow.Graph{},
			wantErr: true,
		},
		{
			name:     "api error",
			graph:    graphWithAudio(audioSrv.URL + "/test.wav"),
			respBody: `{"error":{"code":"err","message":"fail"}}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify input_audio is present in request.
					var payload map[string]any
					json.NewDecoder(r.Body).Decode(&payload)
					messages, _ := payload["messages"].([]any)
					if len(messages) != 1 {
						t.Fatalf("expected 1 message, got %d", len(messages))
					}
					msg, _ := messages[0].(map[string]any)
					content, _ := msg["content"].([]any)
					if len(content) != 2 {
						t.Fatalf("expected 2 content blocks, got %d", len(content))
					}
					audioBlock, _ := content[0].(map[string]any)
					if audioBlock["type"] != "input_audio" {
						t.Fatalf("expected input_audio type, got %v", audioBlock["type"])
					}
					inputAudio, _ := audioBlock["input_audio"].(map[string]any)
					b64, _ := inputAudio["data"].(string)
					decoded, _ := base64.StdEncoding.DecodeString(b64)
					if string(decoded) != string(audioData) {
						t.Fatalf("decoded audio mismatch: got %q", string(decoded))
					}

					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{httpClient: http.DefaultClient}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runASR(context.Background(), e, "key", ModelGPTAudio, tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantText {
				t.Fatalf("expected %q, got %q", tt.wantText, result)
			}
		})
	}
}

// --- Engine Execute tests ---

func TestExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"https://img.example.com/result.png"}}]}}]}`))
	}))
	defer srv.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   ModelGPT5Image,
	})

	result, err := e.Execute(context.Background(), graphWithPrompt("a cat"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "https://img.example.com/result.png" {
		t.Fatalf("unexpected value: %s", result.Value)
	}
}

func TestExecute_MissingAPIKey(t *testing.T) {
	e := New(Config{Model: ModelGPT5Image, BaseURL: "https://example.com"})
	// Clear env to ensure no fallback.
	t.Setenv("OPENROUTER_API_KEY", "")
	_, err := e.Execute(context.Background(), graphWithPrompt("test"))
	if err == nil {
		t.Fatal("expected ErrMissingAPIKey")
	}
}

func TestExecute_UnsupportedModel(t *testing.T) {
	e := New(Config{APIKey: "key", Model: "unknown/model", BaseURL: "https://example.com"})
	_, err := e.Execute(context.Background(), graphWithPrompt("test"))
	if err == nil {
		t.Fatal("expected ErrUnsupportedModel")
	}
}

// --- ModelsByCapability ---

func TestModelsByCapability(t *testing.T) {
	caps := ModelsByCapability()

	if len(caps["image"]) == 0 {
		t.Fatal("expected image models")
	}
	if len(caps["tts"]) == 0 {
		t.Fatal("expected tts models")
	}
	if len(caps["asr"]) == 0 {
		t.Fatal("expected asr models")
	}

	// Verify known models are present.
	found := map[string]bool{}
	for _, m := range caps["image"] {
		found[m] = true
	}
	if !found[ModelGPT5Image] {
		t.Fatal("missing ModelGPT5Image in image cap")
	}
	if !found[ModelGeminiFlashImage] {
		t.Fatal("missing ModelGeminiFlashImage in image cap")
	}
}

// --- Capabilities ---

func TestCapabilities(t *testing.T) {
	e := New(Config{APIKey: "key", Model: ModelGPT5Image})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("expected image media type, got %v", cap.MediaTypes)
	}

	e2 := New(Config{APIKey: "key", Model: ModelGPTAudio})
	cap2 := e2.Capabilities()
	if len(cap2.MediaTypes) != 1 || cap2.MediaTypes[0] != "audio" {
		t.Fatalf("expected audio media type, got %v", cap2.MediaTypes)
	}
	if len(cap2.Voices) == 0 {
		t.Fatal("expected voices for audio model")
	}
}

// --- extractImageFromChat ---

func TestExtractImageFromChat(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "image_url block",
			body: `{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}}]}`,
			want: "data:image/png;base64,abc",
		},
		{
			name: "string content with URL",
			body: `{"choices":[{"message":{"content":"https://example.com/img.png"}}]}`,
			want: "https://example.com/img.png",
		},
		{
			name:    "no image in content",
			body:    `{"choices":[{"message":{"content":[{"type":"text","text":"no image"}]}}]}`,
			wantErr: true,
		},
		{
			name:    "api error",
			body:    `{"error":{"code":"err","message":"fail"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractImageFromChat([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
