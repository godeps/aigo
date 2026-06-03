// Package search provides a unified interface for material/asset search
// across multiple backends (stock platforms, OSS, local vector stores).
package material

import "context"

// Searcher searches for materials/assets from a specific backend.
type Searcher interface {
	Search(ctx context.Context, req Request) (Result, error)
}

// Request describes what to search for.
type Request struct {
	Query      string   // keyword or natural language description
	MediaTypes []string // filter: "image", "video", "audio", "document"
	Tags       []string // tag filters
	MaxResults int      // max items to return (1-100)
	Page       int      // pagination page number (1-based)
	Sort       string   // "relevance", "newest", "popular"
	Order      string   // "asc", "desc"
	NextToken  string   // cursor for continuation
	Locale     string   // "zh", "en", etc.

	// OSS MetaQuery basic mode fields
	FieldQuery  string // structured JSON query for scalar search
	SimpleQuery string // additional filter for semantic mode
}

// Result is the outcome of a search operation.
type Result struct {
	Items     []Item `json:"items"`
	Total     int    `json:"total"`
	NextToken string `json:"next_token,omitempty"`
	Source    string `json:"source"`
}

// Item represents a single search result.
type Item struct {
	ID          string            `json:"id"`
	URI         string            `json:"uri"`
	Filename    string            `json:"filename,omitempty"`
	PreviewURL  string            `json:"preview_url,omitempty"`
	DownloadURL string            `json:"download_url,omitempty"`
	Size        int64             `json:"size,omitempty"`
	MediaType   string            `json:"media_type"`
	ContentType string            `json:"content_type,omitempty"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	Duration    float64           `json:"duration,omitempty"`
	Source      string            `json:"source"`
	Author      string            `json:"author,omitempty"`
	License     string            `json:"license,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Score       float64           `json:"score,omitempty"`
	Caption     string            `json:"caption,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Describer is an optional interface for searchers to advertise capabilities.
type Describer interface {
	// Source returns the backend identifier (e.g. "pexels", "unsplash", "oss").
	Source() string
	// SupportedMediaTypes returns the media types this searcher can find.
	SupportedMediaTypes() []string
}
