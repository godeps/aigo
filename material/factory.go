package material

import (
	"fmt"
	"os"
	"strings"
)

// Registry maps scheme names to searcher constructors.
// Backends register themselves so the factory can build searchers from URIs
// without search/ importing all backend packages directly.
var registry = map[string]Constructor{}

// Constructor builds a Searcher from a parsed URI.
type Constructor func(parsed ParsedURI) (Searcher, error)

// Register adds a backend constructor for the given scheme.
func Register(scheme string, ctor Constructor) {
	registry[scheme] = ctor
}

// NewSearcher creates a single Searcher from a URI string.
func NewSearcher(uri string) (Searcher, error) {
	parsed, err := ParseURI(uri)
	if err != nil {
		return nil, err
	}
	return buildSearcher(parsed)
}

// NewFromURIs creates a MultiSearcher from a comma-separated URI list.
// Each URI configures one backend; all backends are aggregated into a single searcher.
//
// Example:
//
//	searcher, err := search.NewFromURIs("pexels://KEY1,unsplash://KEY2,oss://AK:SK@bucket.cn-hangzhou")
func NewFromURIs(uris string) (*MultiSearcher, error) {
	parsed, err := ParseURIs(uris)
	if err != nil {
		return nil, err
	}

	backends := make([]Searcher, 0, len(parsed))
	for _, p := range parsed {
		s, err := buildSearcher(p)
		if err != nil {
			return nil, err
		}
		backends = append(backends, s)
	}

	return NewMultiSearcher(backends...), nil
}

// Environment variable names for URI-based configuration.
const (
	// EnvMaterialURIs is the combined URI list env var.
	// Format: comma-separated URIs, e.g. "pexels://KEY,unsplash://KEY,oss://AK:SK@bucket.region"
	EnvMaterialURIs = "MATERIAL_URIS"

	// Per-backend URI env vars.
	EnvPexelsURI   = "PEXELS_URI"
	EnvUnsplashURI = "UNSPLASH_URI"
	EnvPixabayURI  = "PIXABAY_URI"
	EnvOSSURI      = "OSS_META_URI"
	EnvLocalURI    = "LOCAL_MATERIAL_URI"
)

// NewFromEnv creates a MultiSearcher from environment variables.
//
// Resolution order:
//  1. MATERIAL_URIS — if set, parsed as comma-separated URI list (takes precedence)
//  2. Per-backend env vars: PEXELS_URI, UNSPLASH_URI, PIXABAY_URI, OSS_META_URI, LOCAL_MATERIAL_URI
//
// Returns nil if no env vars are configured.
func NewFromEnv() (*MultiSearcher, error) {
	if uris := os.Getenv(EnvMaterialURIs); uris != "" {
		return NewFromURIs(uris)
	}

	var parts []string
	for _, env := range []string{EnvPexelsURI, EnvUnsplashURI, EnvPixabayURI, EnvOSSURI, EnvLocalURI} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("material: no URI env vars configured (set %s or individual %s / %s / %s / %s / %s)",
			EnvMaterialURIs, EnvPexelsURI, EnvUnsplashURI, EnvPixabayURI, EnvOSSURI, EnvLocalURI)
	}

	return NewFromURIs(strings.Join(parts, ","))
}

func buildSearcher(p ParsedURI) (Searcher, error) {
	ctor, ok := registry[p.Scheme]
	if !ok {
		return nil, fmt.Errorf("search: no registered constructor for scheme %q (available: %v)", p.Scheme, registeredSchemes())
	}
	return ctor(p)
}

func registeredSchemes() []string {
	schemes := make([]string, 0, len(registry))
	for s := range registry {
		schemes = append(schemes, s)
	}
	return schemes
}
