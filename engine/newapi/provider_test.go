package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) == 0 {
		t.Fatal("ConfigSchema returned empty")
	}

	foundAPIKey := false
	foundBaseURL := false
	found := map[string]bool{
		"model":              false,
		"capability":         false,
		"quality":            false,
		"style":              false,
		"background":         false,
		"output_format":      false,
		"moderation":         false,
		"output_compression": false,
	}
	for _, f := range fields {
		if _, ok := found[f.Key]; ok {
			found[f.Key] = true
		}
		switch f.Key {
		case "apiKey":
			foundAPIKey = true
			if !f.Required {
				t.Error("apiKey should be required")
			}
			if f.Type != "secret" {
				t.Errorf("apiKey type = %q, want secret", f.Type)
			}
		case "baseUrl":
			foundBaseURL = true
			if f.Type != "url" {
				t.Errorf("baseUrl type = %q, want url", f.Type)
			}
		}
	}
	if !foundAPIKey {
		t.Error("ConfigSchema missing apiKey field")
	}
	if !foundBaseURL {
		t.Error("ConfigSchema missing baseUrl field")
	}
	for key, ok := range found {
		if !ok {
			t.Errorf("ConfigSchema missing %s field", key)
		}
	}
}

func TestModelInfos(t *testing.T) {
	t.Parallel()

	infos := ModelInfos()
	if len(infos) == 0 {
		t.Fatal("ModelInfos returned empty")
	}

	for _, info := range infos {
		if info.Name == "" {
			t.Error("ModelInfo has empty Name")
		}
		if info.Provider == "" {
			t.Errorf("ModelInfo %q has empty Provider", info.Name)
		}
		if len(info.DisplayName) == 0 {
			t.Errorf("ModelInfo %q has empty DisplayName", info.Name)
		}
		if info.Capability == "" {
			t.Errorf("ModelInfo %q has empty Capability", info.Name)
		}
		// All newapi models should have "newapi" provider
		if info.Provider != "newapi" {
			t.Errorf("ModelInfo %q provider = %q, want newapi", info.Name, info.Provider)
		}
	}
}

func TestDefaultProvider(t *testing.T) {
	t.Parallel()

	p := DefaultProvider()
	if p.Name != "newapi" {
		t.Errorf("provider name = %q, want newapi", p.Name)
	}
	if len(p.Configs) == 0 {
		t.Fatal("DefaultProvider has no configs")
	}
	cfg := p.Configs[0]
	if cfg.Name != "newapi" {
		t.Errorf("config name = %q, want newapi", cfg.Name)
	}
	if cfg.Engine == nil {
		t.Error("config engine is nil")
	}
	if len(cfg.EnvVars) == 0 {
		t.Error("config envvars is empty")
	}
}

func TestFactoryPassesImageOptions(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("path = %q, want /v1/images/generations", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAECAw=="}]}`))
	}))
	defer server.Close()

	factory, ok := engine.GetFactory("newapi")
	if !ok {
		t.Fatal("newapi factory not registered")
	}
	eng, err := factory(engine.EngineConfig{
		APIKey:            "sk-test",
		BaseURL:           server.URL,
		Model:             "gpt-image-1-mini",
		Quality:           "high",
		Style:             "vivid",
		Background:        "transparent",
		OutputFormat:      "webp",
		Moderation:        "low",
		OutputCompression: 72,
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a studio product photo"}},
	}
	if _, err := eng.Execute(context.Background(), graph); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPayload["quality"] != "high" {
		t.Fatalf("quality = %#v, want high; payload=%#v", gotPayload["quality"], gotPayload)
	}
	if _, ok := gotPayload["style"]; ok {
		t.Fatalf("gpt-image-* payload must omit style: %#v", gotPayload)
	}
	if gotPayload["background"] != "transparent" {
		t.Fatalf("background = %#v, want transparent; payload=%#v", gotPayload["background"], gotPayload)
	}
	if gotPayload["output_format"] != "webp" {
		t.Fatalf("output_format = %#v, want webp; payload=%#v", gotPayload["output_format"], gotPayload)
	}
	if gotPayload["moderation"] != "low" {
		t.Fatalf("moderation = %#v, want low; payload=%#v", gotPayload["moderation"], gotPayload)
	}
	if gotPayload["output_compression"] != float64(72) {
		t.Fatalf("output_compression = %#v, want 72; payload=%#v", gotPayload["output_compression"], gotPayload)
	}
}

