package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestCompileExtractsPromptAndSize(t *testing.T) {
	t.Parallel()

	engine := New(Config{})
	graph := workflow.Graph{
		"1": {
			ClassType: "PromptSource",
			Inputs:    map[string]any{"value": "cinematic city skyline"},
		},
		"2": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": []any{"1", 0}},
		},
		"3": {
			ClassType: "EmptyLatentImage",
			Inputs:    map[string]any{"width": 1600, "height": 900},
		},
	}

	req, err := engine.Compile(graph)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if req.Prompt != "cinematic city skyline" {
		t.Fatalf("Compile().Prompt = %q, want %q", req.Prompt, "cinematic city skyline")
	}
	if req.Size != "1536x1024" {
		t.Fatalf("Compile().Size = %q, want %q", req.Size, "1536x1024")
	}
}

func TestExecuteCallsImagesAPI(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/images/generations")
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q", got)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/image.png"}]}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	graph := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": "an astronaut reading in a tea house"},
		},
		"2": {
			ClassType: "EmptyLatentImage",
			Inputs:    map[string]any{"width": 1024, "height": 1536},
		},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got.Value != "https://cdn.example.com/image.png" {
		t.Fatalf("Execute() = %q, want image URL", got.Value)
	}

	if gotPayload["prompt"] != "an astronaut reading in a tea house" {
		t.Fatalf("prompt = %#v", gotPayload["prompt"])
	}
	if gotPayload["size"] != "1024x1536" {
		t.Fatalf("size = %#v", gotPayload["size"])
	}
	if gotPayload["model"] != defaultModel {
		t.Fatalf("model = %#v, want %q", gotPayload["model"], defaultModel)
	}
}

func TestExecuteGPTImage2OmitsUnsupportedFieldsAndDecodesBase64(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-image-2",
		Quality: "high",
		Style:   "vivid", // must be silently dropped for gpt-image-*
	})

	graph := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs:    map[string]any{"text": "a quiet zen garden at dawn"},
		},
		"2": {
			ClassType: "EmptyLatentImage",
			Inputs:    map[string]any{"width": 1024, "height": 1024},
		},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, ok := gotPayload["response_format"]; ok {
		t.Errorf("payload must not include response_format for gpt-image-*: %#v", gotPayload)
	}
	if _, ok := gotPayload["style"]; ok {
		t.Errorf("payload must not include style for gpt-image-*: %#v", gotPayload)
	}
	if gotPayload["quality"] != "high" {
		t.Errorf("quality = %#v, want \"high\"", gotPayload["quality"])
	}
	if gotPayload["model"] != "gpt-image-2" {
		t.Errorf("model = %#v, want \"gpt-image-2\"", gotPayload["model"])
	}
	wantPrefix := "data:image/png;base64,"
	if got.Value[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Execute() = %q, want data URI", got.Value)
	}
}

func TestNewQualityDefaultsByModelFamily(t *testing.T) {
	t.Parallel()

	dalle := New(Config{Model: "dall-e-3"})
	if dalle.quality != "standard" {
		t.Errorf("dall-e-3 default quality = %q, want \"standard\"", dalle.quality)
	}

	gpt := New(Config{Model: "gpt-image-2"})
	if gpt.quality != "" {
		t.Errorf("gpt-image-2 default quality = %q, want \"\" (omit)", gpt.quality)
	}
}

func TestExecuteGPTImage2PassesNewParamsAndUsesOutputFormatMIME(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		Model:             "gpt-image-2",
		Background:        "transparent",
		OutputFormat:      "webp",
		Moderation:        "low",
		OutputCompression: 80,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a sky lantern"}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if gotPayload["background"] != "transparent" {
		t.Errorf("background = %#v", gotPayload["background"])
	}
	if gotPayload["output_format"] != "webp" {
		t.Errorf("output_format = %#v", gotPayload["output_format"])
	}
	if gotPayload["moderation"] != "low" {
		t.Errorf("moderation = %#v", gotPayload["moderation"])
	}
	if v, _ := gotPayload["output_compression"].(float64); v != 80 {
		t.Errorf("output_compression = %#v, want 80", gotPayload["output_compression"])
	}
	wantPrefix := "data:image/webp;base64,"
	if len(got.Value) < len(wantPrefix) || got.Value[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Execute() = %q, want %s prefix", got.Value, wantPrefix)
	}
}

