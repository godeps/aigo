package aigo

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/godeps/aigo/engine"
)

// FileConfig is the top-level structure for JSON-based engine configuration.
type FileConfig struct {
	Engines []engine.EngineConfig `json:"engines"`
}

// LoadConfig reads engine configuration from a JSON file.
func LoadConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, fmt.Errorf("aigo: read config %s: %w", path, err)
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("aigo: parse config %s: %w", path, err)
	}
	return cfg, nil
}

// ApplyURI parses a URI (or comma-separated URIs) and registers engines
// using the factory system. Returns the names of successfully registered engines.
//
// Example:
//
//	client.ApplyURI("dashscope://sk-xxx@proxy.com/api/v1?model=qwen-image,openai://sk-abc?model=dall-e-3")
func (c *Client) ApplyURI(uris string) ([]string, error) {
	configs, err := engine.ParseURIs(uris)
	if err != nil {
		return nil, err
	}
	var registered []string
	for _, ec := range configs {
		factory, ok := engine.GetFactory(ec.Provider)
		if !ok {
			return registered, fmt.Errorf("aigo: no factory registered for provider %q (from URI)", ec.Provider)
		}
		eng, err := factory(ec)
		if err != nil {
			return registered, fmt.Errorf("aigo: create engine %s (provider=%s): %w", ec.Name, ec.Provider, err)
		}
		if err := c.RegisterEngine(ec.Name, eng); err != nil {
			return registered, err
		}
		registered = append(registered, ec.Name)
	}
	return registered, nil
}

// ApplyEnvURI reads ENGINE_URIS from the environment and registers engines.
// Returns nil if the env var is not set. This is a convenience wrapper around ApplyURI.
func (c *Client) ApplyEnvURI() ([]string, error) {
	configs, err := engine.NewFromEnv()
	if err != nil {
		return nil, err
	}
	if configs == nil {
		return nil, nil
	}
	var registered []string
	for _, ec := range configs {
		factory, ok := engine.GetFactory(ec.Provider)
		if !ok {
			return registered, fmt.Errorf("aigo: no factory registered for provider %q (from URI)", ec.Provider)
		}
		eng, err := factory(ec)
		if err != nil {
			return registered, fmt.Errorf("aigo: create engine %s (provider=%s): %w", ec.Name, ec.Provider, err)
		}
		if err := c.RegisterEngine(ec.Name, eng); err != nil {
			return registered, err
		}
		registered = append(registered, ec.Name)
	}
	return registered, nil
}

// ApplyConfig registers engines from a FileConfig using registered factories.
// Disabled entries are silently skipped. Returns the names of successfully registered engines.
func (c *Client) ApplyConfig(cfg FileConfig) ([]string, error) {
	var registered []string
	for _, ec := range cfg.Engines {
		if !ec.IsEnabled() {
			continue
		}
		if ec.Name == "" {
			return registered, fmt.Errorf("aigo: config entry missing name (provider=%s)", ec.Provider)
		}
		factory, ok := engine.GetFactory(ec.Provider)
		if !ok {
			return registered, fmt.Errorf("aigo: no factory registered for provider %q", ec.Provider)
		}
		eng, err := factory(ec)
		if err != nil {
			return registered, fmt.Errorf("aigo: create engine %s (provider=%s): %w", ec.Name, ec.Provider, err)
		}
		if err := c.RegisterEngine(ec.Name, eng); err != nil {
			return registered, err
		}
		registered = append(registered, ec.Name)
	}
	return registered, nil
}
