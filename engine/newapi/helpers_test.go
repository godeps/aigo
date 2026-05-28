package newapi

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter", "abc", 5, "abc"},
		{"equal", "abcde", 5, "abcde"},
		{"longer", "abcdefgh", 5, "abcde..."},
		{"empty", "", 5, ""},
		{"zero_n", "abc", 0, "..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tc.s, tc.n)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestImageMIMEFromOutputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format string
		want   string
	}{
		{"png", "image/png"},
		{"PNG", "image/png"},
		{"jpeg", "image/jpeg"},
		{"jpg", "image/jpeg"},
		{"webp", "image/webp"},
		{"", "image/png"},
		{"unknown", "image/png"},
		{"  png  ", "image/png"},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			t.Parallel()
			got := imageMIMEFromOutputFormat(tc.format)
			if got != tc.want {
				t.Errorf("imageMIMEFromOutputFormat(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

func TestSpeechMIME(t *testing.T) {
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
		{"", "audio/mpeg"},
		{"unknown", "audio/mpeg"},
		{"MP3", "audio/mpeg"},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			t.Parallel()
			got := speechMIME(tc.format)
			if got != tc.want {
				t.Errorf("speechMIME(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

func TestDefaultRouteForKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind MediaKind
		want Route
	}{
		{KindImage, RouteOpenAIImagesGenerations},
		{KindVideo, RouteOpenAIVideoGenerations},
		{KindSpeech, RouteOpenAISpeech},
		{"", RouteOpenAIImagesGenerations},
		{"unknown", RouteOpenAIImagesGenerations},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			got := defaultRouteForKind(tc.kind)
			if got != tc.want {
				t.Errorf("defaultRouteForKind(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestJimengURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    string
		action  string
		version string
		wantSub string // substring that must appear
	}{
		{"basic", "https://api.example.com/jimeng/", "CVSync2AsyncSubmitTask", "2022-08-31", "Action=CVSync2AsyncSubmitTask"},
		{"with_version", "https://api.example.com/jimeng/", "CVSync2AsyncGetResult", "2022-08-31", "Version=2022-08-31"},
		{"invalid_url_fallback", "://bad", "Act", "V1", "Action=Act"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := jimengURL(tc.base, tc.action, tc.version)
			if got == "" {
				t.Fatal("jimengURL returned empty")
			}
			if !containsStr(got, tc.wantSub) {
				t.Errorf("jimengURL(%q,%q,%q) = %q, want substring %q", tc.base, tc.action, tc.version, got, tc.wantSub)
			}
		})
	}
}

func TestJimengParseTaskID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			"valid",
			`{"code":0,"message":"ok","data":{"task_id":"t123"}}`,
			"t123", false,
		},
		{
			"nested_taskid",
			`{"code":0,"message":"ok","data":{"result":{"task_id":"t456"}}}`,
			"t456", false,
		},
		{
			"missing_taskid",
			`{"code":0,"message":"ok","data":{}}`,
			"", true,
		},
		{
			"non_zero_code",
			`{"code":1001,"message":"bad request","data":null}`,
			"", true,
		},
		{
			"invalid_json",
			`not json`,
			"", true,
		},
		{
			"null_data",
			`{"code":0,"message":"ok","data":null}`,
			"", true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := jimengParseTaskID([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJimengParseResultURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		wantURL  string
		wantDone bool
		wantErr  bool
	}{
		{
			"valid_with_url",
			`{"code":0,"message":"ok","data":{"video_url":"https://cdn.example.com/v.mp4"}}`,
			"https://cdn.example.com/v.mp4", true, false,
		},
		{
			"code_zero_no_url",
			`{"code":0,"message":"ok","data":{}}`,
			"", false, false,
		},
		{
			"non_zero_continue",
			`{"code":429,"message":"too many requests","data":null}`,
			"", false, false,
		},
		{
			"non_zero_fatal",
			`{"code":401,"message":"unauthorized","data":null}`,
			"", true, true,
		},
		{
			"invalid_json",
			`not json`,
			"", false, true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			url, done, err := jimengParseResultURL([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if done != tc.wantDone {
				t.Errorf("done = %v, want %v", done, tc.wantDone)
			}
			if url != tc.wantURL {
				t.Errorf("url = %q, want %q", url, tc.wantURL)
			}
		})
	}
}

func TestDeepFindTaskID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"flat_task_id", map[string]any{"task_id": "abc"}, "abc"},
		{"flat_TaskId", map[string]any{"TaskId": "def"}, "def"},
		{"nested", map[string]any{"result": map[string]any{"task_id": "ghi"}}, "ghi"},
		{"in_array", map[string]any{"items": []any{map[string]any{"task_id": "jkl"}}}, "jkl"},
		{"nil_map", nil, ""},
		{"empty_map", map[string]any{}, ""},
		{"empty_string_value", map[string]any{"task_id": ""}, ""},
		{"whitespace_value", map[string]any{"task_id": "  "}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deepFindTaskID(tc.m)
			if got != tc.want {
				t.Errorf("deepFindTaskID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeepFindHTTPURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"url_key", map[string]any{"video_url": "https://cdn.example.com/v.mp4"}, "https://cdn.example.com/v.mp4"},
		{"mp4_suffix", map[string]any{"result": "https://cdn.example.com/v.mp4"}, "https://cdn.example.com/v.mp4"},
		{"nested", map[string]any{"data": map[string]any{"url": "https://cdn.example.com/v.mp4"}}, "https://cdn.example.com/v.mp4"},
		{"in_array", map[string]any{"items": []any{map[string]any{"url": "https://cdn.example.com/v.mp4"}}}, "https://cdn.example.com/v.mp4"},
		{"nil_map", nil, ""},
		{"empty_map", map[string]any{}, ""},
		{"non_http", map[string]any{"url": "ftp://example.com"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deepFindHTTPURL(tc.m)
			if got != tc.want {
				t.Errorf("deepFindHTTPURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJsonBody(t *testing.T) {
	t.Parallel()

	input := map[string]any{"key": "value", "num": float64(42)}
	data, err := jsonBody(input)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if decoded["key"] != "value" {
		t.Errorf("key = %v, want value", decoded["key"])
	}
}

func TestDecodeOpenAIImageData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		b64MIME string
		want    string
		wantErr bool
	}{
		{
			"url_response",
			`{"data":[{"url":"https://cdn.example.com/img.png"}]}`,
			"",
			"https://cdn.example.com/img.png",
			false,
		},
		{
			"b64_response_default_mime",
			`{"data":[{"b64_json":"AAEC"}]}`,
			"",
			"data:image/png;base64,AAEC",
			false,
		},
		{
			"b64_response_custom_mime",
			`{"data":[{"b64_json":"AAEC"}]}`,
			"image/jpeg",
			"data:image/jpeg;base64,AAEC",
			false,
		},
		{
			"empty_data",
			`{"data":[]}`,
			"",
			"",
			true,
		},
		{
			"no_url_or_b64",
			`{"data":[{}]}`,
			"",
			"",
			true,
		},
		{
			"invalid_json",
			`not json`,
			"",
			"",
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeOpenAIImageData([]byte(tc.body), tc.b64MIME)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsGPTImageModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{"gpt-image-2", true},
		{"gpt-image-1", true},
		{"gpt-image-1-mini", true},
		{"dall-e-3", false},
		{"", false},
		{" gpt-image-2", true}, // TrimSpace
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isGPTImageModel(tc.name)
			if got != tc.want {
				t.Errorf("isGPTImageModel(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestImageSizesForModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model string
		want  int // expected length, 0 means nil
	}{
		{"gpt-image-2", 3},
		{"dall-e-3", 3},
		{"dall-e-2", 3},
		{"unknown", 0},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()
			got := imageSizesForModel(tc.model)
			if len(got) != tc.want {
				t.Errorf("imageSizesForModel(%q) len = %d, want %d", tc.model, len(got), tc.want)
			}
		})
	}
}

func TestWrapGraphErr(t *testing.T) {
	t.Parallel()

	if wrapGraphErr(nil) != nil {
		t.Error("wrapGraphErr(nil) should be nil")
	}
}

func TestEffectiveRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		route Route
		kind  MediaKind
		want  Route
	}{
		{"explicit_route", RouteGeminiGenerateContent, KindImage, RouteGeminiGenerateContent},
		{"auto_image", RouteAuto, KindImage, RouteOpenAIImagesGenerations},
		{"empty_video", "", KindVideo, RouteOpenAIVideoGenerations},
		{"auto_speech", RouteAuto, KindSpeech, RouteOpenAISpeech},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := &Engine{route: tc.route, kind: tc.kind}
			got := eng.effectiveRoute()
			if got != tc.want {
				t.Errorf("effectiveRoute() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDoRequest_ErrorStatus(t *testing.T) {
	t.Parallel()

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	})
	defer srv.Close()

	eng := New(Config{BaseURL: srv.URL, Model: "test", APIKey: "key"})
	_, err := eng.doRequest(t.Context(), http.MethodPost, srv.URL+"/test", "key", []byte("{}"), "application/json")
	if err == nil {
		t.Fatal("expected error for 400 status")
	}
}

func TestDoRequest_Success(t *testing.T) {
	t.Parallel()

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("ct = %q", r.Header.Get("Content-Type"))
		}
		w.Write(body) // echo back
	})
	defer srv.Close()

	eng := New(Config{BaseURL: srv.URL, Model: "test", APIKey: "testkey"})
	got, err := eng.doRequest(t.Context(), http.MethodPost, srv.URL+"/echo", "testkey", []byte(`{"ok":true}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("got %q", string(got))
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
