package newapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/workflow"
)

// testPNGData is a minimal valid 1x1 RGB PNG used across image-related tests.
var testPNGData = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

// ---------------------------------------------------------------------------
// wrapGraphErr (18.2% -> 100%)
// ---------------------------------------------------------------------------

func TestWrapGraphErr_AllBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      error
		want    error
		wantNil bool
		wantMsg string
	}{
		{"nil", nil, nil, true, ""},
		{"missing_prompt", graph.ErrMissingPrompt, ErrMissingPrompt, false, ""},
		{"missing_image_source", graph.ErrMissingImageSource, ErrMissingImageSource, false, ""},
		{"missing_audio_source", graph.ErrMissingAudioSource, ErrMissingAudioSource, false, ""},
		{"remote_media_disabled", graph.ErrRemoteMediaDisabled, ErrRemoteMediaDisabled, false, ""},
		{"passthrough", errors.New("some other error"), nil, false, "some other error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := wrapGraphErr(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Errorf("wrapGraphErr(%v) = %v, want nil", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("wrapGraphErr(%v) = nil, want non-nil", tc.in)
			}
			if tc.want != nil && !errors.Is(got, tc.want) {
				t.Errorf("wrapGraphErr(%v) = %v, want %v", tc.in, got, tc.want)
			}
			if tc.wantMsg != "" && got.Error() != tc.wantMsg {
				t.Errorf("got error message %q, want %q", got.Error(), tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Capabilities (73.3% -> 100%)
// ---------------------------------------------------------------------------

func TestCapabilities_VisionAndUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       MediaKind
		waitVideo  bool
		wantMedia  string
		wantSync   bool
		wantPoll   bool
	}{
		{"image", KindImage, false, "image", true, false},
		{"video_wait", KindVideo, true, "video", false, true},
		{"video_no_wait", KindVideo, false, "video", true, false},
		{"speech", KindSpeech, false, "audio", true, false},
		{"vision", KindVision, false, "text", true, false},
		{"default_empty", "", false, "image", true, false},
		{"default_unknown", MediaKind("xyz"), false, "image", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := &Engine{model: "test", kind: tc.kind, waitVideo: tc.waitVideo}
			cap := eng.Capabilities()
			if len(cap.MediaTypes) == 0 || cap.MediaTypes[0] != tc.wantMedia {
				t.Errorf("MediaTypes[0] = %v, want %q", cap.MediaTypes, tc.wantMedia)
			}
			if cap.SupportsSync != tc.wantSync {
				t.Errorf("SupportsSync = %v, want %v", cap.SupportsSync, tc.wantSync)
			}
			if cap.SupportsPoll != tc.wantPoll {
				t.Errorf("SupportsPoll = %v, want %v", cap.SupportsPoll, tc.wantPoll)
			}
			if len(cap.Models) != 1 || cap.Models[0] != "test" {
				t.Errorf("Models = %v, want [test]", cap.Models)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// defaultRouteForKind - KindVision branch (83.3% -> 100%)
// ---------------------------------------------------------------------------

func TestDefaultRouteForKind_Vision(t *testing.T) {
	t.Parallel()

	got := defaultRouteForKind(KindVision)
	if got != RouteChatCompletions {
		t.Errorf("defaultRouteForKind(KindVision) = %q, want %q", got, RouteChatCompletions)
	}
}

// ---------------------------------------------------------------------------
// runJimengVideo + pollJimeng (both 0%)
// ---------------------------------------------------------------------------

func TestRunJimengVideoFullFlow(t *testing.T) {
	t.Parallel()

	var pollCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("Action")
		switch {
		case r.Method == http.MethodPost && action == jimengSubmitAction:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "paint me a sunrise" {
				t.Errorf("prompt = %v", body["prompt"])
			}
			if body["req_key"] != "high_aes_general_v21L_ttp" {
				t.Errorf("req_key = %v", body["req_key"])
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":0,"message":"ok","data":{"task_id":"jt-123"}}`))

		case r.Method == http.MethodPost && action == jimengGetAction:
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount < 2 {
				w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
			} else {
				w.Write([]byte(`{"code":0,"message":"ok","data":{"video_url":"https://cdn.example.com/jimeng.mp4"}}`))
			}

		default:
			t.Errorf("unexpected %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "jimeng-v1",
		Route:             RouteJimengVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "paint me a sunrise"}},
		"2": {ClassType: "JimengOptions", Inputs: map[string]any{"req_key": "high_aes_general_v21L_ttp"}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/jimeng.mp4" {
		t.Errorf("got %q", out.Value)
	}
}

func TestRunJimengVideoNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","data":{"task_id":"jt-nowait"}}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "jimeng-v1",
		Route:             RouteJimengVideo,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "JimengOptions", Inputs: map[string]any{"jimeng_req_key": "some_key"}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "jt-nowait" {
		t.Errorf("got %q, want jt-nowait", out.Value)
	}
}

func TestRunJimengVideoMissingReqKey(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "jimeng-v1",
		Route:   RouteJimengVideo,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if !errors.Is(err, ErrMissingJimengReqKey) {
		t.Errorf("got %v, want ErrMissingJimengReqKey", err)
	}
}

func TestRunJimengVideoSubmitError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL,
		Model:   "jimeng-v1",
		Route:   RouteJimengVideo,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "JimengOptions", Inputs: map[string]any{"req_key": "k"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunJimengVideoWithBinaryData(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","data":{"task_id":"jt-bin"}}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "jimeng-v1",
		Route:             RouteJimengVideo,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "JimengOptions", Inputs: map[string]any{
			"req_key":            "k",
			"binary_data_base64": "AAEC",
		}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["binary_data_base64"] == nil {
		t.Error("expected binary_data_base64 in request body")
	}
}

// ---------------------------------------------------------------------------
// runOpenAIWhisper (0%)
// ---------------------------------------------------------------------------

func TestRunOpenAIWhisperTranscription(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("unexpected content-type: %s", ct)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("model = %q", r.FormValue("model"))
		}
		if r.FormValue("response_format") != "json" {
			t.Errorf("response_format = %q", r.FormValue("response_format"))
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("missing file: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		f.Close()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"Hello world"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "whisper-1",
		Route:   RouteOpenAITranscriptions,
		APIKey:  "sk-test",
	})

	audioB64 := base64.StdEncoding.EncodeToString([]byte{0xff, 0xf3, 0x90, 0x00})
	g := workflow.Graph{
		"1": {ClassType: "AudioInput", Inputs: map[string]any{
			"audio_b64": audioB64,
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != `{"text":"Hello world"}` {
		t.Errorf("got %q", out.Value)
	}
}

func TestRunOpenAIWhisperTranslation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/translations" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"Translated text"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "whisper-1",
		Route:   RouteOpenAITranslations,
		APIKey:  "sk-test",
	})

	audioB64 := base64.StdEncoding.EncodeToString([]byte{0x00, 0x01})
	g := workflow.Graph{
		"1": {ClassType: "AudioInput", Inputs: map[string]any{
			"audio_b64": audioB64,
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != `{"text":"Translated text"}` {
		t.Errorf("got %q", out.Value)
	}
}

func TestRunOpenAIWhisperTextFormat(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("  Hello world  "))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "whisper-1",
		Route:   RouteOpenAITranscriptions,
		APIKey:  "sk-test",
	})

	audioB64 := base64.StdEncoding.EncodeToString([]byte{0x00})
	g := workflow.Graph{
		"1": {ClassType: "AudioInput", Inputs: map[string]any{
			"audio_b64":       audioB64,
			"response_format": "text",
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "Hello world" {
		t.Errorf("got %q, want %q", out.Value, "Hello world")
	}
}

func TestRunOpenAIWhisperWithLanguageAndPrompt(t *testing.T) {
	t.Parallel()

	var gotModel, gotLang, gotPrompt, gotFormat string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		gotModel = r.FormValue("model")
		gotLang = r.FormValue("language")
		gotPrompt = r.FormValue("prompt")
		gotFormat = r.FormValue("response_format")
		w.Write([]byte(`{"text":"ok"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "whisper-1",
		Route:   RouteOpenAITranscriptions,
		APIKey:  "sk-test",
	})

	audioB64 := base64.StdEncoding.EncodeToString([]byte{0x00})
	g := workflow.Graph{
		"1": {ClassType: "AudioInput", Inputs: map[string]any{
			"audio_b64":       audioB64,
			"language":        "en",
			"whisper_prompt":  "meeting notes",
			"response_format": "srt",
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "whisper-1" {
		t.Errorf("model = %q", gotModel)
	}
	if gotLang != "en" {
		t.Errorf("language = %q", gotLang)
	}
	if gotPrompt != "meeting notes" {
		t.Errorf("prompt = %q", gotPrompt)
	}
	if gotFormat != "srt" {
		t.Errorf("response_format = %q", gotFormat)
	}
	// srt format returns trimmed text
	if out.Value != `{"text":"ok"}` {
		t.Errorf("got %q", out.Value)
	}
}

// ---------------------------------------------------------------------------
// runOpenAIVideoGenerations additional paths (66.7%)
// ---------------------------------------------------------------------------

func TestRunOpenAIVideoGenerationsNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/video/generations" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task_id":"vt-nowait"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL + "/v1",
		Model:             "video-model",
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "vt-nowait" {
		t.Errorf("got %q, want vt-nowait", out.Value)
	}
}

func TestRunOpenAIVideoGenerationsMissingTaskID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL + "/v1",
		Model:             "video-model",
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing task_id")
	}
	if !strings.Contains(err.Error(), "missing task_id") {
		t.Errorf("error = %q, want 'missing task_id'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// buildStandardVideoPayload additional options (64%)
// ---------------------------------------------------------------------------

func TestBuildStandardVideoPayload_AllOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&gotPayload)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"task_id":"t-opts"}`))
		} else {
			t.Errorf("unexpected %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL + "/v1",
		Model:             "video-model",
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "mountain scene"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.jpg"}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{
			"duration":        5.0,
			"width":           1280,
			"height":          720,
			"fps":             30,
			"seed":            42,
			"n":               2,
			"negative_prompt": "blurry",
		}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if gotPayload["model"] != "video-model" {
		t.Errorf("model = %v", gotPayload["model"])
	}
	if gotPayload["prompt"] != "mountain scene" {
		t.Errorf("prompt = %v", gotPayload["prompt"])
	}
	if gotPayload["image"] != "https://example.com/ref.jpg" {
		t.Errorf("image = %v", gotPayload["image"])
	}
	if v, ok := gotPayload["duration"].(float64); !ok || v != 5 {
		t.Errorf("duration = %v", gotPayload["duration"])
	}
	if v, ok := gotPayload["width"].(float64); !ok || v != 1280 {
		t.Errorf("width = %v", gotPayload["width"])
	}
	if v, ok := gotPayload["height"].(float64); !ok || v != 720 {
		t.Errorf("height = %v", gotPayload["height"])
	}
	if v, ok := gotPayload["fps"].(float64); !ok || v != 30 {
		t.Errorf("fps = %v", gotPayload["fps"])
	}
	if v, ok := gotPayload["seed"].(float64); !ok || v != 42 {
		t.Errorf("seed = %v", gotPayload["seed"])
	}
	if v, ok := gotPayload["n"].(float64); !ok || v != 2 {
		t.Errorf("n = %v", gotPayload["n"])
	}
	if meta, ok := gotPayload["metadata"].(map[string]any); !ok || meta["negative_prompt"] != "blurry" {
		t.Errorf("metadata = %v", gotPayload["metadata"])
	}
	if gotPayload["response_format"] != "url" {
		t.Errorf("response_format = %v", gotPayload["response_format"])
	}
}

// ---------------------------------------------------------------------------
// runOpenAIImageEdits with gpt-image-* model (60.5%)
// ---------------------------------------------------------------------------

func TestRunOpenAIImageEditsGPTImage(t *testing.T) {
	t.Parallel()

	var gotFields []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(testPNGData)
		case r.URL.Path == "/v1/images/edits":
			_ = r.ParseMultipartForm(10 << 20)
			for k := range r.MultipartForm.Value {
				gotFields = append(gotFields, k)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"b64_json":"AAEC"}]}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL + "/v1",
		Model:             "gpt-image-1",
		Route:             RouteOpenAIImagesEdits,
		APIKey:            "sk-test",
		Background:        "transparent",
		OutputFormat:      "webp",
		Moderation:        "low",
		OutputCompression: 80,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "add snow"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image_url": server.URL + "/image.png"}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "data:image/webp;base64,"
	if !strings.HasPrefix(out.Value, wantPrefix) {
		t.Errorf("got %q, want prefix %q", out.Value, wantPrefix)
	}

	hasField := func(name string) bool {
		for _, f := range gotFields {
			if f == name {
				return true
			}
		}
		return false
	}
	if !hasField("background") {
		t.Error("missing field background")
	}
	if !hasField("output_format") {
		t.Error("missing field output_format")
	}
	if !hasField("moderation") {
		t.Error("missing field moderation")
	}
	if !hasField("output_compression") {
		t.Error("missing field output_compression")
	}
	// gpt-image-* should NOT have response_format
	if hasField("response_format") {
		t.Error("gpt-image-* should not have response_format field")
	}
}

func TestRunOpenAIImageEditsWithNAndSize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/image.png":
			w.Write(testPNGData)
		case r.URL.Path == "/v1/images/edits":
			_ = r.ParseMultipartForm(10 << 20)
			if r.FormValue("n") != "3" {
				t.Errorf("n = %q, want 3", r.FormValue("n"))
			}
			if r.FormValue("size") != "1024x1024" {
				t.Errorf("size = %q, want 1024x1024", r.FormValue("size"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/edited.png"}]}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "dall-e-2",
		Route:   RouteOpenAIImagesEdits,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "edit"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image_url": server.URL + "/image.png"}},
		"3": {ClassType: "EmptyLatentImage", Inputs: map[string]any{
			"width":  1024,
			"height": 1024,
		}},
		"4": {ClassType: "ImageOptions", Inputs: map[string]any{"n": 3}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// runKlingVideo no-wait + missing task_id (66.7%)
// ---------------------------------------------------------------------------

func TestRunKlingVideoNoWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/kling/v1/videos/image2video" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task_id":"kt-nowait"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "kling-v1",
		Route:             RouteKlingImage2Video,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "fly"}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "kt-nowait" {
		t.Errorf("got %q", out.Value)
	}
}

func TestRunKlingVideoMissingTaskID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "kling-v1",
		Route:             RouteKlingText2Video,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing task_id")
	}
	if !strings.Contains(err.Error(), "missing task_id") {
		t.Errorf("error = %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// runOpenAISpeech additional paths (73.7%)
// ---------------------------------------------------------------------------

func TestRunOpenAISpeechMissingVoice(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "tts-1",
		Kind:    KindSpeech,
		APIKey:  "sk-test",
	})

	// Graph without AudioOptions (no voice)
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if !errors.Is(err, ErrMissingVoice) {
		t.Errorf("got %v, want ErrMissingVoice", err)
	}
}

func TestRunOpenAISpeechWithSpeed(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Write([]byte{0xff, 0xf3})
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "tts-1",
		Kind:    KindSpeech,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{
			"voice":           "nova",
			"response_format": "opus",
			"speed":           1.5,
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if gotPayload["speed"] == nil {
		t.Error("expected speed in payload")
	}
	if v, ok := gotPayload["speed"].(float64); !ok || v != 1.5 {
		t.Errorf("speed = %v, want 1.5", gotPayload["speed"])
	}
	if gotPayload["voice"] != "nova" {
		t.Errorf("voice = %v", gotPayload["voice"])
	}
	wantPrefix := "data:audio/opus;base64,"
	if !strings.HasPrefix(out.Value, wantPrefix) {
		t.Errorf("got %q, want prefix %q", out.Value, wantPrefix)
	}
}

// ---------------------------------------------------------------------------
// Resume additional routes (68.4%)
// ---------------------------------------------------------------------------

func TestResumeKlingText2Video(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/kling/v1/videos/text2video/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls < 2 {
			w.Write([]byte(`{"task_id":"kt1","status":"in_progress"}`))
		} else {
			w.Write([]byte(`{"task_id":"kt1","status":"completed","url":"https://cdn.example.com/kling.mp4"}`))
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "kling-v1",
		Route:             RouteKlingText2Video,
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	out, err := eng.Resume(context.Background(), "kt1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/kling.mp4" {
		t.Errorf("got %q", out.Value)
	}
}

func TestResumeKlingImage2Video(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/kling/v1/videos/image2video/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task_id":"ki1","status":"completed","url":"https://cdn.example.com/ki.mp4"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "kling-v1",
		Route:             RouteKlingImage2Video,
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	out, err := eng.Resume(context.Background(), "ki1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/ki.mp4" {
		t.Errorf("got %q", out.Value)
	}
}

func TestResumeSoraVideo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/videos/sora-r1/content":
			w.Header().Set("Content-Type", "video/mp4")
			w.Write([]byte{0xAA, 0xBB})
		case r.URL.Path == "/v1/videos/sora-r1":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-r1","status":"completed"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		Kind:              KindVideo,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	out, err := eng.Resume(context.Background(), "sora-r1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.Value, "data:video/mp4;base64,") {
		t.Errorf("got %q", out.Value)
	}
}

// ---------------------------------------------------------------------------
// runChatCompletions error paths (70.8%)
// ---------------------------------------------------------------------------

func TestRunChatCompletions_EmptyChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunChatCompletions_EmptyContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunChatCompletions_CustomMaxTokens(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{
			"text":       "test",
			"max_tokens": 8192,
		}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := gotPayload["max_tokens"].(float64); !ok || v != 8192 {
		t.Errorf("max_tokens = %v, want 8192", gotPayload["max_tokens"])
	}
}

func TestRunChatCompletions_DecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRunChatCompletions_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen3.6-plus",
		Route:   RouteChatCompletions,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

// ---------------------------------------------------------------------------
// runQwenImageGenerations additional options (70.8%)
// ---------------------------------------------------------------------------

func TestRunQwenImageGenerationsWithOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/qwen.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "qwen-max-vl",
		Route:   RouteQwenImagesGenerations,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a landscape"}},
		"2": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
		"3": {ClassType: "QwenOptions", Inputs: map[string]any{
			"negative_prompt": "blurry",
			"prompt_extend":   "true",
			"watermark":       "false",
		}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}

	params, ok := gotPayload["parameters"].(map[string]any)
	if !ok {
		t.Fatal("missing parameters")
	}
	if params["negative_prompt"] != "blurry" {
		t.Errorf("negative_prompt = %v", params["negative_prompt"])
	}
	if params["prompt_extend"] != true {
		t.Errorf("prompt_extend = %v", params["prompt_extend"])
	}
	if params["watermark"] != false {
		t.Errorf("watermark = %v", params["watermark"])
	}
}

// ---------------------------------------------------------------------------
// Sora additional paths: canceled status, content error (67-77%)
// ---------------------------------------------------------------------------

func TestRunSoraVideoStatusCanceled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-cancel","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/sora-cancel":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-cancel","status":"canceled"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for canceled task")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error = %q, want to contain 'failed'", err.Error())
	}
}

func TestFetchSoraContentHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-err","status":"queued"}`))
		case r.URL.Path == "/v1/videos/sora-err/content":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
		case r.URL.Path == "/v1/videos/sora-err":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-err","status":"completed"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for content fetch failure")
	}
}

func TestRunSoraVideoWithAllOptions(t *testing.T) {
	t.Parallel()

	var gotContentType string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/videos" {
			gotContentType = r.Header.Get("Content-Type")
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"sora-opts","status":"queued"}`))
		} else {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "sora",
		Route:             RouteSoraVideos,
		APIKey:            "sk-test",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "flying car"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/ref.jpg"}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{
			"duration": 10.0,
			"width":    1920,
			"height":   1080,
			"fps":      24,
			"seed":     99,
		}},
	}
	out, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "sora-opts" {
		t.Errorf("got %q", out.Value)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("content-type = %q, want multipart", gotContentType)
	}
	body := string(gotBody)
	for _, field := range []string{"model", "prompt", "image", "duration", "width", "height", "fps", "seed"} {
		if !strings.Contains(body, field) {
			t.Errorf("missing field %q in body", field)
		}
	}
}

// ---------------------------------------------------------------------------
// pollVideoGET error status (75%)
// ---------------------------------------------------------------------------

func TestPollVideoGET_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL:           server.URL,
		Model:             "test",
		APIKey:            "sk-test",
		WaitForCompletion: true,
		PollInterval:      2 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := eng.pollVideoGET(ctx, "sk-test", func(id string) string {
		return server.URL + "/v1/video/generations/" + id
	}, "test-id")
	if err == nil {
		t.Fatal("expected error for 403 status")
	}
}

// ---------------------------------------------------------------------------
// runOpenAIImageGenerations additional paths (68.2%)
// ---------------------------------------------------------------------------

func TestRunOpenAIImageGenerations_DallE3Style(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/img.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "dall-e-3",
		Kind:    KindImage,
		APIKey:  "sk-test",
		Quality: "hd",
		Style:   "vivid",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset"}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	// Non-gpt-image model should have response_format and style
	if gotPayload["response_format"] != "url" {
		t.Errorf("response_format = %v, want url", gotPayload["response_format"])
	}
	if gotPayload["style"] != "vivid" {
		t.Errorf("style = %v, want vivid", gotPayload["style"])
	}
	if gotPayload["quality"] != "hd" {
		t.Errorf("quality = %v, want hd", gotPayload["quality"])
	}
}

func TestRunOpenAIImageGenerations_CustomN(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/img.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "dall-e-3",
		Kind:    KindImage,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"n": 5}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := gotPayload["n"].(float64); !ok || v != 5 {
		t.Errorf("n = %v, want 5", gotPayload["n"])
	}
}

// ---------------------------------------------------------------------------
// runGeminiGenerateContent missing prompt path (77.8%)
// ---------------------------------------------------------------------------

func TestRunGeminiGenerateContent_MissingPrompt(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com",
		Model:   "gemini-pro",
		Route:   RouteGeminiGenerateContent,
		APIKey:  "sk-test",
	})

	g := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 512, "height": 512}},
	}
	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

// ---------------------------------------------------------------------------
// New() edge cases (only tests not already in provider_test.go)
// ---------------------------------------------------------------------------

func TestNewWithEnvBaseURL(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("NEWAPI_BASE_URL", "https://from-env.example.com/v1")
	eng := New(Config{Model: "test", APIKey: "key"})
	if eng.origin == "" {
		t.Error("expected origin from NEWAPI_BASE_URL env")
	}
}

func TestNewDisableRemoteMediaFetch(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		Model:                   "test",
		DisableRemoteMediaFetch: true,
	})
	if eng.allowRemoteMediaFetch {
		t.Error("expected allowRemoteMediaFetch = false")
	}
}
