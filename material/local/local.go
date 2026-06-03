// Package local implements the material.Searcher interface using local vector
// embeddings for similarity search over a user's own asset library.
package local

import (
	"context"
	"container/heap"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	Size      int64             `json:"size,omitempty"`
	ModTime   int64             `json:"mod_time,omitempty"` // unix timestamp for change detection
	Vector    []float32         `json:"vector"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Searcher performs vector similarity search over locally indexed assets.
type Searcher struct {
	embedEngine embed.EmbedEngine
	indexPath   string
	mu          sync.RWMutex
	entries     []IndexEntry
	pathIndex   map[string]int // path → entries slice index (dedup + fast lookup)
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
		pathIndex:   make(map[string]int),
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

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	// Use a min-heap approach for large indexes: only keep top-K candidates.
	candidates := topK(entries, queryResult.Vector, req.MediaTypes, maxResults)

	items := make([]material.Item, 0, len(candidates))
	for _, c := range candidates {
		items = append(items, material.Item{
			URI:       c.entry.Path,
			Filename:  filepath.Base(c.entry.Path),
			Size:      c.entry.Size,
			MediaType: c.entry.MediaType,
			Source:    "local",
			Score:     c.score,
			Metadata:  c.entry.Metadata,
		})
	}

	return material.Result{
		Items:  items,
		Total:  len(entries),
		Source: "local",
	}, nil
}

// IndexFile adds or updates a single file in the vector index.
// If the file is already indexed and unchanged (same size + mod time), it is skipped.
func (s *Searcher) IndexFile(ctx context.Context, path string) error {
	absPath, _ := filepath.Abs(path)

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("local search: stat %s: %w", path, err)
	}

	// Check if already indexed and unchanged.
	s.mu.RLock()
	if idx, ok := s.pathIndex[absPath]; ok {
		existing := s.entries[idx]
		if existing.Size == info.Size() && existing.ModTime == info.ModTime().Unix() {
			s.mu.RUnlock()
			return nil // already indexed, unchanged
		}
	}
	s.mu.RUnlock()

	mediaType := guessMediaType(absPath)

	var req embed.Request
	switch mediaType {
	case "image":
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("local search: read file %s: %w", absPath, err)
		}
		req = embed.ImageRequest(data, "RETRIEVAL_DOCUMENT")
	default:
		req = embed.TextRequest(filepath.Base(absPath), "RETRIEVAL_DOCUMENT")
	}

	result, err := s.embedEngine.Embed(ctx, req)
	if err != nil {
		return fmt.Errorf("local search: embed %s: %w", absPath, err)
	}

	entry := IndexEntry{
		Path:      absPath,
		MediaType: mediaType,
		Size:      info.Size(),
		ModTime:   info.ModTime().Unix(),
		Vector:    result.Vector,
	}

	s.mu.Lock()
	if idx, ok := s.pathIndex[absPath]; ok {
		s.entries[idx] = entry // update in place
	} else {
		s.pathIndex[absPath] = len(s.entries)
		s.entries = append(s.entries, entry)
	}
	s.mu.Unlock()

	return nil
}

// IndexDir scans a directory and indexes all supported files incrementally.
// Files already indexed with the same size and mod time are skipped.
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
		if ctx.Err() != nil {
			return ctx.Err()
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

// RemoveStale removes index entries whose files no longer exist on disk.
func (s *Searcher) RemoveStale() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []IndexEntry
	newIndex := make(map[string]int)
	removed := 0

	for _, e := range s.entries {
		if _, err := os.Stat(e.Path); err != nil {
			removed++
			continue
		}
		newIndex[e.Path] = len(kept)
		kept = append(kept, e)
	}

	s.entries = kept
	s.pathIndex = newIndex
	return removed
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
	s.pathIndex = make(map[string]int, len(entries))
	for i, e := range entries {
		s.pathIndex[e.Path] = i
	}
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

// scored pairs an entry with its similarity score.
type scored struct {
	entry IndexEntry
	score float64
}

// minHeap implements heap.Interface for scored items (min by score).
type minHeap []scored

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool   { return h[i].score < h[j].score }
func (h minHeap) Swap(i, j int)        { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)          { *h = append(*h, x.(scored)) }
func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// topK returns the top-K entries by cosine similarity in O(n log k) time.
func topK(entries []IndexEntry, queryVec []float32, mediaTypes []string, k int) []scored {
	h := &minHeap{}
	heap.Init(h)

	for _, e := range entries {
		if len(mediaTypes) > 0 && !containsMediaType(mediaTypes, e.MediaType) {
			continue
		}
		score := cosineSimilarity(queryVec, e.Vector)
		if h.Len() < k {
			heap.Push(h, scored{entry: e, score: score})
		} else if score > (*h)[0].score {
			(*h)[0] = scored{entry: e, score: score}
			heap.Fix(h, 0)
		}
	}

	// Extract in descending score order.
	result := make([]scored, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(scored)
	}
	return result
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
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif", ".tiff",
		".mp4", ".mov", ".avi", ".mkv",
		".mp3", ".wav", ".flac", ".aac",
		".pdf", ".doc", ".docx", ".txt":
		return true
	}
	return false
}

var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
