package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// ProviderCache manages caching for LLM provider responses
type ProviderCache struct {
	cache *Cache
}

// NewProviderCache creates a new provider cache
func NewProviderCache() *ProviderCache {
	config := &CacheConfig{
		MaxSize:         200 * 1024 * 1024, // 200MB for provider responses
		DefaultTTL:      60 * time.Minute,  // Provider responses are relatively stable
		CleanupInterval: 10 * time.Minute,
		MaxEntries:      5000,
	}

	return &ProviderCache{
		cache: NewCache(config),
	}
}

// NewProviderCacheWithConfig creates a new provider cache with custom config
func NewProviderCacheWithConfig(config *CacheConfig) *ProviderCache {
	return &ProviderCache{
		cache: NewCache(config),
	}
}

// GetCommandResponse retrieves cached command generation response
func (pc *ProviderCache) GetCommandResponse(prompt string, ctx *types.Context, provider, model string) (*types.CommandResponse, bool) {
	key := pc.commandResponseCacheKey(prompt, ctx, provider, model)

	if value, exists := pc.cache.Get(key); exists {
		if response, ok := value.(*types.CommandResponse); ok {
			return response, true
		}
	}

	return nil, false
}

// SetCommandResponse stores command generation response in cache
func (pc *ProviderCache) SetCommandResponse(prompt string, ctx *types.Context, provider, model string, response *types.CommandResponse) error {
	key := pc.commandResponseCacheKey(prompt, ctx, provider, model)

	// Use longer TTL for successful responses
	ttl := 60 * time.Minute
	if response.Confidence < 0.7 {
		// Shorter TTL for low-confidence responses
		ttl = 15 * time.Minute
	}

	return pc.cache.SetWithTTL(key, response, ttl)
}

// GetValidationResponse retrieves cached validation response
func (pc *ProviderCache) GetValidationResponse(command, output, intent, provider, model string) (*types.ValidationResponse, bool) {
	key := pc.validationResponseCacheKey(command, output, intent, provider, model)

	if value, exists := pc.cache.Get(key); exists {
		if response, ok := value.(*types.ValidationResponse); ok {
			return response, true
		}
	}

	return nil, false
}

// SetValidationResponse stores validation response in cache
func (pc *ProviderCache) SetValidationResponse(command, output, intent, provider, model string, response *types.ValidationResponse) error {
	key := pc.validationResponseCacheKey(command, output, intent, provider, model)

	// Validation responses are cached for a shorter time as they're more context-specific
	ttl := 30 * time.Minute
	if !response.IsCorrect {
		// Even shorter TTL for incorrect validations
		ttl = 10 * time.Minute
	}

	return pc.cache.SetWithTTL(key, response, ttl)
}

// InvalidateProvider invalidates all cache entries for a specific provider
func (pc *ProviderCache) InvalidateProvider(provider string) {
	pc.cache.mutex.Lock()
	defer pc.cache.mutex.Unlock()

	keysToDelete := make([]string, 0)
	for key := range pc.cache.entries {
		if strings.Contains(key, provider) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		if entry, exists := pc.cache.entries[key]; exists {
			pc.cache.totalSize -= entry.Size
			delete(pc.cache.entries, key)
		}
	}
}

// InvalidateModel invalidates all cache entries for a specific model
func (pc *ProviderCache) InvalidateModel(provider, model string) {
	pc.cache.mutex.Lock()
	defer pc.cache.mutex.Unlock()

	modelKey := fmt.Sprintf("%s:%s", provider, model)
	keysToDelete := make([]string, 0)
	for key := range pc.cache.entries {
		if strings.Contains(key, modelKey) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		if entry, exists := pc.cache.entries[key]; exists {
			pc.cache.totalSize -= entry.Size
			delete(pc.cache.entries, key)
		}
	}
}

