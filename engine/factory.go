package engine

import (
	"net/http"
	"sync"
	"time"
)

// EngineConfig is a generic, JSON-friendly configuration for creating an engine.
// Used by LoadConfig / ApplyConfig for declarative engine setup.
type EngineConfig struct {
	Name     string            `json:"name"`               // registration name
	Provider string            `json:"provider"`           // engine package key, e.g. "alibabacloud", "kling"
	Model    string            `json:"model,omitempty"`    // model override
	APIKey   string            `json:"api_key,omitempty"`  // explicit API key (overrides env)
	BaseURL  string            `json:"base_url,omitempty"` // custom API endpoint
	Enabled  *bool             `json:"enabled,omitempty"`  // default true; set false to skip
	Metadata map[string]string `json:"metadata,omitempty"` // provider-specific fields (e.g. voiceId, endpoint)

	// Capability tells the factory what media capability this engine serves
	// ("image", "video", "tts", "asr", "music", "3d"). Used by engines that
	// support multiple capabilities (e.g. newapi) to select the correct route
	// when the model is not in the known catalog.
	Capability string `json:"capability,omitempty"`

	// WaitForCompletion controls async-task polling on backends that submit
	// asynchronous jobs (DashScope X-DashScope-Async, etc.). nil = use the
	// engine's smart default; *true = always poll until SUCCEEDED/FAILED;
	// *false = return the upstream task_id immediately so the caller resumes
	// it later. The pointer type is intentional so engines can distinguish
	// "user did not say" from "user explicitly disabled".
	WaitForCompletion *bool `json:"wait_for_completion,omitempty"`

	// PollInterval overrides the engine's default polling cadence; only
	// effective when WaitForCompletion resolves to true.
	PollInterval time.Duration `json:"poll_interval,omitempty"`

	// HTTPClient optionally overrides the HTTP client used by engines created
	// through generic factories. It is intentionally excluded from JSON config
	// so callers can inject transports without serializing runtime state.
	HTTPClient *http.Client `json:"-"`
}

// WaitForCompletionOr returns the resolved WaitForCompletion value, falling
// back to def when the user did not configure it explicitly.
func (c EngineConfig) WaitForCompletionOr(def bool) bool {
	if c.WaitForCompletion == nil {
		return def
	}
	return *c.WaitForCompletion
}

// Meta returns the metadata value for key, or fallback if not present.
func (c EngineConfig) Meta(key, fallback string) string {
	if c.Metadata != nil {
		if v, ok := c.Metadata[key]; ok && v != "" {
			return v
		}
	}
	return fallback
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
