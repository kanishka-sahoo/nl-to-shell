package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestCache_BasicOperations(t *testing.T) {
	cache := NewCache(DefaultCacheConfig())
	defer cache.Close()

	// Test Set and Get
	key := "test_key"
	value := "test_value"

	err := cache.Set(key, value)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	retrieved, exists := cache.Get(key)
	if !exists {
		t.Fatal("Cache entry should exist")
	}

	if retrieved != value {
		t.Fatalf("Expected %v, got %v", value, retrieved)
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	config := &CacheConfig{
		MaxSize:         1024 * 1024,
		DefaultTTL:      100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
		MaxEntries:      100,
	}
	cache := NewCache(config)
	defer cache.Close()

	key := "expiring_key"
	value := "expiring_value"

	// Set with short TTL
	err := cache.SetWithTTL(key, value, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	// Should exist immediately
	_, exists := cache.Get(key)
	if !exists {
		t.Fatal("Cache entry should exist immediately after setting")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not exist after expiration
	_, exists = cache.Get(key)
	if exists {
		t.Fatal("Cache entry should not exist after expiration")
	}
}

func TestCache_SizeLimit(t *testing.T) {
	config := &CacheConfig{
		MaxSize:         1024, // Very small size
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
		MaxEntries:      100,
	}
	cache := NewCache(config)
	defer cache.Close()

	// Add entries until we exceed size limit
	largeValue := make([]byte, 500)
	for i := 0; i < len(largeValue); i++ {
		largeValue[i] = byte(i % 256)
	}

	// First entry should succeed
	err := cache.Set("key1", largeValue)
	if err != nil {
		t.Fatalf("Failed to set first cache entry: %v", err)
	}

	// Second entry should succeed and evict first
	err = cache.Set("key2", largeValue)
	if err != nil {
		t.Fatalf("Failed to set second cache entry: %v", err)
	}

	// Third entry should succeed and evict second
	err = cache.Set("key3", largeValue)
	if err != nil {
		t.Fatalf("Failed to set third cache entry: %v", err)
	}

	// First entry should be evicted
	_, exists := cache.Get("key1")
	if exists {
		t.Fatal("First entry should have been evicted")
	}

	// Third entry should still exist
	_, exists = cache.Get("key3")
	if !exists {
		t.Fatal("Third entry should still exist")
	}
}

func TestCache_MaxEntries(t *testing.T) {
	config := &CacheConfig{
		MaxSize:         1024 * 1024,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
		MaxEntries:      3, // Very small entry limit
	}
	cache := NewCache(config)
	defer cache.Close()

	// Add entries up to limit
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		err := cache.Set(key, fmt.Sprintf("value%d", i))
		if err != nil {
			t.Fatalf("Failed to set cache entry %d: %v", i, err)
		}
	}

	stats := cache.Stats()
	if stats.Entries > 3 {
		t.Fatalf("Expected at most 3 entries, got %d", stats.Entries)
	}
}

func TestCache_Stats(t *testing.T) {
	cache := NewCache(DefaultCacheConfig())
	defer cache.Close()

	// Add some entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		err := cache.Set(key, fmt.Sprintf("value%d", i))
		if err != nil {
			t.Fatalf("Failed to set cache entry %d: %v", i, err)
		}
	}

	// Access some entries to increase access count
	cache.Get("key0")
	cache.Get("key1")
	cache.Get("key0") // Access key0 again

	stats := cache.Stats()
	if stats.Entries != 5 {
		t.Fatalf("Expected 5 entries, got %d", stats.Entries)
	}

	if stats.TotalAccess != 3 {
		t.Fatalf("Expected 3 total accesses, got %d", stats.TotalAccess)
	}

	if stats.TotalSize <= 0 {
		t.Fatal("Total size should be greater than 0")
	}
}

func TestCacheKey_Generation(t *testing.T) {
	// Test basic key generation
	key1 := CacheKey("component1", "component2", "component3")
	key2 := CacheKey("component1", "component2", "component3")
	key3 := CacheKey("component1", "component2", "different")

	if key1 != key2 {
		t.Fatal("Same components should generate same key")
	}

	if key1 == key3 {
		t.Fatal("Different components should generate different keys")
	}
}

func TestCacheKeyFromContext(t *testing.T) {
	ctx1 := &types.Context{
		WorkingDirectory: "/test/dir",
		Files: []types.FileInfo{
			{Name: "file1.txt", Path: "/test/dir/file1.txt"},
			{Name: "file2.txt", Path: "/test/dir/file2.txt"},
		},
		GitInfo: &types.GitContext{
			IsRepository:          true,
			CurrentBranch:         "main",
			WorkingTreeStatus:     "clean",
			HasUncommittedChanges: false,
		},
		Environment: map[string]string{
			"PATH": "/usr/bin:/bin",
			"HOME": "/home/user",
		},
	}

	ctx2 := &types.Context{
		WorkingDirectory: "/test/dir",
		Files: []types.FileInfo{
			{Name: "file1.txt", Path: "/test/dir/file1.txt"},
			{Name: "file2.txt", Path: "/test/dir/file2.txt"},
		},
		GitInfo: &types.GitContext{
			IsRepository:          true,
			CurrentBranch:         "main",
			WorkingTreeStatus:     "clean",
			HasUncommittedChanges: false,
		},
		Environment: map[string]string{
			"PATH": "/usr/bin:/bin",
			"HOME": "/home/user",
		},
	}

	ctx3 := &types.Context{
		WorkingDirectory: "/different/dir",
		Files: []types.FileInfo{
			{Name: "file1.txt", Path: "/different/dir/file1.txt"},
		},
		GitInfo: &types.GitContext{
			IsRepository:          true,
			CurrentBranch:         "develop",
			WorkingTreeStatus:     "dirty",
			HasUncommittedChanges: true,
		},
		Environment: map[string]string{
			"PATH": "/usr/bin:/bin",
			"HOME": "/home/user",
		},
	}

	key1 := CacheKeyFromContext(ctx1)
	key2 := CacheKeyFromContext(ctx2)
	key3 := CacheKeyFromContext(ctx3)

	if key1 != key2 {
		t.Fatal("Same contexts should generate same key")
	}

	if key1 == key3 {
		t.Fatal("Different contexts should generate different keys")
	}
}

func TestCacheKeyFromPrompt(t *testing.T) {
	ctx := &types.Context{
		WorkingDirectory: "/test/dir",
		Files:            []types.FileInfo{},
		Environment:      map[string]string{},
	}

	key1 := CacheKeyFromPrompt("list files", ctx, "openai", "gpt-4")
	key2 := CacheKeyFromPrompt("list files", ctx, "openai", "gpt-4")
	key3 := CacheKeyFromPrompt("list files", ctx, "anthropic", "claude-3")
	key4 := CacheKeyFromPrompt("different prompt", ctx, "openai", "gpt-4")

	if key1 != key2 {
		t.Fatal("Same prompt, context, provider, and model should generate same key")
	}

	if key1 == key3 {
		t.Fatal("Different provider should generate different key")
	}

	if key1 == key4 {
		t.Fatal("Different prompt should generate different key")
	}
}
