package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	ExpiresAt   time.Time   `json:"expires_at"`
	CreatedAt   time.Time   `json:"created_at"`
	AccessCount int64       `json:"access_count"`
	LastAccess  time.Time   `json:"last_access"`
	Size        int64       `json:"size"`
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Touch updates the last access time and increments access count
func (e *CacheEntry) Touch() {
	e.LastAccess = time.Now()
	e.AccessCount++
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	MaxSize         int64         // Maximum cache size in bytes
	DefaultTTL      time.Duration // Default time-to-live for cache entries
	CleanupInterval time.Duration // How often to run cleanup
	MaxEntries      int           // Maximum number of entries
}

// DefaultCacheConfig returns a sensible default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         100 * 1024 * 1024, // 100MB
		DefaultTTL:      30 * time.Minute,  // 30 minutes
		CleanupInterval: 5 * time.Minute,   // 5 minutes
		MaxEntries:      10000,             // 10k entries
	}
}

// Cache represents an in-memory cache with TTL and size limits
type Cache struct {
	config    *CacheConfig
	entries   map[string]*CacheEntry
	mutex     sync.RWMutex
	totalSize int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewCache creates a new cache instance
func NewCache(config *CacheConfig) *Cache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &Cache{
		config:  config,
		entries: make(map[string]*CacheEntry),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	cache.wg.Add(1)
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if entry.IsExpired() {
		// Don't return expired entries, but don't delete here to avoid write lock
		return nil, false
	}

	entry.Touch()
	return entry.Value, true
}

// Set stores a value in the cache with default TTL
func (c *Cache) Set(key string, value interface{}) error {
	return c.SetWithTTL(key, value, c.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with custom TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate size of the entry
	size, err := c.calculateSize(value)
	if err != nil {
		return fmt.Errorf("failed to calculate entry size: %w", err)
	}

	// Check if we need to make space
	if err := c.makeSpace(size); err != nil {
		return fmt.Errorf("failed to make space for cache entry: %w", err)
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		ExpiresAt:   now.Add(ttl),
		CreatedAt:   now,
		LastAccess:  now,
		AccessCount: 0,
		Size:        size,
	}

	// Remove existing entry if it exists
	if existing, exists := c.entries[key]; exists {
		c.totalSize -= existing.Size
	}

	c.entries[key] = entry
	c.totalSize += size

	return nil
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.totalSize -= entry.Size
		delete(c.entries, key)
	}
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.totalSize = 0
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := CacheStats{
		Entries:    len(c.entries),
		TotalSize:  c.totalSize,
		MaxSize:    c.config.MaxSize,
		MaxEntries: c.config.MaxEntries,
	}

	// Calculate hit rate and other metrics
	var totalAccess int64
	var expiredEntries int

	for _, entry := range c.entries {
		totalAccess += entry.AccessCount
		if entry.IsExpired() {
			expiredEntries++
		}
	}

	stats.TotalAccess = totalAccess
	stats.ExpiredEntries = expiredEntries

	return stats
}

// Close stops the cache and cleanup goroutines
func (c *Cache) Close() {
	close(c.stopCh)
	c.wg.Wait()
}

// makeSpace ensures there's enough space for a new entry
func (c *Cache) makeSpace(requiredSize int64) error {
	// Check if entry is too large for cache
	if requiredSize > c.config.MaxSize {
		return fmt.Errorf("entry size %d exceeds maximum cache size %d", requiredSize, c.config.MaxSize)
	}

	// Remove expired entries first
	c.removeExpiredEntries()

	// If still not enough space, remove least recently used entries
	for c.totalSize+requiredSize > c.config.MaxSize || len(c.entries) >= c.config.MaxEntries {
		if len(c.entries) == 0 {
			break
		}

		// Find least recently used entry
		var lruKey string
		var lruTime time.Time
		first := true

		for key, entry := range c.entries {
			if first || entry.LastAccess.Before(lruTime) {
				lruKey = key
				lruTime = entry.LastAccess
				first = false
			}
		}

		// Remove LRU entry
		if lruEntry, exists := c.entries[lruKey]; exists {
			c.totalSize -= lruEntry.Size
			delete(c.entries, lruKey)
		}
	}

	return nil
}

// removeExpiredEntries removes all expired entries
func (c *Cache) removeExpiredEntries() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			c.totalSize -= entry.Size
			delete(c.entries, key)
		}
	}
}

// cleanupLoop runs periodic cleanup of expired entries
func (c *Cache) cleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mutex.Lock()
			c.removeExpiredEntries()
			c.mutex.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// calculateSize estimates the size of a value in bytes
func (c *Cache) calculateSize(value interface{}) (int64, error) {
	// Serialize to JSON to get approximate size
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return int64(len(data)), nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	Entries        int   `json:"entries"`
	TotalSize      int64 `json:"total_size"`
	MaxSize        int64 `json:"max_size"`
	MaxEntries     int   `json:"max_entries"`
	TotalAccess    int64 `json:"total_access"`
	ExpiredEntries int   `json:"expired_entries"`
}

// CacheKey generates a cache key from multiple components
func CacheKey(components ...string) string {
	hasher := sha256.New()
	for _, component := range components {
		hasher.Write([]byte(component))
		hasher.Write([]byte("|")) // separator
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// CacheKeyFromContext generates a cache key from context data
func CacheKeyFromContext(ctx *types.Context) string {
	components := []string{
		ctx.WorkingDirectory,
		fmt.Sprintf("%d", len(ctx.Files)),
	}

	if ctx.GitInfo != nil {
		components = append(components,
			ctx.GitInfo.CurrentBranch,
			ctx.GitInfo.WorkingTreeStatus,
			fmt.Sprintf("%t", ctx.GitInfo.HasUncommittedChanges),
		)
	}

	// Add environment variables that might affect commands
	for key, value := range ctx.Environment {
		components = append(components, fmt.Sprintf("%s=%s", key, value))
	}

	return CacheKey(components...)
}

// CacheKeyFromPrompt generates a cache key from prompt and context
func CacheKeyFromPrompt(prompt string, ctx *types.Context, provider string, model string) string {
	components := []string{
		prompt,
		provider,
		model,
		CacheKeyFromContext(ctx),
	}
	return CacheKey(components...)
}
