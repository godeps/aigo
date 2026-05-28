package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestGeminiExtractOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantPrefix string
		wantErr    bool
	}{
		{
			"inline_data",
			`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"AAEC"}}]}}]}`,
			"data:image/png;base64,AAEC",
			false,
		},
		{
			"inline_data_snake_mime",
			`{"candidates":[{"content":{"parts":[{"inlineData":{"mime_type":"image/jpeg","data":"XXYY"}}]}}]}`,
			"data:image/jpeg;base64,XXYY",
			false,
		},
		{
			"text_output",
			`{"candidates":[{"content":{"parts":[{"text":"hello world"}]}}]}`,
			"hello world",
			false,
		},
		{
			"file_uri",
			`{"candidates":[{"content":{"parts":[{"fileData":{"fileUri":"https://storage.example.com/file.png"}}]}}]}`,
			"https://storage.example.com/file.png",
			false,
		},
		{
			"empty_response",
			`{}`,
			"",
			true,
		},
		{
			"invalid_json",
			`not json`,
			"",
			true,
		},
		{
			"no_content",
			`{"candidates":[{"content":{"parts":[]}}]}`,
			"",
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := geminiExtractOutput([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantPrefix {
				t.Errorf("got %q, want %q", got, tc.wantPrefix)
			}
		})
	}
}

func TestDeepGeminiInlineData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		wantB64 string
	}{
		{
			"direct",
			map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "abc"}},
			"abc",
		},
		{
			"nested_in_array",
			[]any{map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "def"}}},
			"def",
		},
		{
			"no_data",
			map[string]any{"inlineData": map[string]any{"mimeType": "image/png"}},
			"",
		},
		{
			"nil",
			nil,
			"",
		},
		{
			"string_not_map",
			"just a string",
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deepGeminiInlineData(tc.input)
			if got.b64 != tc.wantB64 {
				t.Errorf("b64 = %q, want %q", got.b64, tc.wantB64)
			}
		})
	}
}

func TestDeepGeminiURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			"url_key",
			map[string]any{"fileUri": "https://example.com/file.png"},
			"https://example.com/file.png",
		},
		{
			"nested",
			map[string]any{"data": map[string]any{"url": "https://example.com/img.png"}},
			"https://example.com/img.png",
		},
		{
			"in_array",
			[]any{map[string]any{"url": "https://example.com/a.png"}},
			"https://example.com/a.png",
		},
		{
			"non_http",
			map[string]any{"url": "ftp://example.com"},
			"",
		},
		{
			"nil",
			nil,
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deepGeminiURL(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeepGeminiText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			"direct",
			map[string]any{"text": "hello"},
			"hello",
		},
		{
			"nested",
			map[string]any{"parts": []any{map[string]any{"text": "world"}}},
			"world",
		},
		{
			"empty_text",
			map[string]any{"text": "   "},
			"",
		},
		{
			"nil",
			nil,
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := deepGeminiText(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunGeminiGenerateContent(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"AAEC"}}]}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL,
		Model:   "gemini-2.0-flash",
		Route:   RouteGeminiGenerateContent,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	want := "data:image/png;base64,AAEC"
	if out.Value != want {
		t.Errorf("got %q, want %q", out.Value, want)
	}

	// verify payload structure
	contents, ok := gotPayload["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatal("missing contents in payload")
	}
}

func TestRunGeminiGenerateContentTextResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"generated text"}]}}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL,
		Model:   "gemini-2.0-flash",
		Route:   RouteGeminiGenerateContent,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "describe this"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "generated text" {
		t.Errorf("got %q, want %q", out.Value, "generated text")
	}
}
