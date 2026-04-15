package volcvoice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func ttsGraph(text, voice string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt": text,
			"voice":  voice,
		}},
	}
}

func asrGraph(url string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"audio_url": url}},
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
			graph:    ttsGraph("Hello world", "BV001_streaming"),
			respBody: `{"code":3000,"message":"Success","data":"dGVzdA=="}`,
		},
		{
			name:    "missing text",
			graph:   workflow.Graph{},
			wantErr: true,
		},
		{
			name: "missing voice",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			wantErr: true,
		},
		{
			name:     "api error",
			graph:    ttsGraph("Hello", "BV001_streaming"),
			respBody: `{"code":4001,"message":"invalid appid"}`,
			wantErr:  true,
		},
		{
			name:     "no audio data",
			graph:    ttsGraph("Hello", "BV001_streaming"),
			respBody: `{"code":3000,"message":"Success","data":""}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/api/v1/tts" {
						t.Fatalf("expected /api/v1/tts, got %s", r.URL.Path)
					}
					if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer;") {
						t.Fatal("missing auth header")
					}
					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{
				httpClient: http.DefaultClient,
				model:      ModelTTSMega,
			}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runTTS(context.Background(), e, "app123", "token456", tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(result, "data:audio/") {
				t.Fatalf("expected data URI, got %q", result)
			}
		})
	}
}

func TestRunTTS_RequestFormat(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"code":3000,"message":"Success","data":"dGVzdA=="}`))
	}))
	defer srv.Close()

	e := &Engine{baseURL: srv.URL, httpClient: http.DefaultClient, model: ModelTTSMega}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt":   "Say hello",
			"voice":    "BV700_streaming",
			"encoding": "wav",
		}},
	}

	_, err := runTTS(context.Background(), e, "myapp", "mytoken", graph)
	if err != nil {
		t.Fatal(err)
	}

	app, _ := gotPayload["app"].(map[string]any)
	if app["appid"] != "myapp" {
		t.Fatalf("expected appid myapp, got %v", app["appid"])
	}
	if app["token"] != "mytoken" {
		t.Fatalf("expected token mytoken, got %v", app["token"])
	}

	audio, _ := gotPayload["audio"].(map[string]any)
	if audio["voice_type"] != "BV700_streaming" {
		t.Fatalf("expected voice BV700_streaming, got %v", audio["voice_type"])
	}
	if audio["encoding"] != "wav" {
		t.Fatalf("expected encoding wav, got %v", audio["encoding"])
	}

	req, _ := gotPayload["request"].(map[string]any)
	if req["text"] != "Say hello" {
		t.Fatalf("expected text 'Say hello', got %v", req["text"])
	}
}

// --- ASR tests ---

func TestRunASR(t *testing.T) {
	// Serve a tiny audio file.
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
			name:     "success",
			graph:    asrGraph(audioSrv.URL + "/test.wav"),
			respBody: `{"code":1000,"message":"Success","result":[{"text":"hello world"}]}`,
			wantText: "hello world",
		},
		{
			name:    "missing audio url",
			graph:   workflow.Graph{},
			wantErr: true,
		},
		{
			name:     "api error",
			graph:    asrGraph(audioSrv.URL + "/test.wav"),
			respBody: `{"code":4001,"message":"invalid token"}`,
			wantErr:  true,
		},
		{
			name:     "no text in result",
			graph:    asrGraph(audioSrv.URL + "/test.wav"),
			respBody: `{"code":1000,"message":"Success","result":[]}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respBody != "" {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/api/v1/asr" {
						t.Fatalf("expected /api/v1/asr, got %s", r.URL.Path)
					}
					w.WriteHeader(200)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			e := &Engine{httpClient: http.DefaultClient, model: ModelASR}
			if srv != nil {
				e.baseURL = srv.URL
			}

			result, err := runASR(context.Background(), e, "app123", "token456", tt.graph)
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

// --- Execute tests ---

func TestExecuteTTS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":3000,"message":"Success","data":"dGVzdA=="}`))
	}))
	defer srv.Close()

	e := New(Config{
		AppID:       "app123",
		AccessToken: "token456",
		BaseURL:     srv.URL,
		Model:       ModelTTSMega,
	})

	graph := ttsGraph("Hello", "BV001_streaming")
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result.Value, "data:audio/") {
		t.Fatalf("expected data URI, got %q", result.Value)
	}
}

func TestExecuteASR(t *testing.T) {
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("fake"))
	}))
	defer audioSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":1000,"message":"Success","result":[{"text":"transcribed text"}]}`))
	}))
	defer srv.Close()

	e := New(Config{
		AppID:       "app123",
		AccessToken: "token456",
		BaseURL:     srv.URL,
		Model:       ModelASR,
	})

	graph := asrGraph(audioSrv.URL + "/test.wav")
	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "transcribed text" {
		t.Fatalf("expected 'transcribed text', got %q", result.Value)
	}
}

func TestExecuteMissingAppID(t *testing.T) {
	e := New(Config{Model: ModelTTSMega, BaseURL: "https://example.com"})
	t.Setenv("VOLC_SPEECH_APPID", "")
	_, err := e.Execute(context.Background(), ttsGraph("test", "BV001_streaming"))
	if err == nil {
		t.Fatal("expected ErrMissingAppID")
	}
}

func TestExecuteUnsupportedModel(t *testing.T) {
	e := New(Config{
		AppID:       "app",
		AccessToken: "token",
		Model:       "unknown_model",
		BaseURL:     "https://example.com",
	})
	_, err := e.Execute(context.Background(), ttsGraph("test", "BV001_streaming"))
	if err == nil {
		t.Fatal("expected unsupported model error")
	}
}

func TestModelsByCapability(t *testing.T) {
	caps := ModelsByCapability()
	if len(caps["tts"]) == 0 {
		t.Fatal("expected tts models")
	}
	if len(caps["asr"]) == 0 {
		t.Fatal("expected asr models")
	}
}

func TestCapabilities(t *testing.T) {
	e := New(Config{AppID: "app", AccessToken: "token", Model: ModelTTSMega})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "audio" {
		t.Fatalf("expected audio media type, got %v", cap.MediaTypes)
	}
	if len(cap.Voices) == 0 {
		t.Fatal("expected voices for TTS model")
	}

	e2 := New(Config{AppID: "app", AccessToken: "token", Model: ModelASR})
	cap2 := e2.Capabilities()
	if len(cap2.MediaTypes) != 1 || cap2.MediaTypes[0] != "audio" {
		t.Fatalf("expected audio media type, got %v", cap2.MediaTypes)
	}
}
