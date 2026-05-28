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
	t.Parallel()
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
			t.Parallel()
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode: %v", err)
						return
					}
					if payload["model"] != ModelGPT5Image {
						t.Errorf("expected model %s, got %v", ModelGPT5Image, payload["model"])
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Error("missing auth header")
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					// Verify modalities.
					mods, _ := payload["modalities"].([]any)
					if len(mods) != 2 {
						t.Errorf("expected 2 modalities, got %d", len(mods))
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
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
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
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
	t.Parallel()
	// Serve a tiny audio file for the ASR handler to fetch.
	audioData := []byte("fake-wav-data")
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(audioData)
	}))
	t.Cleanup(audioSrv.Close)

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
			t.Parallel()
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify input_audio is present in request.
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode: %v", err)
						return
					}
					messages, _ := payload["messages"].([]any)
					if len(messages) != 1 {
						t.Errorf("expected 1 message, got %d", len(messages))
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					msg, _ := messages[0].(map[string]any)
					content, _ := msg["content"].([]any)
					if len(content) != 2 {
						t.Errorf("expected 2 content blocks, got %d", len(content))
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					audioBlock, _ := content[0].(map[string]any)
					if audioBlock["type"] != "input_audio" {
						t.Errorf("expected input_audio type, got %v", audioBlock["type"])
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					inputAudio, _ := audioBlock["input_audio"].(map[string]any)
					b64, _ := inputAudio["data"].(string)
					decoded, _ := base64.StdEncoding.DecodeString(b64)
					if string(decoded) != string(audioData) {
						t.Errorf("decoded audio mismatch: got %q", string(decoded))
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
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
	t.Parallel()
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
	t.Parallel()
	e := New(Config{APIKey: "key", Model: "unknown/model", BaseURL: "https://example.com"})
	_, err := e.Execute(context.Background(), graphWithPrompt("test"))
	if err == nil {
		t.Fatal("expected ErrUnsupportedModel")
	}
}

// --- ModelsByCapability ---

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// --- ConfigSchema ---

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) != 2 {
		t.Fatalf("expected 2 config fields, got %d", len(fields))
	}

	apiKeyField := fields[0]
	if apiKeyField.Key != "apiKey" {
		t.Fatalf("expected first field key 'apiKey', got %q", apiKeyField.Key)
	}
	if apiKeyField.Type != "secret" {
		t.Fatalf("expected type 'secret', got %q", apiKeyField.Type)
	}
	if !apiKeyField.Required {
		t.Fatal("expected apiKey field to be required")
	}
	if apiKeyField.EnvVar != "OPENROUTER_API_KEY" {
		t.Fatalf("expected EnvVar 'OPENROUTER_API_KEY', got %q", apiKeyField.EnvVar)
	}

	baseURLField := fields[1]
	if baseURLField.Key != "baseUrl" {
		t.Fatalf("expected second field key 'baseUrl', got %q", baseURLField.Key)
	}
	if baseURLField.Type != "url" {
		t.Fatalf("expected type 'url', got %q", baseURLField.Type)
	}
	if baseURLField.Required {
		t.Fatal("expected baseUrl field to be optional")
	}
	if baseURLField.Default != defaultBaseURL {
		t.Fatalf("expected default %q, got %q", defaultBaseURL, baseURLField.Default)
	}
}

// --- DefaultProvider ---

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "openrouter" {
		t.Fatalf("expected provider name 'openrouter', got %q", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(p.Configs))
	}
	cfg := p.Configs[0]
	if cfg.Name != "openrouter" {
		t.Fatalf("expected config name 'openrouter', got %q", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(cfg.EnvVars) != 1 || cfg.EnvVars[0] != "OPENROUTER_API_KEY" {
		t.Fatalf("expected EnvVars [OPENROUTER_API_KEY], got %v", cfg.EnvVars)
	}
}

// --- ModelInfos ---

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 6 {
		t.Fatalf("expected 6 model infos, got %d", len(infos))
	}
	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
		if info.Provider != "openrouter" {
			t.Fatalf("expected provider 'openrouter' for %s, got %q", info.Name, info.Provider)
		}
		if info.DisplayName["en"] == "" {
			t.Fatalf("missing English display name for %s", info.Name)
		}
		if info.DocURL == "" {
			t.Fatalf("missing DocURL for %s", info.Name)
		}
	}
	for _, want := range []string{ModelGPT5Image, ModelGPT5ImageMini, ModelGeminiFlashImage, ModelGemini3ProImage, ModelGPTAudio, ModelGPTAudioMini} {
		if !names[want] {
			t.Fatalf("missing model info for %s", want)
		}
	}
}

