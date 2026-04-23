package engine

import (
	"context"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// OutputKind classifies the string returned by an engine.
type OutputKind int

const (
	OutputUnknown   OutputKind = iota
	OutputURL
	OutputDataURI
	OutputJSON
	OutputPlainText
)

// Result is the structured output of Engine.Execute.
type Result struct {
	Value   string
	Kind    OutputKind
	Results []ResultItem // multiple results for batch generation; may be nil for single-result engines
}

// ResultItem represents a single output in a multi-result response.
type ResultItem struct {
	Value    string            // output URL, data URI, or text
	Kind     OutputKind        // classification of Value
	Metadata map[string]string // engine-specific metadata (e.g. "seed", "index")
}

// WebhookConfig is a common webhook configuration for engines that support
// completion notifications via webhook.
type WebhookConfig struct {
	URL     string            // webhook endpoint
	Secret  string            // optional signing secret for verification
	Headers map[string]string // optional additional headers
}

// Engine executes a workflow graph against a concrete backend.
type Engine interface {
	Execute(ctx context.Context, graph workflow.Graph) (Result, error)
}

// Capability describes what an engine can do.
type Capability struct {
	MediaTypes   []string // e.g. ["image", "video", "audio"]
	Models       []string
	Sizes        []string // e.g. ["1024x1024", "1280x720"]
	Voices       []string // supported voice identifiers for TTS engines
	MaxDuration  int      // max video/audio duration in seconds; 0 = not applicable
	SupportsSync bool
	SupportsPoll bool
}

// Describer is an optional interface that engines can implement to advertise capabilities.
type Describer interface {
	Capabilities() Capability
}

// Namer is an optional interface that engines can implement to provide localized display names.
type Namer interface {
	DisplayName() DisplayName
}

// ModelInfo describes a model with i18n metadata for display purposes.
type ModelInfo struct {
	Name        string      `json:"name"`                   // API model string, e.g. "kling-v2-master"
	Provider    string      `json:"provider"`               // engine package name, e.g. "kling", "alibabacloud"
	DisplayName DisplayName `json:"display_name"`           // i18n display name, e.g. "Kling V2 Master" / "可灵 V2 大师版"
	Description DisplayName `json:"description"`            // i18n short functional description
	Intro       DisplayName `json:"intro,omitempty"`        // i18n detailed introduction
	DocURL      string      `json:"doc_url,omitempty"`      // official documentation URL
	Capability  string      `json:"capability"`             // "image", "video", "tts", "music", "3d", etc.
	Deprecated  bool        `json:"deprecated,omitempty"`   // true if the model is retired or superseded
	Tags        []string    `json:"tags,omitempty"`         // flexible classification, e.g. "open-source", "fast", "chinese-optimized"
}

// ModelInfoProvider is an optional interface for engines that expose i18n model metadata.
type ModelInfoProvider interface {
	ModelInfos() []ModelInfo
}

// DryRunResult is the outcome of a dry-run estimation.
type DryRunResult struct {
	WillPoll      bool
	EstimatedTime string   // human-readable estimate, e.g. "30s-2m"
	Warnings      []string // potential issues with the request
}

// DryRunner is an optional interface for engines that support dry-run estimation.
type DryRunner interface {
	DryRun(graph workflow.Graph) (DryRunResult, error)
}

// Resumer is an optional interface for engines that support resuming
// an already-submitted async task by its remote ID.
type Resumer interface {
	Resume(ctx context.Context, remoteID string) (Result, error)
}

// ClassifyOutput heuristically classifies a raw output string.
func ClassifyOutput(s string) OutputKind {
	s = strings.TrimSpace(s)
	if s == "" {
		return OutputUnknown
	}
	if strings.HasPrefix(s, "data:") {
		return OutputDataURI
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return OutputURL
	}
	if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
		return OutputJSON
	}
	return OutputPlainText
}

// DisplayName holds localized display texts keyed by language code (e.g. "en", "zh", "ja").
type DisplayName map[string]string

// Get returns the text for the given language code, or empty string if not found.
func (d DisplayName) Get(lang string) string { return d[lang] }

// String returns the English text as default display.
func (d DisplayName) String() string { return d["en"] }

// Discoverer is a package-level interface for providers that can enumerate
// all known models grouped by capability (e.g. "image", "video", "tts").
// Unlike Engine (per-instance), Discoverer is a static catalog of models
// the provider SDK knows how to handle.
type Discoverer interface {
	ModelsByCapability() map[string][]string
}

// ConfigField describes a single configuration parameter for an engine provider.
// Engine packages expose a package-level ConfigSchema() []ConfigField function
// so that UIs can dynamically render configuration forms.
type ConfigField struct {
	Key         string `json:"key"`                   // field identifier, e.g. "apiKey", "appId"
	Label       string `json:"label"`                 // human-readable label, e.g. "API Key"
	Type        string `json:"type"`                  // "string", "secret", "url"
	Required    bool   `json:"required"`
	EnvVar      string `json:"envVar,omitempty"`       // fallback environment variable name
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}
