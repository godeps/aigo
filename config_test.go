package aigo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/godeps/aigo/engine"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "engines.json")

	data := `{
		"engines": [
			{"name": "img", "provider": "test-provider", "model": "test-model"},
			{"name": "disabled", "provider": "test-provider", "enabled": false}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.Engines) != 2 {
		t.Fatalf("LoadConfig() got %d engines, want 2", len(cfg.Engines))
	}
	if cfg.Engines[0].Name != "img" {
		t.Errorf("Engines[0].Name = %q, want %q", cfg.Engines[0].Name, "img")
	}
	if cfg.Engines[0].Provider != "test-provider" {
		t.Errorf("Engines[0].Provider = %q, want %q", cfg.Engines[0].Provider, "test-provider")
	}
	if cfg.Engines[0].Model != "test-model" {
		t.Errorf("Engines[0].Model = %q, want %q", cfg.Engines[0].Model, "test-model")
	}
	if !cfg.Engines[0].IsEnabled() {
		t.Error("Engines[0].IsEnabled() = false, want true")
	}
	if cfg.Engines[1].IsEnabled() {
		t.Error("Engines[1].IsEnabled() = true, want false")
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("/nonexistent/path.json")
	if err == nil {
		t.Fatal("LoadConfig() expected error for missing file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected error for invalid JSON")
	}
}

func TestApplyConfig(t *testing.T) {
	t.Parallel()

	// Register a test factory.
	engine.RegisterFactory("test-apply", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return stubEngine{result: "from-" + cfg.Model}, nil
	})

	client := NewClient()

	f := false
	cfg := FileConfig{
		Engines: []engine.EngineConfig{
			{Name: "e1", Provider: "test-apply", Model: "m1"},
			{Name: "e2", Provider: "test-apply", Model: "m2"},
			{Name: "disabled", Provider: "test-apply", Enabled: &f},
		},
	}

	registered, err := client.ApplyConfig(cfg)
	if err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}
	if len(registered) != 2 {
		t.Fatalf("ApplyConfig() registered %d, want 2", len(registered))
	}
	if registered[0] != "e1" || registered[1] != "e2" {
		t.Errorf("registered = %v, want [e1, e2]", registered)
	}

	// Verify engines work.
	names := client.EngineNames()
	found := 0
	for _, n := range names {
		if n == "e1" || n == "e2" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected e1 and e2 in engine names, got %v", names)
	}
}

func TestApplyConfigMissingFactory(t *testing.T) {
	t.Parallel()

	client := NewClient()
	cfg := FileConfig{
		Engines: []engine.EngineConfig{
			{Name: "x", Provider: "nonexistent-provider"},
		},
	}

	_, err := client.ApplyConfig(cfg)
	if err == nil {
		t.Fatal("ApplyConfig() expected error for missing factory")
	}
}

func TestApplyConfigMissingName(t *testing.T) {
	t.Parallel()

	engine.RegisterFactory("test-noname", func(cfg engine.EngineConfig) (engine.Engine, error) {
		return stubEngine{}, nil
	})

	client := NewClient()
	cfg := FileConfig{
		Engines: []engine.EngineConfig{
			{Provider: "test-noname"},
		},
	}

	_, err := client.ApplyConfig(cfg)
	if err == nil {
		t.Fatal("ApplyConfig() expected error for missing name")
	}
}
