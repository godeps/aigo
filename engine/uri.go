package engine

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// URI scheme 格式:
//
//	dashscope://sk-xxx@my-proxy.com/api/v1?model=qwen-image
//	openai://sk-abc?base_url=https://custom.com/v1&model=dall-e-3
//	dashscope://sk-xxx
//	kling://ak:sk@api.klingai.com/v1?model=kling-v3
//
// 多 engine 聚合通过逗号分割:
//
//	dashscope://sk-xxx?model=qwen-image,openai://sk-abc?model=dall-e-3
//
// 环境变量:
//
//	ENGINE_URIS="dashscope://sk-xxx?model=qwen-image,openai://sk-abc"

const EnvEngineURIs = "ENGINE_URIS"

// ParseURI parses an engine URI into an EngineConfig.
//
// Format: <provider>://<apikey>[@<host><path>][?params]
//
// Dual-key format for providers that need AK:SK:
//
//	<provider>://<access_key>:<secret_key>[@<host><path>][?params]
//
// The scheme maps to Provider (with alias resolution). The userinfo portion
// (before @) is the APIKey; if it contains ":", it's split into APIKey:SecretKey
// (SecretKey stored in Metadata["secretKey"]).
//
// The host+path (after @) becomes the BaseURL. Protocol defaults to https://
// unless the host is localhost/127.0.0.1 or scheme=http query param is set.
//
// Query parameters map to EngineConfig fields:
//
//	base_url       → BaseURL (overrides host-based URL)
//	model          → Model
//	quality        → Quality
//	style          → Style
//	background     → Background
//	output_format  → OutputFormat
//	moderation     → Moderation
//	output_compression → OutputCompression
//	name           → Name (defaults to Provider-Model or Provider if no model)
//	wait           → WaitForCompletion ("true"/"false")
//	poll_interval  → PollInterval (Go duration string)
//	capability     → Capability
//	scheme         → URL protocol override ("http" or "https")
//	<other>        → Metadata[key]
func ParseURI(raw string) (EngineConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return EngineConfig{}, fmt.Errorf("engine: empty URI")
	}

	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return EngineConfig{}, fmt.Errorf("engine: invalid URI (no scheme): %q", RedactURI(raw))
	}

	scheme := strings.ToLower(raw[:schemeEnd])
	rest := raw[schemeEnd+3:]

	provider := resolveScheme(scheme)
	cfg := EngineConfig{
		Provider: provider,
	}

	// Split query params
	var queryStr string
	if qIdx := strings.IndexByte(rest, '?'); qIdx >= 0 {
		queryStr = rest[qIdx+1:]
		rest = rest[:qIdx]
	}

	// Determine URL protocol (may be overridden by ?scheme= param later)
	urlScheme := "https"

	// Split userinfo@host
	if atIdx := strings.LastIndexByte(rest, '@'); atIdx >= 0 {
		userInfo := rest[:atIdx]
		hostPath := rest[atIdx+1:]

		// Parse AK:SK dual-key format
		if colonIdx := strings.IndexByte(userInfo, ':'); colonIdx >= 0 {
			cfg.APIKey = userInfo[:colonIdx]
			secret := userInfo[colonIdx+1:]
			if secret != "" {
				if cfg.Metadata == nil {
					cfg.Metadata = make(map[string]string)
				}
				cfg.Metadata["secretKey"] = secret
			}
		} else {
			cfg.APIKey = userInfo
		}

		if hostPath != "" {
			if isLocalHost(hostPath) {
				urlScheme = "http"
			}
			cfg.BaseURL = urlScheme + "://" + hostPath
		}
	} else {
		// No @, entire rest is apikey (possibly with AK:SK)
		if colonIdx := strings.IndexByte(rest, ':'); colonIdx >= 0 {
			cfg.APIKey = rest[:colonIdx]
			secret := rest[colonIdx+1:]
			if secret != "" {
				if cfg.Metadata == nil {
					cfg.Metadata = make(map[string]string)
				}
				cfg.Metadata["secretKey"] = secret
			}
		} else {
			cfg.APIKey = rest
		}
	}

	if cfg.APIKey == "" {
		return EngineConfig{}, fmt.Errorf("engine: %s URI missing API key (in %s)", scheme, RedactURI(raw))
	}

	// Parse query parameters
	if queryStr != "" {
		params, err := url.ParseQuery(queryStr)
		if err != nil {
			return EngineConfig{}, fmt.Errorf("engine: parse query params in %s: %w", RedactURI(raw), err)
		}

		// Handle scheme override before applyParams processes base_url
		if s := params.Get("scheme"); s == "http" || s == "https" {
			if cfg.BaseURL != "" {
				cfg.BaseURL = s + "://" + strings.TrimPrefix(strings.TrimPrefix(cfg.BaseURL, "https://"), "http://")
			}
			urlScheme = s
		}

		applyParams(&cfg, params)
	}

	// Auto-generate Name: provider-model or just provider
	if cfg.Name == "" {
		if cfg.Model != "" {
			cfg.Name = provider + "-" + sanitizeName(cfg.Model)
		} else {
			cfg.Name = provider
		}
	}

	return cfg, nil
}

