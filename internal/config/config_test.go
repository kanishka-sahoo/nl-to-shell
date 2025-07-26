package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestGetConfigDirectory(t *testing.T) {
	configDir, err := getConfigDirectory()
	if err != nil {
		t.Fatalf("getConfigDirectory() failed: %v", err)
	}

	if configDir == "" {
		t.Fatal("getConfigDirectory() returned empty string")
	}

	// Verify the path contains the app name
	if !filepath.IsAbs(configDir) {
		t.Errorf("getConfigDirectory() returned relative path: %s", configDir)
	}

	// Platform-specific checks
	switch runtime.GOOS {
	case "windows":
		if !filepath.IsAbs(configDir) {
			t.Errorf("Windows config directory should be absolute: %s", configDir)
		}
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		expectedPrefix := filepath.Join(homeDir, "Library", "Application Support")
		if !filepath.HasPrefix(configDir, expectedPrefix) {
			t.Errorf("macOS config directory should be under ~/Library/Application Support, got: %s", configDir)
		}
	default:
		homeDir, _ := os.UserHomeDir()
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			expectedPrefix := xdgConfigHome
			if !filepath.HasPrefix(configDir, expectedPrefix) {
				t.Errorf("Linux config directory should respect XDG_CONFIG_HOME, got: %s", configDir)
			}
		} else {
			expectedPrefix := filepath.Join(homeDir, ".config")
			if !filepath.HasPrefix(configDir, expectedPrefix) {
				t.Errorf("Linux config directory should be under ~/.config, got: %s", configDir)
			}
		}
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	// Type assertion to access internal fields for testing
	m, ok := manager.(*Manager)
	if !ok {
		t.Fatal("NewManager() did not return *Manager")
	}

	if m.configPath == "" {
		t.Error("Manager configPath is empty")
	}

	if m.configDir == "" {
		t.Error("Manager configDir is empty")
	}

	if !filepath.IsAbs(m.configPath) {
		t.Errorf("Manager configPath should be absolute: %s", m.configPath)
	}
}

func TestManagerLoadDefaultConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "nl-to-shell-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create manager with custom config path
	manager := &Manager{
		configDir:  tempDir,
		configPath: filepath.Join(tempDir, configFileName),
	}

	// Load config (should return defaults since file doesn't exist)
	config, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default values
	if config.DefaultProvider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", config.DefaultProvider)
	}

	if config.Providers == nil {
		t.Error("Providers map should not be nil")
	}

	if config.UserPreferences.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.UserPreferences.DefaultTimeout)
	}

	if config.UserPreferences.MaxFileListSize != 100 {
		t.Errorf("Expected max file list size 100, got %d", config.UserPreferences.MaxFileListSize)
	}

	if !config.UserPreferences.EnablePlugins {
		t.Error("Expected plugins to be enabled by default")
	}

	if !config.UserPreferences.AutoUpdate {
		t.Error("Expected auto update to be enabled by default")
	}

	if !config.UpdateSettings.AutoCheck {
		t.Error("Expected auto check to be enabled by default")
	}

	if config.UpdateSettings.CheckInterval != 24*time.Hour {
		t.Errorf("Expected check interval 24h, got %v", config.UpdateSettings.CheckInterval)
	}
}

func TestManagerSaveAndLoad(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "nl-to-shell-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create manager with custom config path
	manager := &Manager{
		configDir:  tempDir,
		configPath: filepath.Join(tempDir, configFileName),
	}

	// Create test configuration
	testConfig := &types.Config{
		DefaultProvider: "anthropic",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      45 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{
			SkipConfirmation: true,
			VerboseOutput:    true,
			DefaultTimeout:   60 * time.Second,
			MaxFileListSize:  200,
			EnablePlugins:    false,
			AutoUpdate:       false,
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck:          false,
			CheckInterval:      48 * time.Hour,
			AllowPrerelease:    true,
			BackupBeforeUpdate: false,
		},
	}

	// Save configuration
	err = manager.Save(testConfig)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(manager.configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load configuration
	loadedConfig, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded configuration matches saved configuration
	if loadedConfig.DefaultProvider != testConfig.DefaultProvider {
		t.Errorf("Expected default provider '%s', got '%s'", testConfig.DefaultProvider, loadedConfig.DefaultProvider)
	}

	if len(loadedConfig.Providers) != len(testConfig.Providers) {
		t.Errorf("Expected %d providers, got %d", len(testConfig.Providers), len(loadedConfig.Providers))
	}

	openaiConfig, exists := loadedConfig.Providers["openai"]
	if !exists {
		t.Fatal("OpenAI provider config not found")
	}

	if openaiConfig.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", openaiConfig.APIKey)
	}

	if openaiConfig.DefaultModel != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", openaiConfig.DefaultModel)
	}

	if loadedConfig.UserPreferences.SkipConfirmation != true {
		t.Error("Expected SkipConfirmation to be true")
	}

	if loadedConfig.UserPreferences.VerboseOutput != true {
		t.Error("Expected VerboseOutput to be true")
	}
}

func TestManagerGetProviderConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "nl-to-shell-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create manager with custom config path
	manager := &Manager{
		configDir:         tempDir,
		configPath:        filepath.Join(tempDir, configFileName),
		credentialManager: NewCredentialManager(tempDir),
	}

	// Create test configuration with provider
	testConfig := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      45 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{},
		UpdateSettings:  types.UpdateSettings{},
	}

	// Save configuration
	err = manager.Save(testConfig)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Test getting existing provider config
	providerConfig, err := manager.GetProviderConfig("openai")
	if err != nil {
		t.Fatalf("GetProviderConfig() failed: %v", err)
	}

	if providerConfig.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", providerConfig.APIKey)
	}

	if providerConfig.DefaultModel != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", providerConfig.DefaultModel)
	}

	// Test getting non-existing provider config (should return defaults)
	defaultConfig, err := manager.GetProviderConfig("nonexistent")
	if err != nil {
		t.Fatalf("GetProviderConfig() for non-existing provider failed: %v", err)
	}

	if defaultConfig.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", defaultConfig.Timeout)
	}
}

