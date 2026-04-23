package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
