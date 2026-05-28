package volcvoice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
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
	t.Parallel()
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
						t.Errorf("expected /api/v1/tts, got %s", r.URL.Path)
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer;") {
						t.Error("missing auth header")
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
	t.Parallel()
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
	t.Parallel()
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
						t.Errorf("expected /api/v1/asr, got %s", r.URL.Path)
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	caps := ModelsByCapability()
	if len(caps["tts"]) == 0 {
		t.Fatal("expected tts models")
	}
	if len(caps["asr"]) == 0 {
		t.Fatal("expected asr models")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
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

// --- ConfigSchema / ModelInfos / DefaultProvider ---

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) != 2 {
		t.Fatalf("expected 2 config fields, got %d", len(fields))
	}
	keys := map[string]bool{}
	for _, f := range fields {
		keys[f.Key] = true
		if f.Label == "" {
			t.Fatalf("field %q missing label", f.Key)
		}
		if !f.Required {
			t.Fatalf("field %q should be required", f.Key)
		}
	}
	if !keys["apiKey"] {
		t.Fatal("missing apiKey field")
	}
	if !keys["appId"] {
		t.Fatal("missing appId field")
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 5 {
		t.Fatalf("expected 5 model infos, got %d", len(infos))
	}
	names := map[string]bool{}
	for _, m := range infos {
		names[m.Name] = true
		if m.Provider != "volcvoice" {
			t.Fatalf("expected provider volcvoice, got %q", m.Provider)
		}
		if m.Capability == "" {
			t.Fatalf("model %q missing capability", m.Name)
		}
	}
	for _, want := range []string{ModelTTSMega, ModelTTSIcl, ModelTTSDefault, ModelASR, ModelASRLarge} {
		if !names[want] {
			t.Fatalf("missing model info for %q", want)
		}
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "volcvoice" {
		t.Fatalf("expected provider name volcvoice, got %q", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("expected at least one provider config")
	}
}

// --- ttsCluster ---

func TestTTSCluster(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model   string
		cluster string
		want    string
	}{
		{model: ModelTTSMega, want: "volcano_mega"},
		{model: ModelTTSIcl, want: "volcano_icl"},
		{model: ModelTTSDefault, want: clusterTTS},
		{model: ModelTTSMega, cluster: "custom_cluster", want: "custom_cluster"},
	}
	for _, tt := range tests {
		e := &Engine{model: tt.model, cluster: tt.cluster}
		got := e.ttsCluster()
		if got != tt.want {
			t.Errorf("ttsCluster(model=%q, cluster=%q) = %q, want %q", tt.model, tt.cluster, got, tt.want)
		}
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
		{"audio/x-wav", "wav"},
		{"audio/mp3", "mp3"},
		{"audio/mpeg", "mp3"},
		{"audio/pcm", "pcm"},
		{"audio/ogg", "ogg"},
		{"application/octet-stream", "wav"},
		{"", "wav"},
	}
	for _, tt := range tests {
		got := formatFromMIME(tt.mime)
		if got != tt.want {
			t.Errorf("formatFromMIME(%q) = %q, want %q", tt.mime, got, tt.want)
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
		{"https://example.com/audio.pcm", "pcm"},
		{"https://example.com/audio.ogg", "ogg"},
		{"https://example.com/audio.m4a", "m4a"},
		{"https://example.com/audio.mp3?token=abc", "mp3"},
		{"https://example.com/audio", "wav"},
	}
	for _, tt := range tests {
		got := formatFromURL(tt.url)
		if got != tt.want {
			t.Errorf("formatFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

// --- audioMIME ---

func TestAudioMIME(t *testing.T) {
	t.Parallel()
	tests := []struct {
		encoding string
		want     string
	}{
		{"mp3", "audio/mpeg"},
		{"ogg_opus", "audio/ogg"},
		{"ogg", "audio/ogg"},
		{"pcm", "audio/pcm"},
		{"wav", "audio/wav"},
		{"unknown", "audio/mpeg"},
		{"", "audio/mpeg"},
	}
	for _, tt := range tests {
		got := audioMIME(tt.encoding)
		if got != tt.want {
			t.Errorf("audioMIME(%q) = %q, want %q", tt.encoding, got, tt.want)
		}
	}
}

// --- extractTTSResult / extractASRResult edge cases ---

func TestExtractTTSResult_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := extractTTSResult([]byte(`not json`), "mp3")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractASRResult_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := extractASRResult([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractASRResult_WhitespaceText(t *testing.T) {
	t.Parallel()
	_, err := extractASRResult([]byte(`{"code":1000,"message":"Success","result":[{"text":"   "}]}`))
	if err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
}

// --- fetchAudioBase64 ---

func TestFetchAudioBase64_DataURI(t *testing.T) {
	t.Parallel()
	raw := base64.StdEncoding.EncodeToString([]byte("hello"))
	dataURI := "data:audio/wav;base64," + raw
	b64, format, err := fetchAudioBase64(context.Background(), http.DefaultClient, dataURI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b64 != raw {
		t.Fatalf("expected %q, got %q", raw, b64)
	}
	if format != "wav" {
		t.Fatalf("expected wav, got %q", format)
	}
}

func TestFetchAudioBase64_InvalidDataURI(t *testing.T) {
	t.Parallel()
	_, _, err := fetchAudioBase64(context.Background(), http.DefaultClient, "data:invalid-no-comma")
	if err == nil {
		t.Fatal("expected error for invalid data URI")
	}
}

func TestFetchAudioBase64_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _, err := fetchAudioBase64(context.Background(), http.DefaultClient, srv.URL+"/missing.wav")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestFetchAudioBase64_ContentTypeDetection(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/ogg")
		w.Write([]byte("audio-data"))
	}))
	defer srv.Close()

	b64, format, err := fetchAudioBase64(context.Background(), http.DefaultClient, srv.URL+"/test.wav")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "ogg" {
		t.Fatalf("expected ogg from Content-Type, got %q", format)
	}
	if b64 == "" {
		t.Fatal("expected non-empty base64 data")
	}
}

// --- doRequest HTTP error ---

func TestDoRequest_HTTPErrorStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	e := &Engine{httpClient: http.DefaultClient}
	_, err := e.doRequest(context.Background(), srv.URL+"/api/v1/tts", "token", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

// --- runTTS with optional parameters ---

func TestRunTTS_SpeedAndPitchRatio(t *testing.T) {
	t.Parallel()
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
			"prompt":      "Hello",
			"voice":       "BV001_streaming",
			"speed_ratio": "1.5",
			"pitch_ratio": "0.8",
		}},
	}

	_, err := runTTS(context.Background(), e, "app", "token", graph)
	if err != nil {
		t.Fatal(err)
	}

	audio, _ := gotPayload["audio"].(map[string]any)
	if sr, ok := audio["speed_ratio"].(float64); !ok || sr != 1.5 {
		t.Fatalf("expected speed_ratio 1.5, got %v", audio["speed_ratio"])
	}
	if pr, ok := audio["pitch_ratio"].(float64); !ok || pr != 0.8 {
		t.Fatalf("expected pitch_ratio 0.8, got %v", audio["pitch_ratio"])
	}
}

// --- runASR with prompt fallback and language ---

func TestRunASR_PromptFallback(t *testing.T) {
	t.Parallel()
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("fake-wav"))
	}))
	defer audioSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":1000,"message":"Success","result":[{"text":"ok"}]}`))
	}))
	defer srv.Close()

	e := &Engine{httpClient: http.DefaultClient, model: ModelASR, baseURL: srv.URL}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"prompt": audioSrv.URL + "/audio.wav",
		}},
	}

	result, err := runASR(context.Background(), e, "app", "token", graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestRunASR_DataURI(t *testing.T) {
	t.Parallel()
	raw := base64.StdEncoding.EncodeToString([]byte("fake-audio"))
	dataURI := "data:audio/mp3;base64," + raw

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":1000,"message":"Success","result":[{"text":"transcribed"}]}`))
	}))
	defer srv.Close()

	e := &Engine{httpClient: http.DefaultClient, model: ModelASR, baseURL: srv.URL}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url": dataURI,
		}},
	}

	result, err := runASR(context.Background(), e, "app", "token", graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "transcribed" {
		t.Fatalf("expected 'transcribed', got %q", result)
	}
}