// GetCacheHitRate returns the cache hit rate for monitoring
func (pc *ProviderCache) GetCacheHitRate() float64 {
	stats := pc.cache.Stats()
	if stats.TotalAccess == 0 {
		return 0.0
	}

	// This is a simplified calculation - in a production system,
	// you'd want to track hits and misses separately
	return float64(stats.TotalAccess) / float64(stats.TotalAccess+int64(stats.Entries))
}

// Clear clears all cached provider responses
func (pc *ProviderCache) Clear() {
	pc.cache.Clear()
}

// Stats returns cache statistics
func (pc *ProviderCache) Stats() CacheStats {
	return pc.cache.Stats()
}

// Close closes the provider cache
func (pc *ProviderCache) Close() {
	pc.cache.Close()
}

// commandResponseCacheKey generates a cache key for command responses
func (pc *ProviderCache) commandResponseCacheKey(prompt string, ctx *types.Context, provider, model string) string {
	return CacheKeyFromPrompt(prompt, ctx, provider, model)
}

// validationResponseCacheKey generates a cache key for validation responses
func (pc *ProviderCache) validationResponseCacheKey(command, output, intent, provider, model string) string {
	return CacheKey("validation", command, output, intent, provider, model)
}

// ProviderCacheMetrics represents metrics for provider cache performance
type ProviderCacheMetrics struct {
	CommandHits         int64         `json:"command_hits"`
	CommandMisses       int64         `json:"command_misses"`
	ValidationHits      int64         `json:"validation_hits"`
	ValidationMisses    int64         `json:"validation_misses"`
	TotalHits           int64         `json:"total_hits"`
	TotalMisses         int64         `json:"total_misses"`
	HitRate             float64       `json:"hit_rate"`
	AverageResponseTime time.Duration `json:"average_response_time"`
}

// MetricsTracker tracks cache performance metrics
type MetricsTracker struct {
	commandHits       int64
	commandMisses     int64
	validationHits    int64
	validationMisses  int64
	totalResponseTime time.Duration
	responseCount     int64
}

// NewMetricsTracker creates a new metrics tracker
func NewMetricsTracker() *MetricsTracker {
	return &MetricsTracker{}
}

// RecordCommandHit records a cache hit for command generation
func (mt *MetricsTracker) RecordCommandHit() {
	mt.commandHits++
}

// RecordCommandMiss records a cache miss for command generation
func (mt *MetricsTracker) RecordCommandMiss() {
	mt.commandMisses++
}

// RecordValidationHit records a cache hit for validation
func (mt *MetricsTracker) RecordValidationHit() {
	mt.validationHits++
}

// RecordValidationMiss records a cache miss for validation
func (mt *MetricsTracker) RecordValidationMiss() {
	mt.validationMisses++
}

// RecordResponseTime records the response time for a provider call
func (mt *MetricsTracker) RecordResponseTime(duration time.Duration) {
	mt.totalResponseTime += duration
	mt.responseCount++
}

// GetMetrics returns current cache metrics
func (mt *MetricsTracker) GetMetrics() ProviderCacheMetrics {
	totalHits := mt.commandHits + mt.validationHits
	totalMisses := mt.commandMisses + mt.validationMisses
	total := totalHits + totalMisses

	var hitRate float64
	if total > 0 {
		hitRate = float64(totalHits) / float64(total)
	}

	var avgResponseTime time.Duration
	if mt.responseCount > 0 {
		avgResponseTime = mt.totalResponseTime / time.Duration(mt.responseCount)
	}

	return ProviderCacheMetrics{
		CommandHits:         mt.commandHits,
		CommandMisses:       mt.commandMisses,
		ValidationHits:      mt.validationHits,
		ValidationMisses:    mt.validationMisses,
		TotalHits:           totalHits,
		TotalMisses:         totalMisses,
		HitRate:             hitRate,
		AverageResponseTime: avgResponseTime,
	}
}

// Reset resets all metrics
func (mt *MetricsTracker) Reset() {
	mt.commandHits = 0
	mt.commandMisses = 0
	mt.validationHits = 0
	mt.validationMisses = 0
	mt.totalResponseTime = 0
	mt.responseCount = 0
}