func TestManagerMergeWithDefaults(t *testing.T) {
	manager := &Manager{}

	// Create partial config (missing some fields)
	partialConfig := &types.Config{
		DefaultProvider: "anthropic",
		Providers:       map[string]types.ProviderConfig{},
		UserPreferences: types.UserPreferences{
			SkipConfirmation: true,
			// Missing other fields
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck: false,
			// Missing other fields
		},
	}

	defaultConfig := manager.getDefaultConfig()
	manager.mergeWithDefaults(partialConfig, defaultConfig)

	// Verify that missing fields were filled with defaults
	if partialConfig.UserPreferences.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout to be filled, got %v", partialConfig.UserPreferences.DefaultTimeout)
	}

	if partialConfig.UserPreferences.MaxFileListSize != 100 {
		t.Errorf("Expected max file list size to be filled, got %d", partialConfig.UserPreferences.MaxFileListSize)
	}

	if partialConfig.UpdateSettings.CheckInterval != 24*time.Hour {
		t.Errorf("Expected check interval to be filled, got %v", partialConfig.UpdateSettings.CheckInterval)
	}

	// Verify that existing values were preserved
	if partialConfig.DefaultProvider != "anthropic" {
		t.Errorf("Expected existing default provider to be preserved, got %s", partialConfig.DefaultProvider)
	}

	if partialConfig.UserPreferences.SkipConfirmation != true {
		t.Error("Expected existing SkipConfirmation to be preserved")
	}

	if partialConfig.UpdateSettings.AutoCheck != false {
		t.Error("Expected existing AutoCheck to be preserved")
	}
}

func TestManagerEnsureConfigDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "nl-to-shell-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create manager with nested config directory
	nestedConfigDir := filepath.Join(tempDir, "nested", "config")
	manager := &Manager{
		configDir:  nestedConfigDir,
		configPath: filepath.Join(nestedConfigDir, configFileName),
	}

	// Ensure directory doesn't exist initially
	if _, err := os.Stat(nestedConfigDir); !os.IsNotExist(err) {
		t.Fatal("Nested config directory should not exist initially")
	}

	// Call ensureConfigDirectory
	err = manager.ensureConfigDirectory()
	if err != nil {
		t.Fatalf("ensureConfigDirectory() failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(nestedConfigDir)
	if err != nil {
		t.Fatalf("Config directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Config path is not a directory")
	}

	// Verify permissions (on Unix-like systems)
	if runtime.GOOS != "windows" {
		expectedMode := os.FileMode(0700)
		if info.Mode().Perm() != expectedMode {
			t.Errorf("Expected directory permissions %o, got %o", expectedMode, info.Mode().Perm())
		}
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	// Test that our config structures can be properly serialized/deserialized
	testConfig := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      45 * time.Second,
			},
			"anthropic": {
				APIKey:       "test-key-2",
				BaseURL:      "https://api.anthropic.com",
				DefaultModel: "claude-3",
				Timeout:      60 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{
			SkipConfirmation: true,
			VerboseOutput:    false,
			DefaultTimeout:   30 * time.Second,
			MaxFileListSize:  150,
			EnablePlugins:    true,
			AutoUpdate:       false,
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck:          true,
			CheckInterval:      12 * time.Hour,
			AllowPrerelease:    false,
			BackupBeforeUpdate: true,
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	// Unmarshal from JSON
	var loadedConfig types.Config
	err = json.Unmarshal(data, &loadedConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal config from JSON: %v", err)
	}

	// Verify all fields were preserved
	if loadedConfig.DefaultProvider != testConfig.DefaultProvider {
		t.Errorf("DefaultProvider mismatch: expected %s, got %s", testConfig.DefaultProvider, loadedConfig.DefaultProvider)
	}

	if len(loadedConfig.Providers) != len(testConfig.Providers) {
		t.Errorf("Providers count mismatch: expected %d, got %d", len(testConfig.Providers), len(loadedConfig.Providers))
	}

	// Check specific provider
	openaiConfig := loadedConfig.Providers["openai"]
	expectedOpenAI := testConfig.Providers["openai"]
	if openaiConfig.APIKey != expectedOpenAI.APIKey {
		t.Errorf("OpenAI APIKey mismatch: expected %s, got %s", expectedOpenAI.APIKey, openaiConfig.APIKey)
	}

	if openaiConfig.Timeout != expectedOpenAI.Timeout {
		t.Errorf("OpenAI Timeout mismatch: expected %v, got %v", expectedOpenAI.Timeout, openaiConfig.Timeout)
	}

	// Check user preferences
	if loadedConfig.UserPreferences.MaxFileListSize != testConfig.UserPreferences.MaxFileListSize {
		t.Errorf("MaxFileListSize mismatch: expected %d, got %d", testConfig.UserPreferences.MaxFileListSize, loadedConfig.UserPreferences.MaxFileListSize)
	}

	// Check update settings
	if loadedConfig.UpdateSettings.CheckInterval != testConfig.UpdateSettings.CheckInterval {
		t.Errorf("CheckInterval mismatch: expected %v, got %v", testConfig.UpdateSettings.CheckInterval, loadedConfig.UpdateSettings.CheckInterval)
	}
}
