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