func TestExecuteRoutesToEditsWhenImageSourcePresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	var hitGen, hitEdits bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/images/generations":
			hitGen = true
			t.Errorf("must not hit generations when image source present")
		case "/images/edits":
			hitEdits = true
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Errorf("Content-Type = %q, want multipart", ct)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-image-2",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "redraw with neon palette"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !hitEdits {
		t.Fatal("expected /images/edits to be called")
	}
	if hitGen {
		t.Fatal("expected /images/generations NOT to be called")
	}
	if !strings.HasPrefix(got.Value, "data:image/png;base64,") {
		t.Errorf("Execute() = %q, want data URI", got.Value)
	}
}

func TestHasImageSourceDetectsB64AndLoadImage(t *testing.T) {
	t.Parallel()

	g1 := workflow.Graph{
		"1": {ClassType: "ImageOptions", Inputs: map[string]any{"image_b64": "AAECAw=="}},
	}
	if !hasImageSource(g1) {
		t.Error("image_b64 should be detected")
	}
	g2 := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"image": "/tmp/x.png"}},
	}
	if !hasImageSource(g2) {
		t.Error("LoadImage{image} should be detected")
	}
	g3 := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}
	if hasImageSource(g3) {
		t.Error("text-only graph should NOT trigger edits")
	}
}

// ---------- Capabilities / ConfigSchema / ModelsByCapability / DefaultProvider ----------

func TestCapabilities_DallE(t *testing.T) {
	t.Parallel()
	eng := New(Config{Model: "dall-e-3"})
	cap := eng.Capabilities()
	if cap.MediaTypes[0] != "image" {
		t.Errorf("MediaTypes = %v, want [image]", cap.MediaTypes)
	}
	if cap.Models[0] != "dall-e-3" {
		t.Errorf("Models = %v, want [dall-e-3]", cap.Models)
	}
	wantSizes := []string{"1024x1024", "1024x1792", "1792x1024"}
	if len(cap.Sizes) != len(wantSizes) {
		t.Fatalf("Sizes = %v, want %v", cap.Sizes, wantSizes)
	}
	for i, s := range wantSizes {
		if cap.Sizes[i] != s {
			t.Errorf("Sizes[%d] = %q, want %q", i, cap.Sizes[i], s)
		}
	}
	if !cap.SupportsSync {
		t.Error("SupportsSync should be true")
	}
}

func TestCapabilities_GPTImage(t *testing.T) {
	t.Parallel()
	eng := New(Config{Model: "gpt-image-2"})
	cap := eng.Capabilities()
	if cap.Models[0] != "gpt-image-2" {
		t.Errorf("Models = %v", cap.Models)
	}
	wantSizes := []string{"1024x1024", "1024x1536", "1536x1024"}
	if len(cap.Sizes) != len(wantSizes) {
		t.Fatalf("Sizes = %v, want %v", cap.Sizes, wantSizes)
	}
	for i, s := range wantSizes {
		if cap.Sizes[i] != s {
			t.Errorf("Sizes[%d] = %q, want %q", i, cap.Sizes[i], s)
		}
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if fields[0].Key != "apiKey" || !fields[0].Required {
		t.Errorf("first field = %+v, want apiKey required", fields[0])
	}
	if fields[1].Key != "baseUrl" {
		t.Errorf("second field key = %q, want baseUrl", fields[1].Key)
	}
	want := map[string]bool{
		"model":              false,
		"quality":            false,
		"style":              false,
		"background":         false,
		"output_format":      false,
		"moderation":         false,
		"output_compression": false,
	}
	for _, f := range fields {
		if _, ok := want[f.Key]; ok {
			want[f.Key] = true
		}
	}
	for key, ok := range want {
		if !ok {
			t.Errorf("ConfigSchema missing %q", key)
		}
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	imgs, ok := m["image"]
	if !ok {
		t.Fatal("missing image capability")
	}
	if len(imgs) != 3 {
		t.Fatalf("image models = %v, want 3 models", imgs)
	}
	if imgs[0] != "gpt-image-2" {
		t.Errorf("first model = %q, want gpt-image-2", imgs[0])
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()
	p := DefaultProvider()
	if p.Name != "openai" {
		t.Errorf("Name = %q, want openai", p.Name)
	}
	if len(p.Configs) != 1 {
		t.Fatalf("Configs count = %d, want 1", len(p.Configs))
	}
	if p.Configs[0].Name != "openai-image" {
		t.Errorf("config name = %q, want openai-image", p.Configs[0].Name)
	}
}

// ---------- Compile error paths ----------

func TestCompile_MissingPrompt(t *testing.T) {
	t.Parallel()
	eng := New(Config{})
	graph := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
	}
	_, err := eng.Compile(graph)
	if err != ErrMissingPrompt {
		t.Fatalf("Compile() error = %v, want ErrMissingPrompt", err)
	}
}

func TestCompile_EmptyPromptIsError(t *testing.T) {
	t.Parallel()
	eng := New(Config{})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "   "}},
	}
	_, err := eng.Compile(graph)
	if err != ErrMissingPrompt {
		t.Fatalf("Compile() error = %v, want ErrMissingPrompt", err)
	}
}