func TestFactoryPassesMetadataImageOptions(t *testing.T) {
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

	factory, ok := engine.GetFactory("newapi")
	if !ok {
		t.Fatal("newapi factory not registered")
	}
	eng, err := factory(engine.EngineConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "gpt-image-1-mini",
		Metadata: map[string]string{
			"quality":           "high",
			"outputFormat":      "webp",
			"outputCompression": "73",
			"background":        "transparent",
			"moderation":        "low",
		},
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a studio product photo"}},
	}
	if _, err := eng.Execute(context.Background(), graph); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPayload["quality"] != "high" {
		t.Fatalf("quality = %#v, want high; payload=%#v", gotPayload["quality"], gotPayload)
	}
	if gotPayload["output_format"] != "webp" {
		t.Fatalf("output_format = %#v, want webp; payload=%#v", gotPayload["output_format"], gotPayload)
	}
	if gotPayload["output_compression"] != float64(73) {
		t.Fatalf("output_compression = %#v, want 73; payload=%#v", gotPayload["output_compression"], gotPayload)
	}
}

func TestFactoryCustomModelUsesCapabilityFallback(t *testing.T) {
	t.Parallel()

	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("path = %q, want /v1/images/generations", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/custom.png"}]}`))
	}))
	defer server.Close()

	factory, ok := engine.GetFactory("newapi")
	if !ok {
		t.Fatal("newapi factory not registered")
	}
	eng, err := factory(engine.EngineConfig{
		APIKey:     "sk-test",
		BaseURL:    server.URL,
		Model:      "aihub-custom-image-model",
		Capability: "image",
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a custom model prompt"}},
	}
	if _, err := eng.Execute(context.Background(), graph); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPayload["model"] != "aihub-custom-image-model" {
		t.Fatalf("model = %#v, want custom model; payload=%#v", gotPayload["model"], gotPayload)
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      MediaKind
		model     string
		waitVideo bool
		wantMedia string
		wantSync  bool
		wantPoll  bool
	}{
		{"image", KindImage, "dall-e-3", false, "image", true, false},
		{"video_nowait", KindVideo, "sora", false, "video", true, false},
		{"video_wait", KindVideo, "sora", true, "video", false, true},
		{"speech", KindSpeech, "tts-1", false, "audio", true, false},
		{"default_kind", "", "test", false, "image", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := New(Config{
				BaseURL:           "https://example.com",
				Model:             tc.model,
				Kind:              tc.kind,
				WaitForCompletion: tc.waitVideo,
			})
			cap := eng.Capabilities()
			if len(cap.MediaTypes) == 0 || cap.MediaTypes[0] != tc.wantMedia {
				t.Errorf("MediaTypes = %v, want [%s]", cap.MediaTypes, tc.wantMedia)
			}
			if cap.SupportsSync != tc.wantSync {
				t.Errorf("SupportsSync = %v, want %v", cap.SupportsSync, tc.wantSync)
			}
			if cap.SupportsPoll != tc.wantPoll {
				t.Errorf("SupportsPoll = %v, want %v", cap.SupportsPoll, tc.wantPoll)
			}
			if len(cap.Models) != 1 || cap.Models[0] != tc.model {
				t.Errorf("Models = %v, want [%s]", cap.Models, tc.model)
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL: "https://example.com/v1/",
		Model:   "  test-model  ",
		APIKey:  "  sk-key  ",
	})
	if eng.model != "test-model" {
		t.Errorf("model = %q, want trimmed", eng.model)
	}
	if eng.apiKey != "sk-key" {
		t.Errorf("apiKey = %q, want trimmed", eng.apiKey)
	}
	if eng.kind != KindImage {
		t.Errorf("kind = %q, want KindImage (default)", eng.kind)
	}
	if eng.jimengVer != "2022-08-31" {
		t.Errorf("jimengVer = %q, want default", eng.jimengVer)
	}
	if eng.pollInterval <= 0 {
		t.Error("pollInterval should be positive")
	}
}

func TestNewCustomJimengVersion(t *testing.T) {
	t.Parallel()

	eng := New(Config{
		BaseURL:       "https://example.com",
		Model:         "test",
		JimengVersion: "2024-01-01",
	})
	if eng.jimengVer != "2024-01-01" {
		t.Errorf("jimengVer = %q, want 2024-01-01", eng.jimengVer)
	}
}
