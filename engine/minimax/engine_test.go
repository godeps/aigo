package minimax

import (
	"context"
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

func graphWithLyrics(prompt, lyrics string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt": prompt,
			"lyrics": lyrics,
		}},
	}
}

// --- Music generation tests ---

func TestRunMusic(t *testing.T) {
	tests := []struct {
		name      string
		graph     workflow.Graph
		respBody  string
		wantAudio string
		wantErr   bool
	}{
		{
			name:      "success with URL",
			graph:     graphWithPrompt("indie folk, melancholy"),
			respBody:  `{"data":{"status":2,"audio":"https://cdn.minimax.com/audio/123.mp3"},"base_resp":{"status_code":0,"status_msg":"success"},"extra_info":{"music_duration":25364}}`,
			wantAudio: "https://cdn.minimax.com/audio/123.mp3",
		},
		{
			name:      "success with lyrics",
			graph:     graphWithLyrics("pop, upbeat", "[verse]\nHello world"),
			respBody:  `{"data":{"status":2,"audio":"https://cdn.minimax.com/audio/456.mp3"},"base_resp":{"status_code":0,"status_msg":"success"}}`,
			wantAudio: "https://cdn.minimax.com/audio/456.mp3",
		},
		{
			name:     "missing prompt",
			graph:    workflow.Graph{},
			wantErr:  true,
		},
		{
			name:     "API error response",
			graph:    graphWithPrompt("test"),
			respBody: `{"data":{},"base_resp":{"status_code":1001,"status_msg":"invalid parameter"}}`,
			wantErr:  true,
		},
		{
			name:     "empty audio in response",
			graph:    graphWithPrompt("test"),
			respBody: `{"data":{"status":2,"audio":""},"base_resp":{"status_code":0,"status_msg":"success"}}`,
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

			e := &Engine{
				httpClient: http.DefaultClient,
				model:      ModelMusic26,
			}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runMusic(context.Background(), e, "test-key", tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantAudio {
				t.Fatalf("expected %q, got %q", tt.wantAudio, result)
			}
		})
	}
}

func TestRunMusic_RequestFormat(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing or incorrect auth header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("missing content-type header")
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"status":2,"audio":"https://example.com/out.mp3"},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt":          "jazz, smooth",
			"lyrics":          "[verse]\nSmooth sailing",
			"is_instrumental": false,
			"output_format":   "url",
			"sample_rate":     44100,
			"format":          "mp3",
		}},
	}

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient, model: ModelMusic26}
	_, err := runMusic(context.Background(), e, "test-key", graph)
	if err != nil {
		t.Fatal(err)
	}

	if gotPayload["model"] != ModelMusic26 {
		t.Fatalf("expected model %s, got %v", ModelMusic26, gotPayload["model"])
	}
	if gotPayload["prompt"] != "jazz, smooth" {
		t.Fatalf("expected prompt 'jazz, smooth', got %v", gotPayload["prompt"])
	}
	if gotPayload["lyrics"] != "[verse]\nSmooth sailing" {
		t.Fatalf("expected lyrics, got %v", gotPayload["lyrics"])
	}
	if gotPayload["stream"] != false {
		t.Fatalf("expected stream=false, got %v", gotPayload["stream"])
	}

	audioSetting, ok := gotPayload["audio_setting"].(map[string]any)
	if !ok {
		t.Fatal("expected audio_setting in payload")
	}
	if audioSetting["format"] != "mp3" {
		t.Fatalf("expected format mp3, got %v", audioSetting["format"])
	}
}

// --- Engine Execute tests ---

func TestExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"status":2,"audio":"https://cdn.minimax.com/result.mp3"},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	e := New(Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   ModelMusic26,
	})

	result, err := e.Execute(context.Background(), graphWithPrompt("rock, energetic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "https://cdn.minimax.com/result.mp3" {
		t.Fatalf("unexpected value: %s", result.Value)
	}
}

func TestExecute_MissingAPIKey(t *testing.T) {
	e := New(Config{Model: ModelMusic26, BaseURL: "https://example.com"})
	t.Setenv("MINIMAX_API_KEY", "")
	_, err := e.Execute(context.Background(), graphWithPrompt("test"))
	if err == nil {
		t.Fatal("expected ErrMissingAPIKey")
	}
}

func TestExecute_UnsupportedModel(t *testing.T) {
	e := New(Config{APIKey: "key", Model: "unknown-model", BaseURL: "https://example.com"})
	_, err := e.Execute(context.Background(), graphWithPrompt("test"))
	if err == nil {
		t.Fatal("expected ErrUnsupportedModel")
	}
}

// --- Capabilities ---

func TestCapabilities(t *testing.T) {
	e := New(Config{APIKey: "key", Model: ModelMusic26})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "audio" {
		t.Fatalf("expected audio media type, got %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("expected SupportsSync=true")
	}
	if len(cap.Models) != 1 || cap.Models[0] != ModelMusic26 {
		t.Fatalf("expected model %s, got %v", ModelMusic26, cap.Models)
	}
}

// --- ModelsByCapability ---

func TestModelsByCapability(t *testing.T) {
	caps := ModelsByCapability()
	music := caps["music"]
	if len(music) != 4 {
		t.Fatalf("expected 4 music models, got %d", len(music))
	}
	found := map[string]bool{}
	for _, m := range music {
		found[m] = true
	}
	for _, want := range []string{ModelMusic26, ModelMusicCover, ModelMusic26Free, ModelMusicCoverFree} {
		if !found[want] {
			t.Fatalf("missing model %s", want)
		}
	}
}

// --- ConfigSchema ---

func TestConfigSchema(t *testing.T) {
	schema := ConfigSchema()
	if len(schema) != 2 {
		t.Fatalf("expected 2 config fields, got %d", len(schema))
	}
	if schema[0].Key != "apiKey" {
		t.Fatalf("expected first field apiKey, got %s", schema[0].Key)
	}
	if schema[0].EnvVar != "MINIMAX_API_KEY" {
		t.Fatalf("expected envVar MINIMAX_API_KEY, got %s", schema[0].EnvVar)
	}
}

// --- extractMusicResult ---

func TestExtractMusicResult(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "success",
			body: `{"data":{"status":2,"audio":"https://cdn.minimax.com/123.mp3"},"base_resp":{"status_code":0,"status_msg":"success"}}`,
			want: "https://cdn.minimax.com/123.mp3",
		},
		{
			name:    "API error",
			body:    `{"data":{},"base_resp":{"status_code":1001,"status_msg":"bad request"}}`,
			wantErr: true,
		},
		{
			name:    "empty audio",
			body:    `{"data":{"status":2,"audio":""},"base_resp":{"status_code":0,"status_msg":"success"}}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			body:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMusicResult([]byte(tt.body))
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