// ParseURIs parses a comma-separated list of engine URIs.
// Duplicate names are automatically deduplicated with numeric suffixes.
func ParseURIs(raw string) ([]EngineConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("engine: empty URI list")
	}

	parts := strings.Split(raw, ",")
	configs := make([]EngineConfig, 0, len(parts))
	nameCount := map[string]int{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cfg, err := ParseURI(part)
		if err != nil {
			return nil, err
		}

		// Deduplicate names
		nameCount[cfg.Name]++
		if nameCount[cfg.Name] > 1 {
			cfg.Name = fmt.Sprintf("%s-%d", cfg.Name, nameCount[cfg.Name])
		}

		configs = append(configs, cfg)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("engine: no valid URIs found")
	}
	return configs, nil
}

// ParseAndValidateURI parses a URI and validates that the resolved provider
// has a registered factory. Returns early errors before engine creation.
func ParseAndValidateURI(raw string) (EngineConfig, error) {
	cfg, err := ParseURI(raw)
	if err != nil {
		return cfg, err
	}
	if _, ok := GetFactory(cfg.Provider); !ok {
		return cfg, fmt.Errorf("engine: no factory registered for provider %q (resolved from %s); available: %v", cfg.Provider, RedactURI(raw), RegisteredFactories())
	}
	return cfg, nil
}

// NewFromURI creates an Engine from a URI string using registered factories.
func NewFromURI(uri string) (Engine, error) {
	cfg, err := ParseAndValidateURI(uri)
	if err != nil {
		return nil, err
	}
	factory, _ := GetFactory(cfg.Provider)
	return factory(cfg)
}

// NewFromEnv creates engines from the ENGINE_URIS environment variable.
// Returns nil configs if the env var is not set.
func NewFromEnv() ([]EngineConfig, error) {
	uris := os.Getenv(EnvEngineURIs)
	if uris == "" {
		return nil, nil
	}
	return ParseURIs(uris)
}

// RedactURI returns a redacted version of a URI string with the API key masked.
// Useful for logging and error messages.
//
// Example: "dashscope://sk-abc123@host/path" → "dashscope://sk-***@host/path"
func RedactURI(raw string) string {
	raw = strings.TrimSpace(raw)
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return raw
	}

	scheme := raw[:schemeEnd]
	rest := raw[schemeEnd+3:]

	// Split query
	var queryStr string
	if qIdx := strings.IndexByte(rest, '?'); qIdx >= 0 {
		queryStr = rest[qIdx:]
		rest = rest[:qIdx]
	}

	// Redact userinfo
	if atIdx := strings.LastIndexByte(rest, '@'); atIdx >= 0 {
		userInfo := rest[:atIdx]
		hostPath := rest[atIdx:]
		rest = redactKey(userInfo) + hostPath
	} else {
		rest = redactKey(rest)
	}

	return scheme + "://" + rest + queryStr
}

// RedactURIs redacts all URIs in a comma-separated list.
func RedactURIs(raw string) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		parts[i] = RedactURI(p)
	}
	return strings.Join(parts, ",")
}

// ToURI converts an EngineConfig back into a URI string.
// This is useful for serialization, display, or generating config examples.
func ToURI(cfg EngineConfig) string {
	// Determine scheme (use provider directly; aliases are one-way)
	scheme := cfg.Provider

	var b strings.Builder
	b.WriteString(scheme)
	b.WriteString("://")

	// APIKey (+ optional secretKey)
	b.WriteString(cfg.APIKey)
	if cfg.Metadata != nil {
		if sk := cfg.Metadata["secretKey"]; sk != "" {
			b.WriteByte(':')
			b.WriteString(sk)
		}
	}

	// BaseURL → host+path portion
	if cfg.BaseURL != "" {
		b.WriteByte('@')
		hostPath := cfg.BaseURL
		hostPath = strings.TrimPrefix(hostPath, "https://")
		hostPath = strings.TrimPrefix(hostPath, "http://")
		b.WriteString(hostPath)
	}

	// Query params
	params := url.Values{}
	if cfg.Model != "" {
		params.Set("model", cfg.Model)
	}
	if cfg.Quality != "" {
		params.Set("quality", cfg.Quality)
	}
	if cfg.Style != "" {
		params.Set("style", cfg.Style)
	}
	if cfg.Background != "" {
		params.Set("background", cfg.Background)
	}
	if cfg.OutputFormat != "" {
		params.Set("output_format", cfg.OutputFormat)
	}
	if cfg.Moderation != "" {
		params.Set("moderation", cfg.Moderation)
	}
	if cfg.OutputCompression > 0 {
		params.Set("output_compression", fmt.Sprintf("%d", cfg.OutputCompression))
	}
	if cfg.Name != "" && cfg.Name != cfg.Provider && cfg.Name != cfg.Provider+"-"+sanitizeName(cfg.Model) {
		params.Set("name", cfg.Name)
	}
	if cfg.WaitForCompletion != nil {
		if *cfg.WaitForCompletion {
			params.Set("wait", "true")
		} else {
			params.Set("wait", "false")
		}
	}
	if cfg.PollInterval > 0 {
		params.Set("poll_interval", cfg.PollInterval.String())
	}
	if cfg.Capability != "" {
		params.Set("capability", cfg.Capability)
	}
	if cfg.BaseURL != "" && strings.HasPrefix(cfg.BaseURL, "http://") && !isLocalHost(strings.TrimPrefix(cfg.BaseURL, "http://")) {
		params.Set("scheme", "http")
	}
	// Metadata (except secretKey which is in userinfo)
	if cfg.Metadata != nil {
		for k, v := range cfg.Metadata {
			if k == "secretKey" {
				continue
			}
			params.Set(k, v)
		}
	}

	if len(params) > 0 {
		b.WriteByte('?')
		b.WriteString(params.Encode())
	}

	return b.String()
}

