package stability

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestExecuteCallsAPI(t *testing.T) {
	t.Parallel()

	fakeImage := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v2beta/stable-image/generate/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("Content-Type = %q, want multipart", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"image":"` + fakeImage + `","finish_reason":"SUCCESS"}`))
	}))
	defer server.Close()

	e := New(Config{APIKey: "test-key", BaseURL: server.URL})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat in space"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Kind != engine.OutputDataURI {
		t.Fatalf("Kind = %v, want OutputDataURI", result.Kind)
	}
	if !strings.HasPrefix(result.Value, "data:image/png;base64,") {
		t.Fatalf("Value = %q, want data URI prefix", result.Value[:40])
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	e := New(Config{Model: ModelSD35Large})
	cap := e.Capabilities()
	if len(cap.MediaTypes) != 1 || cap.MediaTypes[0] != "image" {
		t.Fatalf("MediaTypes = %v", cap.MediaTypes)
	}
	if !cap.SupportsSync {
		t.Fatal("SupportsSync should be true")
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	if len(m["image"]) == 0 {
		t.Fatal("expected image models")
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()
	fields := ConfigSchema()
	if len(fields) < 2 {
		t.Fatalf("expected at least 2 config fields, got %d", len(fields))
	}
}