// --- formatFromURL ---

func TestFormatFromURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/audio.wav", "wav"},
		{"https://example.com/audio.mp3", "mp3"},
		{"https://example.com/audio.flac", "flac"},
		{"https://example.com/audio.ogg", "ogg"},
		{"https://example.com/audio.webm", "webm"},
		{"https://example.com/audio.m4a", "m4a"},
		{"https://example.com/audio.mp3?token=abc", "mp3"},
		{"https://example.com/audio.WAV", "wav"},
		{"https://example.com/audio", "wav"}, // default
		{"https://example.com/no-ext", "wav"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			got := formatFromURL(tt.url)
			if got != tt.want {
				t.Fatalf("formatFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --- formatFromMIME ---

func TestFormatFromMIME(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mime string
		want string
	}{
		{"audio/wav", "wav"},
		{"audio/mpeg", "mp3"},
		{"audio/mp3", "mp3"},
		{"audio/flac", "flac"},
		{"audio/ogg", "ogg"},
		{"audio/webm", "webm"},
		{"  Audio/WAV  ", "wav"},
		{"application/octet-stream", "wav"}, // default
		{"", "wav"},
	}
	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			t.Parallel()
			got := formatFromMIME(tt.mime)
			if got != tt.want {
				t.Fatalf("formatFromMIME(%q) = %q, want %q", tt.mime, got, tt.want)
			}
		})
	}
}

// --- audioMIME ---