// ---------- Execute error paths ----------

func TestExecute_MissingAPIKey(t *testing.T) {
	t.Parallel()
	eng := New(Config{}) // no APIKey, no env var
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("error = %q, want 'missing API key'", err.Error())
	}
}

func TestExecute_HTTPErrorResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid request"}}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("Execute() expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("error = %q, want status 400", err.Error())
	}
}

func TestExecute_EmptyDataResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a dog"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "did not contain generated images") {
		t.Fatalf("Execute() error = %v, want 'did not contain generated images'", err)
	}
}

func TestExecute_NoUsableResult(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"","b64_json":""}]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a bird"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "usable image result") {
		t.Fatalf("Execute() error = %v, want 'usable image result'", err)
	}
}

func TestExecute_DallEIncludesStyleAndResponseFormat(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/img.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "dall-e-3",
		Quality: "hd",
		Style:   "vivid",
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "sunset"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
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

// ---------- imageMIMEFromOutputFormat ----------

func TestImageMIMEFromOutputFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		format string
		want   string
	}{
		{"png", "image/png"},
		{"", "image/png"},
		{"jpeg", "image/jpeg"},
		{"jpg", "image/jpeg"},
		{"webp", "image/webp"},
		{"JPEG", "image/jpeg"},
		{" WebP ", "image/webp"},
		{"bmp", "image/png"}, // unknown defaults to png
	}
	for _, tt := range tests {
		if got := imageMIMEFromOutputFormat(tt.format); got != tt.want {
			t.Errorf("imageMIMEFromOutputFormat(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}

// ---------- hasImageSource additional coverage ----------

func TestHasImageSource_ImagePath(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"image_path": "/tmp/x.png"}},
	}
	if !hasImageSource(g) {
		t.Error("image_path should be detected")
	}
}

func TestHasImageSource_ImageURL(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"image_url": "https://example.com/img.png"}},
	}
	if !hasImageSource(g) {
		t.Error("image_url should be detected")
	}
}

func TestHasImageSource_LoadImageURL(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
	}
	if !hasImageSource(g) {
		t.Error("LoadImage{url} should be detected")
	}
}

// ---------- fetchImageURL ----------

func TestFetchImageURL_RemoteDisabled(t *testing.T) {
	t.Parallel()
	eng := New(Config{
		APIKey:                  "test-key",
		DisableRemoteMediaFetch: true,
	})
	_, err := eng.fetchImageURL(context.Background(), "https://example.com/img.png")
	if err != ErrRemoteMediaDisabled {
		t.Fatalf("fetchImageURL() error = %v, want ErrRemoteMediaDisabled", err)
	}
}

