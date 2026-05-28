package newapi

import (
	"testing"
)

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	fields := ConfigSchema()
	if len(fields) == 0 {
		t.Fatal("ConfigSchema returned empty")
	}

	foundAPIKey := false
	foundBaseURL := false
	for _, f := range fields {
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
