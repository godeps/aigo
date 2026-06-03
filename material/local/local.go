// Package local implements the material.Searcher interface using local vector
// embeddings for similarity search over a user's own asset library.
package local

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/godeps/aigo/engine/embed"
	"github.com/godeps/aigo/material"
)

func init() {
	material.Register("local", func(p material.ParsedURI) (material.Searcher, error) {
		return nil, fmt.Errorf("local search: use local.New() directly with an embed.EmbedEngine instance")
	})
}

// Config configures the local vector searcher.
type Config struct {
	// EmbedEngine is the embedding backend used for vectorizing queries and assets.
	EmbedEngine embed.EmbedEngine
	// IndexPath is where the vector index is persisted (JSON file).
	IndexPath string
}

// IndexEntry represents a single indexed asset with its vector embedding.
type IndexEntry struct {
	Path      string            `json:"path"`
	MediaType string            `json:"media_type"`
	Vector    []float32         `json:"vector"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Searcher performs vector similarity search over locally indexed assets.
type Searcher struct {
	embedEngine embed.EmbedEngine
	indexPath   string
	mu          sync.RWMutex
	entries     []IndexEntry
}

// New creates a local vector searcher.
func New(cfg Config) (*Searcher, error) {
	if cfg.EmbedEngine == nil {
		return nil, fmt.Errorf("local search: embed engine is required")
	}
	if cfg.IndexPath == "" {
		cfg.IndexPath = ".aigo/search_index.json"
	}

	s := &Searcher{
		embedEngine: cfg.EmbedEngine,
		indexPath:   cfg.IndexPath,
	}

	if err := s.loadIndex(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("local search: load index: %w", err)
	}
	return s, nil
}

func (s *Searcher) Source() string                { return "local" }
func (s *Searcher) SupportedMediaTypes() []string { return []string{"image", "video", "audio", "document"} }

// Search vectorizes the query and finds the most similar assets in the index.
func (s *Searcher) Search(ctx context.Context, req material.Request) (material.Result, error) {
	s.mu.RLock()
	entries := s.entries
	s.mu.RUnlock()

	if len(entries) == 0 {
		return material.Result{Source: "local"}, nil
	}

	queryResult, err := s.embedEngine.Embed(ctx, embed.TextRequest(req.Query, "RETRIEVAL_QUERY"))
	if err != nil {
		return material.Result{}, fmt.Errorf("local search: embed query: %w", err)
	}

	type scored struct {
		entry IndexEntry
		score float64
	}

	candidates := make([]scored, 0, len(entries))
	for _, e := range entries {
		if len(req.MediaTypes) > 0 && !containsMediaType(req.MediaTypes, e.MediaType) {
			continue
		}
		score := cosineSimilarity(queryResult.Vector, e.Vector)
		candidates = append(candidates, scored{entry: e, score: score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > len(candidates) {
		maxResults = len(candidates)
	}

	items := make([]material.Item, 0, maxResults)
	for i := 0; i < maxResults; i++ {
		c := candidates[i]
		items = append(items, material.Item{
			URI:       c.entry.Path,
			Filename:  filepath.Base(c.entry.Path),
			MediaType: c.entry.MediaType,
			Source:    "local",
			Score:     c.score,
			Metadata:  c.entry.Metadata,
		})
	}

	return material.Result{
		Items:  items,
		Total:  len(candidates),
		Source: "local",
	}, nil
}

// IndexFile adds a single file to the vector index.
func (s *Searcher) IndexFile(ctx context.Context, path string) error {
	mediaType := guessMediaType(path)

	var req embed.Request
	switch mediaType {
	case "image":
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("local search: read file %s: %w", path, err)
		}
		req = embed.ImageRequest(data, "RETRIEVAL_DOCUMENT")
	default:
		req = embed.TextRequest(filepath.Base(path), "RETRIEVAL_DOCUMENT")
	}

	result, err := s.embedEngine.Embed(ctx, req)
	if err != nil {
		return fmt.Errorf("local search: embed %s: %w", path, err)
	}

	absPath, _ := filepath.Abs(path)
	entry := IndexEntry{
		Path:      absPath,
		MediaType: mediaType,
		Vector:    result.Vector,
	}

	s.mu.Lock()
	s.entries = append(s.entries, entry)
	s.mu.Unlock()

	return nil
}

// IndexDir scans a directory and indexes all supported files.
func (s *Searcher) IndexDir(ctx context.Context, dir string) (int, error) {
	var count int
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !isSupportedFile(path) {
			return nil
		}
		if err := s.IndexFile(ctx, path); err != nil {
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}

	return count, s.saveIndex()
}

// SaveIndex persists the current index to disk.
func (s *Searcher) SaveIndex() error {
	return s.saveIndex()
}

// EntryCount returns the number of indexed assets.
func (s *Searcher) EntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *Searcher) loadIndex() error {
	data, err := os.ReadFile(s.indexPath)
	if err != nil {
		return err
	}
	var entries []IndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	s.mu.Lock()
	s.entries = entries
	s.mu.Unlock()
	return nil
}

func (s *Searcher) saveIndex() error {
	s.mu.RLock()
	entries := s.entries
	s.mu.RUnlock()

	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.indexPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.indexPath, data, 0o644)
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func containsMediaType(types []string, mt string) bool {
	for _, t := range types {
		if t == mt {
			return true
		}
	}
	return false
}

func guessMediaType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif", ".tiff":
		return "image"
	case ".mp4", ".mov", ".avi", ".mkv", ".wmv", ".flv":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma":
		return "audio"
	default:
		return "document"
	}
}

func isSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supported := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
		".bmp": true, ".gif": true, ".tiff": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".mp3": true, ".wav": true, ".flac": true, ".aac": true,
		".pdf": true, ".doc": true, ".docx": true, ".txt": true,
	}
	return supported[ext]
}

var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
