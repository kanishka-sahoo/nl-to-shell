package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager coordinates all caching operations
type Manager struct {
	contextCache   *ContextCache
	providerCache  *ProviderCache
	configCache    *ConfigCache
	metricsTracker *MetricsTracker
	config         *ManagerConfig
	mutex          sync.RWMutex
}

// ManagerConfig represents configuration for the cache manager
type ManagerConfig struct {
	Enabled           bool          `json:"enabled"`
	PersistentStorage bool          `json:"persistent_storage"`
	StoragePath       string        `json:"storage_path"`
	MaxTotalSize      int64         `json:"max_total_size"`
	GlobalTTL         time.Duration `json:"global_ttl"`
	ContextConfig     *CacheConfig  `json:"context_config"`
	ProviderConfig    *CacheConfig  `json:"provider_config"`
	ConfigConfig      *CacheConfig  `json:"config_config"`
}

// DefaultManagerConfig returns a default cache manager configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		Enabled:           true,
		PersistentStorage: false,             // Disabled by default for security
		MaxTotalSize:      500 * 1024 * 1024, // 500MB total
		GlobalTTL:         60 * time.Minute,
		ContextConfig:     DefaultCacheConfig(),
		ProviderConfig:    DefaultCacheConfig(),
		ConfigConfig:      DefaultCacheConfig(),
	}
}

// NewManager creates a new cache manager
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	// Adjust individual cache configs based on total size limit
	if config.MaxTotalSize > 0 {
		contextSize := config.MaxTotalSize / 4  // 25% for context
		providerSize := config.MaxTotalSize / 2 // 50% for provider responses
		configSize := config.MaxTotalSize / 4   // 25% for config

		if config.ContextConfig != nil {
			config.ContextConfig.MaxSize = contextSize
		}
		if config.ProviderConfig != nil {
			config.ProviderConfig.MaxSize = providerSize
		}
		if config.ConfigConfig != nil {
			config.ConfigConfig.MaxSize = configSize
		}
	}

	manager := &Manager{
		contextCache:   NewContextCacheWithConfig(config.ContextConfig),
		providerCache:  NewProviderCacheWithConfig(config.ProviderConfig),
		configCache:    NewConfigCacheWithConfig(config.ConfigConfig),
		metricsTracker: NewMetricsTracker(),
		config:         config,
	}

	// Load persistent cache if enabled
	if config.PersistentStorage && config.StoragePath != "" {
		if err := manager.loadPersistentCache(); err != nil {
			// Log error but don't fail - continue with empty cache
			fmt.Printf("Warning: Failed to load persistent cache: %v\n", err)
		}
	}

	return manager
}

// GetContextCache returns the context cache
func (m *Manager) GetContextCache() *ContextCache {
	return m.contextCache
}

// GetProviderCache returns the provider cache
func (m *Manager) GetProviderCache() *ProviderCache {
	return m.providerCache
}

// GetConfigCache returns the config cache
func (m *Manager) GetConfigCache() *ConfigCache {
	return m.configCache
}

// GetMetricsTracker returns the metrics tracker
func (m *Manager) GetMetricsTracker() *MetricsTracker {
	return m.metricsTracker
}

// IsEnabled returns whether caching is enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// ClearAll clears all caches
func (m *Manager) ClearAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.contextCache.Clear()
	m.providerCache.Clear()
	m.configCache.Clear()
	m.metricsTracker.Reset()
}

// GetStats returns combined statistics for all caches
func (m *Manager) GetStats() CombinedStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return CombinedStats{
		Context:  m.contextCache.Stats(),
		Provider: m.providerCache.Stats(),
		Config:   m.configCache.Stats(),
		Metrics:  m.metricsTracker.GetMetrics(),
	}
}

// Close closes all caches and saves persistent data if enabled
func (m *Manager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var err error

	// Save persistent cache if enabled
	if m.config.PersistentStorage && m.config.StoragePath != "" {
		if saveErr := m.savePersistentCache(); saveErr != nil {
			err = fmt.Errorf("failed to save persistent cache: %w", saveErr)
		}
	}

	// Close all caches
	m.contextCache.Close()
	m.providerCache.Close()
	m.configCache.Close()

	return err
}