func TestAudioMIME(t *testing.T) {
	t.Parallel()
	tests := []struct {
		format string
		want   string
	}{
		{"mp3", "audio/mpeg"},
		{"opus", "audio/opus"},
		{"aac", "audio/aac"},
		{"flac", "audio/flac"},
		{"wav", "audio/wav"},
		{"pcm", "audio/pcm"},
		{"  WAV  ", "audio/wav"},
		{"unknown", "audio/wav"}, // default
		{"", "audio/wav"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got := audioMIME(tt.format)
			if got != tt.want {
				t.Fatalf("audioMIME(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

// --- fetchAudioBase64 ---

func TestFetchAudioBase64_DataURI(t *testing.T) {
	t.Parallel()
	b64, format, err := fetchAudioBase64(context.Background(), http.DefaultClient, "data:audio/mp3;base64,dGVzdA==")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b64 != "dGVzdA==" {
		t.Fatalf("expected b64 'dGVzdA==', got %q", b64)
	}
	if format != "mp3" {
		t.Fatalf("expected format 'mp3', got %q", format)
	}
}

func TestFetchAudioBase64_DataURI_Invalid(t *testing.T) {
	t.Parallel()
	_, _, err := fetchAudioBase64(context.Background(), http.DefaultClient, "data:invalid-no-comma")
	if err == nil {
		t.Fatal("expected error for invalid data URI")
	}
}

func TestFetchAudioBase64_HTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mp3data"))
	}))
	defer srv.Close()

	b64, format, err := fetchAudioBase64(context.Background(), http.DefaultClient, srv.URL+"/audio.mp3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(b64)
	if string(decoded) != "mp3data" {
		t.Fatalf("expected decoded 'mp3data', got %q", string(decoded))
	}
	if format != "mp3" {
		t.Fatalf("expected format 'mp3', got %q", format)
	}
}

func TestFetchAudioBase64_HTTP_UnknownContentType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	// formatFromMIME defaults to "wav" for unrecognized MIME types.
	_, format, err := fetchAudioBase64(context.Background(), http.DefaultClient, srv.URL+"/audio.flac")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "wav" {
		t.Fatalf("expected format 'wav' (default), got %q", format)
	}
}

func TestFetchAudioBase64_HTTP_Error(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	_, _, err := fetchAudioBase64(context.Background(), http.DefaultClient, srv.URL+"/missing.wav")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// --- Execute ASR auto-detect ---

func TestExecute_ASRAutoDetect(t *testing.T) {
	t.Parallel()
	// Serve audio file.
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("audio"))
	}))
	defer audioSrv.Close()

	// Serve API response.
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"transcribed"}}]}`))
	}))
	defer apiSrv.Close()

	// Use a TTS model but provide audio_url => should auto-detect ASR.
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url": audioSrv.URL + "/test.wav",
		}},
	}

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   ModelGPTAudio,
	})

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "transcribed" {
		t.Fatalf("expected 'transcribed', got %q", result.Value)
	}
}

func TestExecute_InvalidGraph(t *testing.T) {
	t.Parallel()
	e := New(Config{APIKey: "key", Model: ModelGPT5Image, BaseURL: "https://example.com"})
	// An invalid graph: node references a non-existent input.
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "test"}},
		"2": {ClassType: "Something", Inputs: map[string]any{"input": []any{"99", 0}}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- runASR with prompt-as-URL fallback ---

func TestRunASR_PromptAsURL(t *testing.T) {
	t.Parallel()
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("wav-data"))
	}))
	defer audioSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"from prompt url"}}]}`))
	}))
	defer apiSrv.Close()

	// Graph with prompt containing a URL, no audio_url.
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt": audioSrv.URL + "/test.wav",
		}},
	}

	e := &Engine{baseURL: apiSrv.URL, httpClient: http.DefaultClient}
	result, err := runASR(context.Background(), e, "key", ModelGPTAudio, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "from prompt url" {
		t.Fatalf("expected 'from prompt url', got %q", result)
	}
}

