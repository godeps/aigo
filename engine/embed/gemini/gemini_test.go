package gemini

import (
	"context"
	"testing"

	"github.com/godeps/aigo/engine/embed"
)


func TestNew_MissingKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != DefaultModel {
		t.Fatalf("model = %q, want %q", e.model, DefaultModel)
	}
	if e.dimensions != DefaultDimensions {
		t.Fatalf("dimensions = %d, want %d", e.dimensions, DefaultDimensions)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	t.Parallel()

	e, err := New(Config{
		APIKey:     "test-key",
		Model:      "custom-model",
		Dimensions: 512,
		RPM:        100,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.model != "custom-model" {
		t.Fatalf("model = %q, want %q", e.model, "custom-model")
	}
	if e.dimensions != 512 {
		t.Fatalf("dimensions = %d, want 512", e.dimensions)
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key", Dimensions: 256})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.Dimensions() != 256 {
		t.Fatalf("Dimensions() = %d, want 256", e.Dimensions())
	}
}

func TestEmbedCapabilities(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cap := e.EmbedCapabilities()

	if len(cap.SupportedTypes) != 3 {
		t.Fatalf("SupportedTypes = %v, want 3 types", cap.SupportedTypes)
	}
	if cap.MaxDimensions != 3072 {
		t.Fatalf("MaxDimensions = %d, want 3072", cap.MaxDimensions)
	}
	if !cap.SupportsMRL {
		t.Fatal("SupportsMRL should be true")
	}
	if len(cap.Models) != 1 || cap.Models[0] != DefaultModel {
		t.Fatalf("Models = %v", cap.Models)
	}
}

func TestBuildContent_Text(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	content := e.buildContent(embed.TextRequest("hello world", "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].Text != "hello world" {
		t.Fatalf("text = %q, want %q", content.Parts[0].Text, "hello world")
	}
}

func TestBuildContent_Image(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data := []byte{0xFF, 0xD8, 0xFF}
	content := e.buildContent(embed.ImageRequest(data, "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].InlineData == nil {
		t.Fatal("expected InlineData for image")
	}
	if content.Parts[0].InlineData.MIMEType != "image/jpeg" {
		t.Fatalf("mime = %q, want image/jpeg", content.Parts[0].InlineData.MIMEType)
	}
}

func TestBuildContent_Video(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data := []byte{0x00, 0x00, 0x00}
	content := e.buildContent(embed.VideoRequest(data, "RETRIEVAL_DOCUMENT"))
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if len(content.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	if content.Parts[0].InlineData == nil {
		t.Fatal("expected InlineData for video")
	}
	if content.Parts[0].InlineData.MIMEType != "video/mp4" {
		t.Fatalf("mime = %q, want video/mp4", content.Parts[0].InlineData.MIMEType)
	}
}

func TestEmbedBatch_Empty(t *testing.T) {
	t.Parallel()

	e, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	results, err := e.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}