func TestFetchImageURL_Success(t *testing.T) {
	t.Parallel()
	imgData := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imgData)
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key"})
	got, err := eng.fetchImageURL(context.Background(), server.URL+"/img.png")
	if err != nil {
		t.Fatalf("fetchImageURL() error = %v", err)
	}
	if len(got) != len(imgData) {
		t.Fatalf("fetchImageURL() returned %d bytes, want %d", len(got), len(imgData))
	}
}

func TestFetchImageURL_HTTPError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key"})
	_, err := eng.fetchImageURL(context.Background(), server.URL+"/missing.png")
	if err == nil {
		t.Fatal("fetchImageURL() expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want 404 status", err.Error())
	}
}

// ---------- loadImageBytes paths ----------

func TestLoadImageBytes_Base64(t *testing.T) {
	t.Parallel()
	eng := New(Config{APIKey: "test-key"})
	// "AQID" is base64 for {1, 2, 3}
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"image_b64": "AQID"}},
	}
	got, err := eng.loadImageBytes(context.Background(), graph)
	if err != nil {
		t.Fatalf("loadImageBytes() error = %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("loadImageBytes() = %v, want [1 2 3]", got)
	}
}

func TestLoadImageBytes_FilePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/test.png"
	if err := os.WriteFile(imgPath, []byte{0xAA, 0xBB}, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	eng := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"image_path": imgPath}},
	}
	got, err := eng.loadImageBytes(context.Background(), graph)
	if err != nil {
		t.Fatalf("loadImageBytes() error = %v", err)
	}
	if len(got) != 2 || got[0] != 0xAA {
		t.Fatalf("loadImageBytes() = %v, want [0xAA 0xBB]", got)
	}
}

func TestLoadImageBytes_URL(t *testing.T) {
	t.Parallel()
	imgData := []byte{0xDE, 0xAD}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(imgData)
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"image_url": server.URL + "/img.png"}},
	}
	got, err := eng.loadImageBytes(context.Background(), graph)
	if err != nil {
		t.Fatalf("loadImageBytes() error = %v", err)
	}
	if len(got) != 2 || got[0] != 0xDE {
		t.Fatalf("loadImageBytes() = %v, want [0xDE 0xAD]", got)
	}
}

func TestLoadImageBytes_NoSource(t *testing.T) {
	t.Parallel()
	eng := New(Config{APIKey: "test-key"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "no image here"}},
	}
	_, err := eng.loadImageBytes(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "no image source") {
		t.Fatalf("loadImageBytes() error = %v, want 'no image source'", err)
	}
}

// ---------- executeEdits additional coverage ----------

func TestExecuteEdits_DallEModelUsesResponseFormatURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	var gotModel, gotRespFormat string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		gotModel = r.FormValue("model")
		gotRespFormat = r.FormValue("response_format")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/edited.png"}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "dall-e-2",
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "edit this"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}
	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotModel != "dall-e-2" {
		t.Errorf("model = %q, want dall-e-2", gotModel)
	}
	if gotRespFormat != "url" {
		t.Errorf("response_format = %q, want url", gotRespFormat)
	}
	if got.Value != "https://cdn.example.com/edited.png" {
		t.Errorf("result = %q, want URL", got.Value)
	}
}

func TestExecuteEdits_GPTImageWithOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	var gotBackground, gotOutputFormat, gotModeration, gotQuality string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		gotBackground = r.FormValue("background")
		gotOutputFormat = r.FormValue("output_format")
		gotModeration = r.FormValue("moderation")
		gotQuality = r.FormValue("quality")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		Model:        "gpt-image-2",
		Quality:      "high",
		Background:   "transparent",
		OutputFormat: "webp",
		Moderation:   "low",
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "make it glow"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}
	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotBackground != "transparent" {
		t.Errorf("background = %q", gotBackground)
	}
	if gotOutputFormat != "webp" {
		t.Errorf("output_format = %q", gotOutputFormat)
	}
	if gotModeration != "low" {
		t.Errorf("moderation = %q", gotModeration)
	}
	if gotQuality != "high" {
		t.Errorf("quality = %q", gotQuality)
	}
	if !strings.HasPrefix(got.Value, "data:image/webp;base64,") {
		t.Errorf("result = %q, want webp data URI", got.Value)
	}
}