func TestRunASR_WithLanguageAndSampleRate(t *testing.T) {
	t.Parallel()
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("fake"))
	}))
	defer audioSrv.Close()

	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"code":1000,"message":"Success","result":[{"text":"hello"}]}`))
	}))
	defer srv.Close()

	e := &Engine{httpClient: http.DefaultClient, model: ModelASR, baseURL: srv.URL}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url":   audioSrv.URL + "/test.wav",
			"language":    "zh",
			"sample_rate": 44100,
		}},
	}

	_, err := runASR(context.Background(), e, "app", "token", graph)
	if err != nil {
		t.Fatal(err)
	}

	reqMap, _ := gotPayload["request"].(map[string]any)
	if reqMap["language"] != "zh" {
		t.Fatalf("expected language zh, got %v", reqMap["language"])
	}
	audioMap, _ := gotPayload["audio"].(map[string]any)
	if rate, ok := audioMap["rate"].(float64); !ok || int(rate) != 44100 {
		t.Fatalf("expected rate 44100, got %v", audioMap["rate"])
	}
}

// --- Execute with different TTS models ---

func TestExecuteTTS_ICLModel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":3000,"message":"Success","data":"dGVzdA=="}`))
	}))
	defer srv.Close()

	e := New(Config{AppID: "app", AccessToken: "tok", BaseURL: srv.URL, Model: ModelTTSIcl})
	result, err := e.Execute(context.Background(), ttsGraph("Hello", "custom_voice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("expected OutputDataURI, got %v", result.Kind)
	}
}