func redactKey(key string) string {
	// Handle AK:SK format
	if colonIdx := strings.IndexByte(key, ':'); colonIdx >= 0 {
		return redactSingle(key[:colonIdx]) + ":" + redactSingle(key[colonIdx+1:])
	}
	return redactSingle(key)
}

func redactSingle(s string) string {
	if len(s) <= 4 {
		return "***"
	}
	// Show first 3 chars + "***"
	return s[:3] + "***"
}

var (
	aliasMu sync.RWMutex
	// schemeAliases maps user-friendly scheme names to registered factory keys.
	schemeAliases = map[string]string{
		"dashscope":  "alibabacloud",
		"tongyi":     "alibabacloud",
		"bailian":    "alibabacloud",
		"doubao":     "ark",
		"volcengine": "ark",
		"dall-e":     "openai",
		"dalle":      "openai",
	}
)

// RegisterSchemeAlias adds an alias that maps to a registered factory provider key.
// Safe for concurrent use.
func RegisterSchemeAlias(alias, provider string) {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	schemeAliases[strings.ToLower(alias)] = strings.ToLower(provider)
}

// resolveScheme returns the canonical provider key for a scheme,
// checking aliases if the scheme is not directly registered as a factory.
func resolveScheme(scheme string) string {
	if _, ok := GetFactory(scheme); ok {
		return scheme
	}
	aliasMu.RLock()
	canonical, ok := schemeAliases[scheme]
	aliasMu.RUnlock()
	if ok {
		return canonical
	}
	return scheme
}

var knownParams = map[string]bool{
	"base_url":           true,
	"model":              true,
	"quality":            true,
	"style":              true,
	"background":         true,
	"output_format":      true,
	"moderation":         true,
	"output_compression": true,
	"name":               true,
	"wait":               true,
	"poll_interval":      true,
	"capability":         true,
	"scheme":             true,
}

func applyParams(cfg *EngineConfig, params url.Values) {
	if v := params.Get("base_url"); v != "" {
		cfg.BaseURL = v
	}
	if v := params.Get("model"); v != "" {
		cfg.Model = v
	}
	if v := params.Get("quality"); v != "" {
		cfg.Quality = v
	}
	if v := params.Get("style"); v != "" {
		cfg.Style = v
	}
	if v := params.Get("background"); v != "" {
		cfg.Background = v
	}
	if v := params.Get("output_format"); v != "" {
		cfg.OutputFormat = v
	}
	if v := params.Get("moderation"); v != "" {
		cfg.Moderation = v
	}
	if v := params.Get("output_compression"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.OutputCompression = n
		}
	}
	if v := params.Get("name"); v != "" {
		cfg.Name = v
	}
	if v := params.Get("wait"); v != "" {
		b := v == "true" || v == "1" || v == "yes"
		cfg.WaitForCompletion = &b
	}
	if v := params.Get("poll_interval"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if v := params.Get("capability"); v != "" {
		cfg.Capability = v
	}

	// Remaining params → Metadata
	for key, vals := range params {
		if knownParams[key] || len(vals) == 0 {
			continue
		}
		if cfg.Metadata == nil {
			cfg.Metadata = make(map[string]string)
		}
		cfg.Metadata[key] = vals[0]
	}
}

func isLocalHost(hostPath string) bool {
	host := hostPath
	if slashIdx := strings.IndexByte(host, '/'); slashIdx >= 0 {
		host = host[:slashIdx]
	}
	if colonIdx := strings.IndexByte(host, ':'); colonIdx >= 0 {
		host = host[:colonIdx]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]"
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(s, "-")
}
