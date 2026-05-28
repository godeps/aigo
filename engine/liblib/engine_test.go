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

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "liblib" {
		t.Errorf("expected provider name 'liblib', got %q", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("expected at least one provider config")
	}
	cfg := p.Configs[0]
	if cfg.Name != "liblib-image" {
		t.Errorf("expected config name 'liblib-image', got %q", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Error("expected non-nil engine")
	}
	if len(cfg.EnvVars) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(cfg.EnvVars))
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("expected at least one model info")
	}
	info := infos[0]
	if info.Provider != "liblib" {
		t.Errorf("expected provider 'liblib', got %q", info.Provider)
	}
	if info.Capability != "image" {
		t.Errorf("expected capability 'image', got %q", info.Capability)
	}
	if info.Name != "liblib-comfyui" {
		t.Errorf("expected name 'liblib-comfyui', got %q", info.Name)
	}
}

func TestExecute_EmptyGenerateUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"generateUuid": ""},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey: "ak",
		SecretKey: "sk",
		BaseURL:   srv.URL,
		Endpoint:  "/api/generate/webui/text2img/ultra",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for empty generateUuid")
	}
	if !strings.Contains(err.Error(), "empty generateUuid") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecute_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey: "ak",
		SecretKey: "sk",
		BaseURL:   srv.URL,
		Endpoint:  "/api/generate/webui/text2img/ultra",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestExecute_WithLoadImage(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"generateStatus": float64(5),
					"images":         []any{map[string]any{"imageUrl": "https://cdn.liblib.art/out.png"}},
				},
			})
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"generateUuid": "img-uuid"},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "ak",
		SecretKey:         "sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/webui/img2img/ultra",
		TemplateUUID:      TemplateImg2ImgUltra,
		WaitForCompletion: true,
		PollInterval:      10 * time.Millisecond,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "enhance this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/input.png"}},
	}

	result, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value != "https://cdn.liblib.art/out.png" {
		t.Errorf("expected output URL, got %q", result.Value)
	}
	// Verify sourceImage was included in the request body.
	if gotBody != nil {
		params, _ := gotBody["generateParams"].(map[string]any)
		if params != nil {
			if si, ok := params["sourceImage"].(string); !ok || si != "https://example.com/input.png" {
				t.Errorf("expected sourceImage in params, got %v", params["sourceImage"])
			}
		}
	}
}

func TestExecute_WithGraphOptions(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"generateUuid": "opts-uuid"},
		})
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey:         "ak",
		SecretKey:         "sk",
		BaseURL:           srv.URL,
		Endpoint:          "/api/generate/webui/text2img/ultra",
		WaitForCompletion: false,
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
		"2": {ClassType: "AigoOptions", Inputs: map[string]any{
			"width":           1024,
			"height":          768,
			"aspect_ratio":    "16:9",
			"img_count":       2,
			"duration":        5,
			"negative_prompt": "ugly",
			"model":           "custom-model",
		}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody == nil {
		t.Fatal("expected request body")
	}
	params, _ := gotBody["generateParams"].(map[string]any)
	if params == nil {
		t.Fatal("expected generateParams in body")
	}
	if params["aspectRatio"] != "16:9" {
		t.Errorf("expected aspectRatio '16:9', got %v", params["aspectRatio"])
	}
	if params["negativePrompt"] != "ugly" {
		t.Errorf("expected negativePrompt 'ugly', got %v", params["negativePrompt"])
	}
	if params["model"] != "custom-model" {
		t.Errorf("expected model 'custom-model', got %v", params["model"])
	}
}

func TestResume_MissingKeys(t *testing.T) {
	t.Parallel()

	eng := New(Config{})
	_, err := eng.Resume(context.Background(), "some-uuid")
	if err == nil {
		t.Fatal("expected error for missing keys")
	}
}

func TestPoll_SuccessNoOutputURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			// Status 5 (success) but no images or videos.
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateStatus": float64(5)},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "empty-result-uuid"},
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
		t.Fatal("expected error for success with no output URL")
	}
	if !strings.Contains(err.Error(), "no output URL") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPoll_APIErrorCode(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 500,
				"msg":  "internal error",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "err-uuid"},
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
		t.Fatal("expected error for poll API error code")
	}
	if !strings.Contains(err.Error(), "poll error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecute_ContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			// Always return "still running" so we rely on ctx cancel.
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateStatus": float64(1)},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"generateUuid": "cancel-uuid"},
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(ctx, g)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      Config
		wantBase string
		wantEP   string
	}{
		{
			name:     "all defaults",
			cfg:      Config{},
			wantBase: defaultBaseURL,
			wantEP:   "/api/generate/webui/text2img/ultra",
		},
		{
			name:     "custom base with trailing slash",
			cfg:      Config{BaseURL: "https://custom.api.com/"},
			wantBase: "https://custom.api.com",
			wantEP:   "/api/generate/webui/text2img/ultra",
		},
		{
			name:     "custom endpoint",
			cfg:      Config{Endpoint: "/api/generate/video/kling/text2video"},
			wantBase: defaultBaseURL,
			wantEP:   "/api/generate/video/kling/text2video",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			eng := New(tt.cfg)
			if eng.baseURL != tt.wantBase {
				t.Errorf("baseURL = %q, want %q", eng.baseURL, tt.wantBase)
			}
			if eng.endpoint != tt.wantEP {
				t.Errorf("endpoint = %q, want %q", eng.endpoint, tt.wantEP)
			}
		})
	}
}

func TestCapabilities_NoWait(t *testing.T) {
	t.Parallel()
	eng := New(Config{WaitForCompletion: false})
	cap := eng.Capabilities()
	if cap.SupportsPoll {
		t.Error("expected SupportsPoll=false when WaitForCompletion=false")
	}
	if !cap.SupportsSync {
		t.Error("expected SupportsSync=true when WaitForCompletion=false")
	}
}

func TestExecute_InvalidSubmitJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	eng := New(Config{
		AccessKey: "ak",
		SecretKey: "sk",
		BaseURL:   srv.URL,
		Endpoint:  "/api/generate/webui/text2img/ultra",
	})

	g := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	}

	_, err := eng.Execute(context.Background(), g)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "decode submit response") {
		t.Errorf("unexpected error: %v", err)
	}
}
