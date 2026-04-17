package audiogen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

func graphWithAudioURL(url string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"audio_url": url}},
	}
}

func graphWithLanguage(url, lang string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"audio_url": url, "language": lang}},
	}
}

func emptyGraph() workflow.Graph {
	return workflow.Graph{}
}

func TestRunQwenASR(t *testing.T) {
	tests := []struct {
		name       string
		graph      workflow.Graph
		respCode   int
		respBody   string
		wantText   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "success",
			graph:    graphWithAudioURL("https://example.com/audio.wav"),
			respCode: 200,
			respBody: `{"choices":[{"message":{"content":"hello world"}}]}`,
			wantText: "hello world",
		},
		{
			name:     "success with language",
			graph:    graphWithLanguage("https://example.com/audio.wav", "zh"),
			respCode: 200,
			respBody: `{"choices":[{"message":{"content":"你好世界"}}]}`,
			wantText: "你好世界",
		},
		{
			name:       "missing audio url",
			graph:      emptyGraph(),
			wantErr:    true,
			wantErrMsg: ierr.ErrMissingAudioURL.Error(),
		},
		{
			name:     "api error in response",
			graph:    graphWithAudioURL("https://example.com/audio.wav"),
			respCode: 200,
			respBody: `{"error":{"code":"InvalidParameter","message":"bad request"}}`,
			wantErr:  true,
		},
		{
			name:     "http error",
			graph:    graphWithAudioURL("https://example.com/audio.wav"),
			respCode: 400,
			respBody: `{"error":{"code":"BadRequest","message":"invalid"}}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			if tt.respCode > 0 {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatalf("decode request: %v", err)
					}
					if payload["model"] != "qwen3-asr-flash" {
						t.Fatalf("expected model qwen3-asr-flash, got %v", payload["model"])
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Fatalf("missing auth header")
					}

					w.WriteHeader(tt.respCode)
					w.Write([]byte(tt.respBody))
				}))
				defer srv.Close()
			}

			rt := &runtime.RT{
				HTTPClient: http.DefaultClient,
			}
			if srv != nil {
				rt.BaseURL = srv.URL
			}

			result, err := RunQwenASR(context.Background(), rt, "test-key", "qwen3-asr-flash", tt.graph)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
					t.Fatalf("expected error %q, got %q", tt.wantErrMsg, err.Error())
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

func TestRunQwenASR_LanguageInRequest(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunQwenASR(context.Background(), rt, "key", "qwen3-asr-flash", graphWithLanguage("https://a.wav", "zh"))
	if err != nil {
		t.Fatal(err)
	}

	asrOpts, _ := gotPayload["asr_options"].(map[string]any)
	if asrOpts == nil {
		t.Fatal("expected asr_options in request")
	}
	if asrOpts["language"] != "zh" {
		t.Fatalf("expected language zh, got %v", asrOpts["language"])
	}
}

func TestRunQwenASR_RequestFormat(t *testing.T) {
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunQwenASR(context.Background(), rt, "key", "qwen3-asr-flash", graphWithAudioURL("https://example.com/audio.wav"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify OpenAI-compatible messages format.
	messages, _ := gotPayload["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]any)
	if msg["role"] != "user" {
		t.Fatalf("expected role user, got %v", msg["role"])
	}
	content, _ := msg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}
	item, _ := content[0].(map[string]any)
	if item["type"] != "input_audio" {
		t.Fatalf("expected type input_audio, got %v", item["type"])
	}
	inputAudio, _ := item["input_audio"].(map[string]any)
	if inputAudio["data"] != "https://example.com/audio.wav" {
		t.Fatalf("expected audio URL in data field, got %v", inputAudio["data"])
	}

	// Verify stream is false.
	if gotPayload["stream"] != false {
		t.Fatalf("expected stream false, got %v", gotPayload["stream"])
	}
}

func TestExtractChatCompletion(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "success",
			body: `{"choices":[{"message":{"content":"hello world"}}]}`,
			want: "hello world",
		},
		{
			name:    "empty choices",
			body:    `{"choices":[]}`,
			wantErr: true,
		},
		{
			name:    "api error",
			body:    `{"error":{"code":"Error","message":"fail"}}`,
			wantErr: true,
		},
		{
			name:    "empty content",
			body:    `{"choices":[{"message":{"content":""}}]}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractChatCompletion([]byte(tt.body))
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

func TestAudioURL(t *testing.T) {
	tests := []struct {
		name    string
		graph   workflow.Graph
		want    string
		wantErr bool
	}{
		{
			name:  "from audio_url",
			graph: graphWithAudioURL("https://example.com/a.wav"),
			want:  "https://example.com/a.wav",
		},
		{
			name: "from prompt URL",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "https://example.com/b.wav"}},
			},
			want: "https://example.com/b.wav",
		},
		{
			name: "prompt non-URL ignored",
			graph: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "transcribe this"}},
			},
			wantErr: true,
		},
		{
			name:    "empty graph",
			graph:   emptyGraph(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := audioURL(tt.graph)
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
