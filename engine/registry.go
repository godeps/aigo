package engine

import (
	"sort"
	"sync"
)

// Entry describes a registered engine provider.
type Entry struct {
	Name               string
	DisplayName        DisplayName
	Engine             Engine
	ConfigSchemaFunc   func() []ConfigField
	ModelsByCapability func() map[string][]string
}

// Registry provides engine registration, lookup, and discovery.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewRegistry creates an empty engine registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]Entry),
	}
}

// Register adds an engine entry to the registry.
// If an entry with the same name exists, it is replaced.
func (r *Registry) Register(name string, e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e.Name = name
	r.entries[name] = e
}

// Get returns the entry for the given engine name.
func (r *Registry) Get(name string) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	return e, ok
}

// List returns all registered engine names in sorted order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// FindByCapability returns all entries whose ModelsByCapability includes the given media type.
func (r *Registry) FindByCapability(mediaType string) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Entry
	for _, e := range r.entries {
		if e.ModelsByCapability == nil {
			continue
		}
		models := e.ModelsByCapability()
		if _, ok := models[mediaType]; ok {
			result = append(result, e)
		}
	}
	return result
}

// AllModels returns all registered models grouped by engine name and capability.
// Result: map[engineName]map[capability][]models
func (r *Registry) AllModels() map[string]map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]map[string][]string, len(r.entries))
	for name, e := range r.entries {
		if e.ModelsByCapability != nil {
			result[name] = e.ModelsByCapability()
		}
	}
	return result
}

// AllConfigSchemas returns the configuration schema for each registered engine.
func (r *Registry) AllConfigSchemas() map[string][]ConfigField {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string][]ConfigField, len(r.entries))
	for name, e := range r.entries {
		if e.ConfigSchemaFunc != nil {
			result[name] = e.ConfigSchemaFunc()
		}
	}
	return result
}

// Len returns the number of registered engines.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}
