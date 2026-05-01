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
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/images/generations")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
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
			t.Fatalf("decode body: %v", err)
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
			t.Fatalf("decode body: %v", err)
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
			t.Fatalf("unexpected path %s", r.URL.Path)
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