// Invalidate invalidates cache entries based on criteria
func (m *Manager) Invalidate(criteria InvalidationCriteria) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if criteria.Directory != "" {
		m.contextCache.InvalidateDirectory(criteria.Directory)
	}

	if criteria.Provider != "" {
		m.providerCache.InvalidateProvider(criteria.Provider)
		if criteria.Model != "" {
			m.providerCache.InvalidateModel(criteria.Provider, criteria.Model)
		}
	}

	if criteria.ConfigKey != "" {
		m.configCache.InvalidateKey(criteria.ConfigKey)
	}

	if criteria.ClearAll {
		m.ClearAll()
	}
}

// loadPersistentCache loads cache data from persistent storage
func (m *Manager) loadPersistentCache() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("no storage path configured")
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(filepath.Dir(m.config.StoragePath), 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Check if cache file exists
	if _, err := os.Stat(m.config.StoragePath); os.IsNotExist(err) {
		return nil // No cache file to load
	}

	// Read cache file
	data, err := os.ReadFile(m.config.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	// Parse cache data
	var persistentData PersistentCacheData
	if err := json.Unmarshal(data, &persistentData); err != nil {
		return fmt.Errorf("failed to parse cache data: %w", err)
	}

	// Load data into caches (implementation would depend on specific requirements)
	// For security reasons, we might only want to cache certain types of data persistently

	return nil
}

// savePersistentCache saves cache data to persistent storage
func (m *Manager) savePersistentCache() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("no storage path configured")
	}

	// Prepare data for persistence (only non-sensitive data)
	persistentData := PersistentCacheData{
		Timestamp: time.Now(),
		Stats:     m.GetStats(),
		// Note: We don't persist actual cache entries for security reasons
		// Only metadata and statistics
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(persistentData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(filepath.Dir(m.config.StoragePath), 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.config.StoragePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// ConfigCache manages caching for configuration data
type ConfigCache struct {
	cache *Cache
}

// NewConfigCache creates a new config cache
func NewConfigCache() *ConfigCache {
	config := &CacheConfig{
		MaxSize:         10 * 1024 * 1024, // 10MB for config data
		DefaultTTL:      60 * time.Minute, // Config is relatively stable
		CleanupInterval: 15 * time.Minute,
		MaxEntries:      500,
	}

	return &ConfigCache{
		cache: NewCache(config),
	}
}

// NewConfigCacheWithConfig creates a new config cache with custom config
func NewConfigCacheWithConfig(config *CacheConfig) *ConfigCache {
	return &ConfigCache{
		cache: NewCache(config),
	}
}

// Get retrieves a cached configuration value
func (cc *ConfigCache) Get(key string) (interface{}, bool) {
	return cc.cache.Get(key)
}

// Set stores a configuration value in cache
func (cc *ConfigCache) Set(key string, value interface{}) error {
	return cc.cache.Set(key, value)
}

// SetWithTTL stores a configuration value with custom TTL
func (cc *ConfigCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	return cc.cache.SetWithTTL(key, value, ttl)
}

// InvalidateKey invalidates a specific configuration key
func (cc *ConfigCache) InvalidateKey(key string) {
	cc.cache.Delete(key)
}

// Clear clears all cached configuration data
func (cc *ConfigCache) Clear() {
	cc.cache.Clear()
}

// Stats returns cache statistics
func (cc *ConfigCache) Stats() CacheStats {
	return cc.cache.Stats()
}

// Close closes the config cache
func (cc *ConfigCache) Close() {
	cc.cache.Close()
}

// InvalidationCriteria represents criteria for cache invalidation
type InvalidationCriteria struct {
	Directory string
	Provider  string
	Model     string
	ConfigKey string
	ClearAll  bool
}

// CombinedStats represents statistics from all caches
type CombinedStats struct {
	Context  CacheStats           `json:"context"`
	Provider CacheStats           `json:"provider"`
	Config   CacheStats           `json:"config"`
	Metrics  ProviderCacheMetrics `json:"metrics"`
}

// PersistentCacheData represents data that can be persisted to disk
type PersistentCacheData struct {
	Timestamp time.Time     `json:"timestamp"`
	Stats     CombinedStats `json:"stats"`
	// Note: Actual cache entries are not persisted for security reasons
}

// GetTotalSize returns the total size of all caches
func (cs *CombinedStats) GetTotalSize() int64 {
	return cs.Context.TotalSize + cs.Provider.TotalSize + cs.Config.TotalSize
}

// GetTotalEntries returns the total number of entries across all caches
func (cs *CombinedStats) GetTotalEntries() int {
	return cs.Context.Entries + cs.Provider.Entries + cs.Config.Entries
}
