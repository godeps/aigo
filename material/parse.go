package material

import (
	"fmt"
	"net/url"
	"strings"
)

// URI scheme 格式:
//
//   pexels://<api_key>
//   unsplash://<access_key>
//   pixabay://<api_key>
//   oss://<access_key_id>:<access_key_secret>@<bucket>.<region>?mode=semantic&token=<sts_token>
//   local://<index_path>?embed=dashscope&embed_key=<api_key>
//
// 多后端聚合通过逗号分割:
//   pexels://KEY1,unsplash://KEY2,pixabay://KEY3

// ParsedURI holds the parsed components of a search backend URI.
type ParsedURI struct {
	Scheme string // "pexels", "unsplash", "pixabay", "oss", "local"

	// Common auth
	APIKey string // primary key (api_key, access_key)
	Secret string // secondary secret (oss access_key_secret)

	// OSS specific
	Bucket        string
	Region        string
	Mode          string // "basic" or "semantic"
	SecurityToken string

	// Local specific
	IndexPath    string
	EmbedBackend string // "dashscope", "jina", "openai", "voyage", "gemini"
	EmbedKey     string // API key for embedding engine
	EmbedModel   string // optional model override

	// Generic
	Params url.Values // all query parameters
}

// ParseURI parses a single search backend URI into its components.
func ParseURI(raw string) (ParsedURI, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedURI{}, fmt.Errorf("search: empty URI")
	}

	// Handle scheme extraction manually since net/url may not parse custom schemes well
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return ParsedURI{}, fmt.Errorf("search: invalid URI (no scheme): %q", raw)
	}

	scheme := strings.ToLower(raw[:schemeEnd])
	rest := raw[schemeEnd+3:] // after "://"

	parsed := ParsedURI{Scheme: scheme}

	switch scheme {
	case "pexels", "unsplash", "pixabay":
		return parseSimpleKeyURI(parsed, rest)
	case "oss":
		return parseOSSURI(parsed, rest)
	case "local":
		return parseLocalURI(parsed, rest)
	default:
		return ParsedURI{}, fmt.Errorf("search: unsupported scheme %q", scheme)
	}
}

// ParseURIs parses a comma-separated list of search backend URIs.
func ParseURIs(raw string) ([]ParsedURI, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("search: empty URI list")
	}

	parts := strings.Split(raw, ",")
	results := make([]ParsedURI, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := ParseURI(part)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("search: no valid URIs found")
	}
	return results, nil
}

// pexels://API_KEY
// pexels://API_KEY?param=value
func parseSimpleKeyURI(p ParsedURI, rest string) (ParsedURI, error) {
	queryIdx := strings.IndexByte(rest, '?')
	if queryIdx >= 0 {
		params, err := url.ParseQuery(rest[queryIdx+1:])
		if err != nil {
			return p, fmt.Errorf("search: parse query params: %w", err)
		}
		p.Params = params
		rest = rest[:queryIdx]
	}

	p.APIKey = strings.TrimSpace(rest)
	if p.APIKey == "" {
		return p, fmt.Errorf("search: %s URI missing API key", p.Scheme)
	}
	return p, nil
}

// oss://ACCESS_KEY_ID:ACCESS_KEY_SECRET@BUCKET.REGION?mode=semantic&token=STS_TOKEN
func parseOSSURI(p ParsedURI, rest string) (ParsedURI, error) {
	queryIdx := strings.IndexByte(rest, '?')
	if queryIdx >= 0 {
		params, err := url.ParseQuery(rest[queryIdx+1:])
		if err != nil {
			return p, fmt.Errorf("search: parse OSS query params: %w", err)
		}
		p.Params = params
		p.Mode = params.Get("mode")
		p.SecurityToken = params.Get("token")
		rest = rest[:queryIdx]
	}

	// Split userinfo@host
	atIdx := strings.LastIndexByte(rest, '@')
	if atIdx < 0 {
		return p, fmt.Errorf("search: oss URI missing credentials (expected oss://KEY_ID:KEY_SECRET@BUCKET.REGION)")
	}

	userInfo := rest[:atIdx]
	hostPart := rest[atIdx+1:]

	// Parse credentials: KEY_ID:KEY_SECRET
	colonIdx := strings.IndexByte(userInfo, ':')
	if colonIdx < 0 {
		return p, fmt.Errorf("search: oss URI missing secret (expected KEY_ID:KEY_SECRET)")
	}
	p.APIKey = userInfo[:colonIdx]
	p.Secret = userInfo[colonIdx+1:]

	// Parse host: BUCKET.REGION (e.g. my-bucket.cn-hangzhou)
	dotIdx := strings.IndexByte(hostPart, '.')
	if dotIdx < 0 {
		return p, fmt.Errorf("search: oss URI missing region (expected BUCKET.REGION)")
	}
	p.Bucket = hostPart[:dotIdx]
	p.Region = hostPart[dotIdx+1:]

	if p.APIKey == "" || p.Secret == "" || p.Bucket == "" || p.Region == "" {
		return p, fmt.Errorf("search: oss URI incomplete (need KEY_ID:KEY_SECRET@BUCKET.REGION)")
	}

	if p.Mode == "" {
		p.Mode = "semantic"
	}
	return p, nil
}

// local:///path/to/index?embed=dashscope&embed_key=KEY&embed_model=MODEL
// local://./relative/path?embed=jina&embed_key=KEY
func parseLocalURI(p ParsedURI, rest string) (ParsedURI, error) {
	queryIdx := strings.IndexByte(rest, '?')
	if queryIdx >= 0 {
		params, err := url.ParseQuery(rest[queryIdx+1:])
		if err != nil {
			return p, fmt.Errorf("search: parse local query params: %w", err)
		}
		p.Params = params
		p.EmbedBackend = params.Get("embed")
		p.EmbedKey = params.Get("embed_key")
		p.EmbedModel = params.Get("embed_model")
		rest = rest[:queryIdx]
	}

	p.IndexPath = rest
	if p.IndexPath == "" {
		p.IndexPath = ".aigo/search_index.json"
	}

	if p.EmbedBackend == "" {
		p.EmbedBackend = "dashscope"
	}
	return p, nil
}
