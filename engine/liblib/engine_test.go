package liblib

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestExecute_Success(t *testing.T) {
	t.Parallel()
	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify signature params are present.
		q := r.URL.Query()
		if q.Get("AccessKey") == "" || q.Get("Signature") == "" || q.Get("Timestamp") == "" || q.Get("SignatureNonce") == "" {
			t.Error("missing signature query params")
			http.Error(w, "missing auth", 401)
			return
		}

		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/text2img/ultra"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "test-uuid-123"},
			})
		case strings.HasSuffix(path, "/status"):
			n := pollCount.Add(1)
			if n < 2 {
				json.NewEncoder(w).Encode(map[string]any{
					"code": 0,
					"data": map[string]any{"generateStatus": float64(1)},
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"code": 0,
					"data": map[string]any{
						"generateStatus": float64(5),
						"images": []any{
							map[string]any{"imageUrl": "https://cdn.liblib.art/result.png"},
						},
					},
				})
			}
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "test-ak",
		SecretKey:         "test-sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/webui/text2img/ultra",
		TemplateUUID:      TemplateText2ImgUltra,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value != "https://cdn.liblib.art/result.png" {
		t.Errorf("expected result URL, got %q", result.Value)
	}
}

func TestExecute_NoWait(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"generateUuid": "uuid-no-wait"},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "ak",
		SecretKey:         "sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/webui/text2img/ultra",
		TemplateUUID:      TemplateText2ImgUltra,
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value != "uuid-no-wait" {
		t.Errorf("expected generateUuid, got %q", result.Value)
	}
}

func TestExecute_PollFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/status") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateStatus": float64(6)},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "fail-uuid"},
			})
		}
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "ak",
		SecretKey:         "sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/webui/text2img/ultra",
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for failed generation")
	}
	if !strings.Contains(err.Error(), "generation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecute_VideoResult(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/status") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"generateStatus": float64(5),
					"videos": []any{
						map[string]any{"videoUrl": "https://cdn.liblib.art/video.mp4"},
					},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "vid-uuid"},
			})
		}
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "ak",
		SecretKey:         "sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/video/kling/text2video",
		TemplateUUID:      TemplateKlingText2Vid,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a flying bird"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value != "https://cdn.liblib.art/video.mp4" {
		t.Errorf("expected video URL, got %q", result.Value)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"generateStatus": float64(5),
				"images": []any{
					map[string]any{"imageUrl": "https://cdn.liblib.art/resumed.png"},
				},
			},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:    "ak",
		SecretKey:    "sk",
		BaseURL:      srv.URL,
		PollInterval: 10 * time.Millisecond,
	})

	result, err := eng.Resume(context.Background(), "some-uuid")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result.Value != "https://cdn.liblib.art/resumed.png" {
		t.Errorf("expected resumed URL, got %q", result.Value)
	}
}

func TestExecute_MissingKeys(t *testing.T) {
	t.Parallel()

	eng := New(Config{})
	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing keys")
	}
}

func TestExecute_MissingPrompt(t *testing.T) {
	t.Parallel()

	eng := New(Config{AccessKey: "ak", SecretKey: "sk"})
	g := workflow.Graph{
		"1": {ClassType: "Something", Inputs: map[string]any{}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestExecute_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 1001,
			"msg":  "insufficient credits",
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:    "ak",
		SecretKey:    "sk",
		BaseURL:      srv.URL,
		Endpoint:     "/api/generate/webui/text2img/ultra",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "insufficient credits") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSignURL(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		AccessKey: "myak",
		SecretKey: "mysk",
		BaseURL:   "https://openapi.liblibai.cloud",
	})

	url := eng.signURL("myak", "mysk", "/api/generate/webui/text2img/ultra")
	if !strings.HasPrefix(url, "https://openapi.liblibai.cloud/api/generate/webui/text2img/ultra?") {
		t.Errorf("unexpected URL prefix: %s", url)
	}
	if !strings.Contains(url, "AccessKey=myak") {
		t.Error("URL missing AccessKey")
	}
	if !strings.Contains(url, "Signature=") {
		t.Error("URL missing Signature")
	}
	if !strings.Contains(url, "Timestamp=") {
		t.Error("URL missing Timestamp")
	}
	if !strings.Contains(url, "SignatureNonce=") {
		t.Error("URL missing SignatureNonce")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) != 5 {
		t.Errorf("expected 5 config fields, got %d", len(fields))
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	eng := New(Config{WaitForCompletion: true})
	cap := eng.Capabilities()
	if len(cap.MediaTypes) != 2 {
		t.Errorf("expected 2 media types, got %d", len(cap.MediaTypes))
	}
	if !cap.SupportsPoll {
		t.Error("expected SupportsPoll=true")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	if len(m["image"]) == 0 {
		t.Error("expected image models")
	}
	if len(m["video"]) == 0 {
		t.Error("expected video models")
	}
}