func TestExecuteEdits_HTTPError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "bad-key", BaseURL: server.URL, Model: "gpt-image-2"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "edit"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("Execute() error = %v, want 401", err)
	}
}

func TestExecuteEdits_EmptyData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: "gpt-image-2"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "edit"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "did not contain images") {
		t.Fatalf("Execute() error = %v, want 'did not contain images'", err)
	}
}

func TestExecuteEdits_NoUsableResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	imgPath := dir + "/in.png"
	if err := os.WriteFile(imgPath, []byte{0x89}, 0o600); err != nil {
		t.Fatalf("write tmp image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"","b64_json":""}]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: "gpt-image-2"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "edit"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image": imgPath}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err == nil || !strings.Contains(err.Error(), "missing url and b64_json") {
		t.Fatalf("Execute() error = %v, want 'missing url and b64_json'", err)
	}
}

// ---------- Execute via image_b64 source (edits with base64 input) ----------

func TestExecuteEdits_Base64Source(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Errorf("path = %q, want /images/edits", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAAA"}]}`))
	}))
	defer server.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: server.URL, Model: "gpt-image-2"})
	// "AQID" is base64 for {1,2,3}
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "recolor"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"image_b64": "AQID"}},
	}
	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(got.Value, "data:image/png;base64,") {
		t.Errorf("result = %q, want data URI", got.Value)
	}
}

// ---------- New config edge cases ----------

func TestNew_TrailingSlashTrimmed(t *testing.T) {
	t.Parallel()
	eng := New(Config{BaseURL: "https://api.example.com/v1/"})
	if eng.baseURL != "https://api.example.com/v1" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", eng.baseURL)
	}
}

func TestNew_DefaultsApplied(t *testing.T) {
	t.Parallel()
	eng := New(Config{})
	if eng.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", eng.baseURL, defaultBaseURL)
	}
	if eng.model != defaultModel {
		t.Errorf("model = %q, want %q", eng.model, defaultModel)
	}
	if !eng.allowRemoteMediaFetch {
		t.Error("allowRemoteMediaFetch should default to true")
	}
}

// ---------- Execute with image_url source (fetchImageURL integration) ----------

func TestExecuteEdits_ImageURLSource(t *testing.T) {
	t.Parallel()
	imgData := []byte{0x89, 0x50, 0x4e, 0x47}

	// Image server
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(imgData)
	}))
	defer imgServer.Close()

	// API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Errorf("path = %q, want /images/edits", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAAA"}]}`))
	}))
	defer apiServer.Close()

	eng := New(Config{APIKey: "test-key", BaseURL: apiServer.URL, Model: "gpt-image-2"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "enhance"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"image_url": imgServer.URL + "/photo.png"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestExecuteEdits_RemoteMediaDisabled(t *testing.T) {
	t.Parallel()
	eng := New(Config{
		APIKey:                  "test-key",
		BaseURL:                 "http://localhost:1",
		DisableRemoteMediaFetch: true,
		Model:                   "gpt-image-2",
	})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "enhance"}},
		"2": {ClassType: "Options", Inputs: map[string]any{"image_url": "https://example.com/img.png"}},
	}
	_, err := eng.Execute(context.Background(), graph)
	if err != ErrRemoteMediaDisabled {
		t.Fatalf("Execute() error = %v, want ErrRemoteMediaDisabled", err)
	}
}

// ---------- ModelInfos ----------

func TestModelInfos(t *testing.T) {
	t.Parallel()
	infos := ModelInfos()
	if len(infos) != 3 {
		t.Fatalf("ModelInfos() returned %d items, want 3", len(infos))
	}
	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
		if info.Provider != "openai" {
			t.Errorf("info %q provider = %q, want openai", info.Name, info.Provider)
		}
		if info.Capability != "image" {
			t.Errorf("info %q capability = %q, want image", info.Name, info.Capability)
		}
	}
	for _, want := range []string{"gpt-image-2", "dall-e-3", "dall-e-2"} {
		if !names[want] {
			t.Errorf("missing model %q in ModelInfos", want)
		}
	}
}
