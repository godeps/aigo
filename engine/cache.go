package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/godeps/aigo/workflow"
)

type cacheEntry struct {
	result    Result
	createdAt time.Time
}

// Cache wraps an Engine and caches successful results keyed by graph content.
type Cache struct {
	engine  Engine
	mu      sync.RWMutex
	store   map[string]cacheEntry
	order   []string // LRU order (oldest first)
	ttl     time.Duration
	maxSize int
}

// WithCache wraps an Engine with a result cache.
// ttl controls how long results are cached; maxSize limits the number of entries.
func WithCache(e Engine, ttl time.Duration, maxSize int) *Cache {
	if maxSize <= 0 {
		maxSize = 100
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Cache{
		engine:  e,
		store:   make(map[string]cacheEntry, maxSize),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Execute checks the cache before delegating to the underlying engine.
func (c *Cache) Execute(ctx context.Context, g workflow.Graph) (Result, error) {
	key := c.hashGraph(g)

	// Check cache.
	c.mu.RLock()
	if entry, ok := c.store[key]; ok && time.Since(entry.createdAt) < c.ttl {
		c.mu.RUnlock()
		return entry.result, nil
	}
	c.mu.RUnlock()

	// Cache miss — execute.
	result, err := c.engine.Execute(ctx, g)
	if err != nil {
		return result, err
	}

	// Store result.
	c.mu.Lock()
	c.evictLocked()
	c.store[key] = cacheEntry{result: result, createdAt: time.Now()}
	c.order = append(c.order, key)
	c.mu.Unlock()

	return result, nil
}

func (c *Cache) evictLocked() {
	// Remove expired entries.
	now := time.Now()
	for k, v := range c.store {
		if now.Sub(v.createdAt) >= c.ttl {
			delete(c.store, k)
		}
	}

	// Rebuild order (remove deleted keys).
	if len(c.store) < len(c.order) {
		filtered := c.order[:0]
		for _, k := range c.order {
			if _, ok := c.store[k]; ok {
				filtered = append(filtered, k)
			}
		}
		c.order = filtered
	}

	// LRU eviction if still over limit.
	for len(c.store) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.store, oldest)
	}
}

func (c *Cache) hashGraph(g workflow.Graph) string {
	data, _ := json.Marshal(g)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

// Clear removes all cached entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]cacheEntry, c.maxSize)
	c.order = c.order[:0]
}
