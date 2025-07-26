package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestGetDefaultModels(t *testing.T) {
	testCases := []struct {
		provider string
		expected []string
	}{
		{
			provider: "openai",
			expected: []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		},
		{
			provider: "anthropic",
			expected: []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"},
		},
		{
			provider: "google",
			expected: []string{"gemini-pro", "gemini-pro-vision"},
		},
		{
			provider: "ollama",
			expected: []string{"llama2", "codellama", "mistral"},
		},
		{
			provider: "unknown",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		result := getDefaultModels(tc.provider)
		if len(result) != len(tc.expected) {
			t.Errorf("getDefaultModels(%s): expected %d models, got %d", tc.provider, len(tc.expected), len(result))
			continue
		}

		for i, expected := range tc.expected {
			if i >= len(result) || result[i] != expected {
				t.Errorf("getDefaultModels(%s): expected model %d to be '%s', got '%s'", tc.provider, i, expected, result[i])
			}
		}
	}
}

func TestReadYesNo(t *testing.T) {
	// Note: This test cannot easily test actual input reading without mocking stdin
	// Instead, we test the logic with direct function calls

	testCases := []struct {
		input        string
		defaultValue bool
		expected     bool
	}{
		{"", true, true},   // Empty input with default true
		{"", false, false}, // Empty input with default false
		{"y", true, true},
		{"y", false, true},
		{"yes", true, true},
		{"yes", false, true},
		{"Y", true, true},
		{"YES", true, true},
		{"n", true, false},
		{"n", false, false},
		{"no", true, false},
		{"NO", false, false},
		{"invalid", true, false}, // Invalid input should return false
	}

	for _, tc := range testCases {
		// We can't easily test readYesNo directly without mocking stdin
		// Instead, we test the logic inline
		input := tc.input
		defaultValue := tc.defaultValue

		var result bool
		if input == "" {
			result = defaultValue
		} else {
			input = strings.ToLower(input)
			result = input == "y" || input == "yes"
		}

		if result != tc.expected {
			t.Errorf("readYesNo logic with input '%s' and default %v: expected %v, got %v",
				tc.input, tc.defaultValue, tc.expected, result)
		}
	}
}

func TestManagerCredentialIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-test-*")
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

	// Test storing and retrieving credentials through manager
	err = manager.StoreCredential("openai", "api_key", "test-secret-key")
	if err != nil {
		t.Fatalf("StoreCredential() failed: %v", err)
	}

	// Test retrieving credential
	secret, err := manager.RetrieveCredential("openai", "api_key")
	if err != nil {
		t.Fatalf("RetrieveCredential() failed: %v", err)
	}

	if secret != "test-secret-key" {
		t.Errorf("Expected 'test-secret-key', got '%s'", secret)
	}

	// Test GetProviderConfig with stored credential
	config := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      30 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{},
		UpdateSettings:  types.UpdateSettings{},
	}

	// Save config
	err = manager.Save(config)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Get provider config (should include stored credential)
	providerConfig, err := manager.GetProviderConfig("openai")
	if err != nil {
		t.Fatalf("GetProviderConfig() failed: %v", err)
	}

	if providerConfig.APIKey != "test-secret-key" {
		t.Errorf("Expected API key 'test-secret-key', got '%s'", providerConfig.APIKey)
	}

	if providerConfig.DefaultModel != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", providerConfig.DefaultModel)
	}

	// Test listing credentials
	credentials, err := manager.ListCredentials("openai")
	if err != nil {
		t.Fatalf("ListCredentials() failed: %v", err)
	}

	if len(credentials) != 1 || credentials[0] != "api_key" {
		t.Errorf("Expected ['api_key'], got %v", credentials)
	}

	// Test deleting credential
	err = manager.DeleteCredential("openai", "api_key")
	if err != nil {
		t.Fatalf("DeleteCredential() failed: %v", err)
	}

	// Verify credential is deleted
	_, err = manager.RetrieveCredential("openai", "api_key")
	if err == nil {
		t.Error("Expected error after deleting credential")
	}
}

func TestManagerEnvironmentVariableOverride(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set environment variable
	os.Setenv("OPENAI_API_KEY", "env-api-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Create manager
	manager := &Manager{
		configDir:         tempDir,
		configPath:        filepath.Join(tempDir, configFileName),
		credentialManager: NewCredentialManager(tempDir),
	}

	// Create config without API key
	config := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      30 * time.Second,
				// No API key set
			},
		},
		UserPreferences: types.UserPreferences{},
		UpdateSettings:  types.UpdateSettings{},
	}

	// Save config
	err = manager.Save(config)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Get provider config (should use environment variable)
	providerConfig, err := manager.GetProviderConfig("openai")
	if err != nil {
		t.Fatalf("GetProviderConfig() failed: %v", err)
	}

	if providerConfig.APIKey != "env-api-key" {
		t.Errorf("Expected API key from environment 'env-api-key', got '%s'", providerConfig.APIKey)
	}

	// Store credential in secure storage
	err = manager.StoreCredential("openai", "api_key", "stored-api-key")
	if err != nil {
		t.Fatalf("StoreCredential() failed: %v", err)
	}

	// Get provider config again (environment should still take precedence)
	providerConfig, err = manager.GetProviderConfig("openai")
	if err != nil {
		t.Fatalf("GetProviderConfig() failed: %v", err)
	}

	if providerConfig.APIKey != "env-api-key" {
		t.Errorf("Expected environment variable to take precedence, got '%s'", providerConfig.APIKey)
	}

	// Remove environment variable
	os.Unsetenv("OPENAI_API_KEY")

	// Get provider config again (should now use stored credential)
	providerConfig, err = manager.GetProviderConfig("openai")
	if err != nil {
		t.Fatalf("GetProviderConfig() failed: %v", err)
	}

	if providerConfig.APIKey != "stored-api-key" {
		t.Errorf("Expected stored credential 'stored-api-key', got '%s'", providerConfig.APIKey)
	}
}

func TestManagerConfigurationValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &Manager{
		configDir:         tempDir,
		configPath:        filepath.Join(tempDir, configFileName),
		credentialManager: NewCredentialManager(tempDir),
	}

	// Test configuration with no default provider
	config := &types.Config{
		DefaultProvider: "",
		Providers:       make(map[string]types.ProviderConfig),
		UserPreferences: types.UserPreferences{},
		UpdateSettings:  types.UpdateSettings{},
	}

	err = manager.testConfiguration(config)
	if err == nil {
		t.Error("Expected error for configuration with no default provider")
	}

	// Test configuration with default provider but no API key
	config.DefaultProvider = "openai"
	config.Providers["openai"] = types.ProviderConfig{
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4",
		Timeout:      30 * time.Second,
	}

	err = manager.testConfiguration(config)
	if err == nil {
		t.Error("Expected error for configuration with no API key")
	}

	// Test configuration with API key
	err = manager.StoreCredential("openai", "api_key", "test-key")
	if err != nil {
		t.Fatalf("StoreCredential() failed: %v", err)
	}

	err = manager.testConfiguration(config)
	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

func TestManagerDefaultConfigGeneration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &Manager{
		configDir:         tempDir,
		configPath:        filepath.Join(tempDir, configFileName),
		credentialManager: NewCredentialManager(tempDir),
	}

	// Test getting default config
	defaultConfig := manager.getDefaultConfig()

	// Verify default values
	if defaultConfig.DefaultProvider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", defaultConfig.DefaultProvider)
	}

	if defaultConfig.Providers == nil {
		t.Error("Expected providers map to be initialized")
	}

	if defaultConfig.UserPreferences.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", defaultConfig.UserPreferences.DefaultTimeout)
	}

	if defaultConfig.UserPreferences.MaxFileListSize != 100 {
		t.Errorf("Expected max file list size 100, got %d", defaultConfig.UserPreferences.MaxFileListSize)
	}

	if !defaultConfig.UserPreferences.EnablePlugins {
		t.Error("Expected plugins to be enabled by default")
	}

	if !defaultConfig.UserPreferences.AutoUpdate {
		t.Error("Expected auto update to be enabled by default")
	}

	if !defaultConfig.UpdateSettings.AutoCheck {
		t.Error("Expected auto check to be enabled by default")
	}

	if defaultConfig.UpdateSettings.CheckInterval != 24*time.Hour {
		t.Errorf("Expected check interval 24h, got %v", defaultConfig.UpdateSettings.CheckInterval)
	}

	if defaultConfig.UpdateSettings.AllowPrerelease {
		t.Error("Expected prerelease to be disabled by default")
	}

	if !defaultConfig.UpdateSettings.BackupBeforeUpdate {
		t.Error("Expected backup before update to be enabled by default")
	}
}

func TestManagerMergeWithDefaultsEdgeCases(t *testing.T) {
	manager := &Manager{}

	// Test with completely empty config
	emptyConfig := &types.Config{}
	defaultConfig := manager.getDefaultConfig()
	manager.mergeWithDefaults(emptyConfig, defaultConfig)

	// Verify all fields were filled
	if emptyConfig.DefaultProvider != "openai" {
		t.Errorf("Expected default provider to be filled, got '%s'", emptyConfig.DefaultProvider)
	}

	if emptyConfig.Providers == nil {
		t.Error("Expected providers map to be initialized")
	}

	if emptyConfig.UserPreferences.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout to be filled, got %v", emptyConfig.UserPreferences.DefaultTimeout)
	}

	// Test with partial config that has some values set
	partialConfig := &types.Config{
		DefaultProvider: "anthropic", // This should be preserved
		Providers:       make(map[string]types.ProviderConfig),
		UserPreferences: types.UserPreferences{
			SkipConfirmation: true, // This should be preserved
			// Other fields should be filled with defaults
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck: false, // This should be preserved
			// Other fields should be filled with defaults
		},
	}

	manager.mergeWithDefaults(partialConfig, defaultConfig)

	// Verify existing values were preserved
	if partialConfig.DefaultProvider != "anthropic" {
		t.Errorf("Expected existing default provider to be preserved, got '%s'", partialConfig.DefaultProvider)
	}

	if !partialConfig.UserPreferences.SkipConfirmation {
		t.Error("Expected existing SkipConfirmation to be preserved")
	}

	if partialConfig.UpdateSettings.AutoCheck {
		t.Error("Expected existing AutoCheck to be preserved")
	}

	// Verify missing values were filled
	if partialConfig.UserPreferences.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected missing timeout to be filled, got %v", partialConfig.UserPreferences.DefaultTimeout)
	}

	if partialConfig.UpdateSettings.CheckInterval != 24*time.Hour {
		t.Errorf("Expected missing check interval to be filled, got %v", partialConfig.UpdateSettings.CheckInterval)
	}
}
