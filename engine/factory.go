package engine

import "sync"

// EngineConfig is a generic, JSON-friendly configuration for creating an engine.
// Used by LoadConfig / ApplyConfig for declarative engine setup.
type EngineConfig struct {
	Name     string `json:"name"`               // registration name
	Provider string `json:"provider"`            // engine package key, e.g. "alibabacloud", "kling"
	Model    string `json:"model,omitempty"`      // model override
	APIKey   string `json:"api_key,omitempty"`    // explicit API key (overrides env)
	BaseURL  string `json:"base_url,omitempty"`   // custom API endpoint
	Enabled  *bool  `json:"enabled,omitempty"`    // default true; set false to skip
}

// IsEnabled returns whether this engine config is enabled (default true).
func (c EngineConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// EngineFactory creates an Engine from a generic EngineConfig.
// Each engine package registers its factory via RegisterFactory.
type EngineFactory func(cfg EngineConfig) (Engine, error)

var (
	factoryMu sync.RWMutex
	factories = map[string]EngineFactory{}
)

// RegisterFactory registers a factory function for the given provider key.
// Typically called from an engine package's init() function.
func RegisterFactory(provider string, f EngineFactory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[provider] = f
}

// GetFactory returns the factory for the given provider key.
func GetFactory(provider string) (EngineFactory, bool) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[provider]
	return f, ok
}

// RegisteredFactories returns all registered provider keys.
func RegisteredFactories() []string {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	keys := make([]string, 0, len(factories))
	for k := range factories {
		keys = append(keys, k)
	}
	return keys
}