func TestExecuteASR_LargeModel(t *testing.T) {
	t.Parallel()
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("fake"))
	}))
	defer audioSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":1000,"message":"Success","result":[{"text":"pro result"}]}`))
	}))
	defer srv.Close()

	e := New(Config{AppID: "app", AccessToken: "tok", BaseURL: srv.URL, Model: ModelASRLarge})
	result, err := e.Execute(context.Background(), asrGraph(audioSrv.URL+"/test.wav"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != engine.OutputPlainText {
		t.Fatalf("expected OutputPlainText, got %v", result.Kind)
	}
	if result.Value != "pro result" {
		t.Fatalf("expected 'pro result', got %q", result.Value)
	}
}

// --- New with env BaseURL ---

func TestNew_BaseURLFromEnv(t *testing.T) {
	t.Setenv("VOLC_SPEECH_BASE_URL", "https://custom.example.com/")
	e := New(Config{AppID: "app", AccessToken: "tok", Model: ModelTTSMega})
	if e.baseURL != "https://custom.example.com" {
		t.Fatalf("expected trimmed env base URL, got %q", e.baseURL)
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	t.Setenv("VOLC_SPEECH_BASE_URL", "")
	e := New(Config{AppID: "app", AccessToken: "tok", Model: ModelTTSMega})
	if e.baseURL != defaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", defaultBaseURL, e.baseURL)
	}
}

// --- Execute access token from env ---

func TestExecute_AccessTokenFromEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer;env-token" {
			t.Errorf("expected Bearer;env-token, got %q", auth)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"code":3000,"message":"Success","data":"dGVzdA=="}`))
	}))
	defer srv.Close()

	t.Setenv("VOLC_SPEECH_ACCESS_TOKEN", "env-token")
	e := New(Config{AppID: "app", BaseURL: srv.URL, Model: ModelTTSMega})
	_, err := e.Execute(context.Background(), ttsGraph("Hello", "BV001_streaming"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- reqID uniqueness ---

func TestReqID_Unique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := reqID()
		if seen[id] {
			t.Fatalf("duplicate reqID: %s", id)
		}
		seen[id] = true
		if len(id) != 32 {
			t.Fatalf("expected 32 hex chars, got %d: %s", len(id), id)
		}
	}
}