func TestRunASR_WithLanguage(t *testing.T) {
	t.Parallel()
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("wav-data"))
	}))
	defer audioSrv.Close()

	var gotPayload map[string]any
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"bonjour"}}]}`))
	}))
	defer apiSrv.Close()

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url": audioSrv.URL + "/test.wav",
			"language":  "French",
		}},
	}

	e := &Engine{baseURL: apiSrv.URL, httpClient: http.DefaultClient}
	result, err := runASR(context.Background(), e, "key", ModelGPTAudio, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "bonjour" {
		t.Fatalf("expected 'bonjour', got %q", result)
	}

	// Verify the instruction includes the language.
	messages, _ := gotPayload["messages"].([]any)
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].([]any)
	textBlock, _ := content[1].(map[string]any)
	instruction, _ := textBlock["text"].(string)
	if instruction != "Transcribe this audio in French." {
		t.Fatalf("expected language instruction, got %q", instruction)
	}
}

func TestRunASR_DataURI(t *testing.T) {
	t.Parallel()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer apiSrv.Close()

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url": "data:audio/wav;base64,dGVzdA==",
		}},
	}

	e := &Engine{baseURL: apiSrv.URL, httpClient: http.DefaultClient}
	result, err := runASR(context.Background(), e, "key", ModelGPTAudio, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

// --- Capabilities ASR model ---

func TestCapabilities_ASRModel(t *testing.T) {
	t.Parallel()
	// Use an unknown model that is only in asrTable — not possible with current
	// constants, but we can test the "else if" branch by using a model that
	// is in modelTable with TTS cap. The modelTable branch already handles it.
	// Instead, test with a model string not in modelTable at all.
	e := New(Config{APIKey: "key", Model: "custom/unknown"})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 0 {
		t.Fatalf("expected no media types for unknown model, got %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync to be true")
	}
}

// --- extractTextFromChat edge cases ---

func TestExtractTextFromChat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "string content",
			body: `{"choices":[{"message":{"content":"hello world"}}]}`,
			want: "hello world",
		},
		{
			name: "array content",
			body: `{"choices":[{"message":{"content":[{"type":"text","text":"transcribed"}]}}]}`,
			want: "transcribed",
		},
		{
			name:    "empty choices",
			body:    `{"choices":[]}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `{not json}`,
			wantErr: true,
		},
		{
			name:    "api error in response",
			body:    `{"error":{"code":"err","message":"bad request"}}`,
			wantErr: true,
		},
		{
			name:    "empty string content",
			body:    `{"choices":[{"message":{"content":""}}]}`,
			wantErr: true,
		},
		{
			name:    "array with no text blocks",
			body:    `{"choices":[{"message":{"content":[{"type":"audio","data":"x"}]}}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractTextFromChat([]byte(tt.body))
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

// --- extractAudioFromChat edge cases ---

func TestExtractAudioFromChat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		format  string
		want    string
		wantErr bool
	}{
		{
			name:   "success wav",
			body:   `{"choices":[{"message":{"audio":{"data":"abc123"}}}]}`,
			format: "wav",
			want:   "data:audio/wav;base64,abc123",
		},
		{
			name:   "success mp3",
			body:   `{"choices":[{"message":{"audio":{"data":"xyz"}}}]}`,
			format: "mp3",
			want:   "data:audio/mpeg;base64,xyz",
		},
		{
			name:    "invalid json",
			body:    `{bad`,
			format:  "wav",
			wantErr: true,
		},
		{
			name:    "no choices",
			body:    `{"choices":[]}`,
			format:  "wav",
			wantErr: true,
		},
		{
			name:    "null audio",
			body:    `{"choices":[{"message":{}}]}`,
			format:  "wav",
			wantErr: true,
		},
		{
			name:    "empty audio data",
			body:    `{"choices":[{"message":{"audio":{"data":""}}}]}`,
			format:  "wav",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractAudioFromChat([]byte(tt.body), tt.format)
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

// --- Image generation with reference image ---

func TestRunImageGeneration_WithReferenceImage(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,result"}}]}}]}`))
	}))
	defer srv.Close()

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt":    "edit this image",
			"image_url": "https://example.com/input.png",
			"size":      "1024x1024",
		}},
	}

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient}
	result, err := runImageGeneration(context.Background(), e, "key", ModelGPT5Image, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "data:image/png;base64,result" {
		t.Fatalf("unexpected result: %q", result)
	}

	// Verify multipart message format with image_url.
	messages, _ := gotPayload["messages"].([]any)
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	// Verify size was included.
	if gotPayload["size"] != "1024x1024" {
		t.Fatalf("expected size '1024x1024', got %v", gotPayload["size"])
	}
}

// --- New constructor ---

func TestNew_Defaults(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("expected default baseURL %q, got %q", defaultBaseURL, e.baseURL)
	}
	if e.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	t.Parallel()
	e := New(Config{BaseURL: "  https://custom.api.com/  "})
	if e.baseURL != "https://custom.api.com" {
		t.Fatalf("expected trimmed baseURL, got %q", e.baseURL)
	}
}

func TestNew_TrailingSlash(t *testing.T) {
	t.Parallel()
	e := New(Config{BaseURL: "https://custom.api.com/"})
	if e.baseURL != "https://custom.api.com" {
		t.Fatalf("expected trailing slash removed, got %q", e.baseURL)
	}
}

func TestExtractImageFromChat(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
