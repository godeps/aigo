package audiogen

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
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
						t.Errorf("decode request: %v", err)
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					if payload["model"] != "qwen3-asr-flash" {
						t.Errorf("expected model qwen3-asr-flash, got %v", payload["model"])
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
					}
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Errorf("missing auth header")
						http.Error(w, "test assertion failed", http.StatusInternalServerError)
						return
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

func TestRunQwenASR_404ReturnsNonRetryable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunQwenASR(context.Background(), rt, "key", "qwen3-asr-flash", graphWithAudioURL("https://a.wav"))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var ae *aigoerr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *aigoerr.Error", err)
	}
	if ae.Retryable {
		t.Error("404 error should be non-retryable")
	}
	if ae.Code != aigoerr.CodeInvalidInput {
		t.Errorf("Code = %v, want CodeInvalidInput", ae.Code)
	}
	if !strings.Contains(ae.Message, "qwen3-asr-flash") {
		t.Errorf("message should mention model name, got %q", ae.Message)
	}
}

func TestRunQwenASRFiletrans_DataURIRejected(t *testing.T) {
	t.Parallel()
	rt := &runtime.RT{BaseURL: "http://unused", HTTPClient: http.DefaultClient}
	graph := graphWithAudioURL("data:audio/wav;base64,UklGRi...")

	_, err := RunQwenASRFiletrans(context.Background(), rt, "key", "qwen3-asr-flash-filetrans", graph)
	if err == nil {
		t.Fatal("expected error for data URI input")
	}
	if !errors.Is(err, ierr.ErrDataURINotSupported) {
		t.Errorf("err = %v, want ErrDataURINotSupported in chain", err)
	}
}

func TestRunQwenASRFiletrans_HTTPURLSubmits(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		json.NewDecoder(r.Body).Decode(&gotPayload)
		// Return a task creation response.
		json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{"task_id": "asr-task-1"},
		})
	}))
	defer srv.Close()

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        srv.Client(),
		WaitForCompletion: false,
	}
	graph := graphWithAudioURL("https://oss.aliyun.com/test.wav")

	taskID, err := RunQwenASRFiletrans(context.Background(), rt, "test-key", "qwen3-asr-flash-filetrans", graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "asr-task-1" {
		t.Errorf("taskID = %q, want asr-task-1", taskID)
	}

	// Verify async header.
	if gotHeaders.Get("X-DashScope-Async") != "enable" {
		t.Error("missing X-DashScope-Async header")
	}
	// Verify model in payload.
	if gotPayload["model"] != "qwen3-asr-flash-filetrans" {
		t.Errorf("model = %v, want qwen3-asr-flash-filetrans", gotPayload["model"])
	}
	// Verify file_url in input.
	input, _ := gotPayload["input"].(map[string]any)
	if input["file_url"] != "https://oss.aliyun.com/test.wav" {
		t.Errorf("file_url = %v, want https://oss.aliyun.com/test.wav", input["file_url"])
	}
}

func TestRunQwenASRFiletrans_WaitForResult(t *testing.T) {
	t.Parallel()
	var attempt int
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/asr/transcription"):
			json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{"task_id": "asr-task-2"},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/tasks/"):
			attempt++
			if attempt < 2 {
				json.NewEncoder(w).Encode(map[string]any{
					"output": map[string]any{"task_status": "RUNNING"},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_status": "SUCCEEDED",
					"result": map[string]any{
						"transcription_url": srvURL + "/transcription.json",
					},
				},
			})
		case r.URL.Path == "/transcription.json":
			json.NewEncoder(w).Encode(map[string]any{
				"transcripts": []any{
					map[string]any{
						"channel_id": 0,
						"text":       "这是一段测试文本",
					},
				},
			})
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	rt := &runtime.RT{
		BaseURL:           srv.URL,
		HTTPClient:        srv.Client(),
		WaitForCompletion: true,
		PollInterval:      time.Millisecond,
	}
	graph := graphWithAudioURL("https://oss.aliyun.com/test.wav")

	result, err := RunQwenASRFiletrans(context.Background(), rt, "key", "qwen3-asr-flash-filetrans", graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "这是一段测试文本" {
		t.Errorf("result = %q, want 这是一段测试文本", result)
	}
}
