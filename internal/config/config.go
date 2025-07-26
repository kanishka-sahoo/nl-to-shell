package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

const (
	configFileName = "config.json"
	appName        = "nl-to-shell"
)

// Manager implements the ConfigManager interface
type Manager struct {
	configPath        string
	configDir         string
	credentialManager *credentialManager
}

// NewManager creates a new configuration manager
func NewManager() interfaces.ConfigManager {
	configDir, err := getConfigDirectory()
	if err != nil {
		// Fallback to current directory if we can't determine config directory
		configDir = "."
	}

	return &Manager{
		configDir:         configDir,
		configPath:        filepath.Join(configDir, configFileName),
		credentialManager: NewCredentialManager(configDir),
	}
}

// Load loads the configuration from storage
func (m *Manager) Load() (*types.Config, error) {
	// Ensure config directory exists
	if err := m.ensureConfigDirectory(); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Return default configuration if file doesn't exist
		return m.getDefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config types.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge with defaults to ensure all fields are populated
	defaultConfig := m.getDefaultConfig()
	m.mergeWithDefaults(&config, defaultConfig)

	return &config, nil
}

// Save saves the configuration to storage
func (m *Manager) Save(config *types.Config) error {
	// Ensure config directory exists
	if err := m.ensureConfigDirectory(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file with appropriate permissions
	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetProviderConfig retrieves configuration for a specific provider
func (m *Manager) GetProviderConfig(provider string) (*types.ProviderConfig, error) {
	config, err := m.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var providerConfig types.ProviderConfig
	if existingConfig, exists := config.Providers[provider]; exists {
		providerConfig = existingConfig
	} else {
		// Return default provider config if not found
		providerConfig = types.ProviderConfig{
			Timeout: 30 * time.Second,
		}
	}

	// Try to get API key from various sources in order of preference:
	// 1. Environment variables
	// 2. Secure credential storage
	// 3. Configuration file (if already set)

	if providerConfig.APIKey == "" {
		// Try environment variables first
		if envKey := GetCredentialFromEnv(provider, "default"); envKey != "" {
			providerConfig.APIKey = envKey
		} else {
			// Try secure credential storage
			if storedKey, err := m.credentialManager.Retrieve(provider, "api_key"); err == nil {
				providerConfig.APIKey = storedKey
			}
		}
	}

	return &providerConfig, nil
}

// SetupInteractive runs interactive configuration setup
func (m *Manager) SetupInteractive() error {
	fmt.Println("Welcome to nl-to-shell interactive setup!")
	fmt.Println("This will help you configure your LLM providers and preferences.")
	fmt.Println()

	// Load existing config or create new one
	config, err := m.Load()
	if err != nil {
		return fmt.Errorf("failed to load existing config: %w", err)
	}

	// Setup providers
	if err := m.setupProviders(config); err != nil {
		return fmt.Errorf("failed to setup providers: %w", err)
	}

	// Setup user preferences
	if err := m.setupUserPreferences(config); err != nil {
		return fmt.Errorf("failed to setup user preferences: %w", err)
	}

	// Setup update settings
	if err := m.setupUpdateSettings(config); err != nil {
		return fmt.Errorf("failed to setup update settings: %w", err)
	}

	// Save the configuration
	if err := m.Save(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Test the configuration
	if err := m.testConfiguration(config); err != nil {
		fmt.Printf("Warning: Configuration test failed: %v\n", err)
		fmt.Println("You can continue using the application, but some features may not work correctly.")
	} else {
		fmt.Println("✓ Configuration test passed!")
	}

	fmt.Println()
	fmt.Println("Setup completed successfully!")
	fmt.Printf("Configuration saved to: %s\n", m.configPath)

	return nil
}

// GetConfigPath returns the path to the configuration file
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// GetConfigDirectory returns the configuration directory path
func (m *Manager) GetConfigDirectory() string {
	return m.configDir
}

// ensureConfigDirectory creates the configuration directory if it doesn't exist
func (m *Manager) ensureConfigDirectory() error {
	return os.MkdirAll(m.configDir, 0700)
}

// getDefaultConfig returns a default configuration
func (m *Manager) getDefaultConfig() *types.Config {
	return &types.Config{
		DefaultProvider: "openai",
		Providers:       make(map[string]types.ProviderConfig),
		UserPreferences: types.UserPreferences{
			SkipConfirmation: false,
			VerboseOutput:    false,
			DefaultTimeout:   30 * time.Second,
			MaxFileListSize:  100,
			EnablePlugins:    true,
			AutoUpdate:       true,
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck:          true,
			CheckInterval:      24 * time.Hour,
			AllowPrerelease:    false,
			BackupBeforeUpdate: true,
		},
	}
}

// mergeWithDefaults merges the loaded config with default values
func (m *Manager) mergeWithDefaults(config *types.Config, defaults *types.Config) {
	if config.DefaultProvider == "" {
		config.DefaultProvider = defaults.DefaultProvider
	}

	if config.Providers == nil {
		config.Providers = make(map[string]types.ProviderConfig)
	}

	// Merge user preferences with defaults
	if config.UserPreferences.DefaultTimeout == 0 {
		config.UserPreferences.DefaultTimeout = defaults.UserPreferences.DefaultTimeout
	}
	if config.UserPreferences.MaxFileListSize == 0 {
		config.UserPreferences.MaxFileListSize = defaults.UserPreferences.MaxFileListSize
	}

	// Merge update settings with defaults
	if config.UpdateSettings.CheckInterval == 0 {
		config.UpdateSettings.CheckInterval = defaults.UpdateSettings.CheckInterval
	}
}

// StoreCredential stores a credential securely
func (m *Manager) StoreCredential(provider, credentialType, value string) error {
	return m.credentialManager.Store(provider, credentialType, value)
}

// RetrieveCredential retrieves a credential securely
func (m *Manager) RetrieveCredential(provider, credentialType string) (string, error) {
	return m.credentialManager.Retrieve(provider, credentialType)
}

// DeleteCredential deletes a credential
func (m *Manager) DeleteCredential(provider, credentialType string) error {
	return m.credentialManager.Delete(provider, credentialType)
}

// ListCredentials lists all credential types for a provider
func (m *Manager) ListCredentials(provider string) ([]string, error) {
	return m.credentialManager.List(provider)
}

// setupProviders handles interactive provider configuration
func (m *Manager) setupProviders(config *types.Config) error {
	fmt.Println("=== Provider Configuration ===")

	availableProviders := []string{"openai", "anthropic", "google", "ollama"}

	// Ask which providers to configure
	fmt.Println("Available LLM providers:")
	for i, provider := range availableProviders {
		fmt.Printf("%d. %s\n", i+1, provider)
	}
	fmt.Println()

	selectedProviders := make(map[string]bool)

	for {
		fmt.Print("Enter provider numbers to configure (comma-separated, e.g., 1,2) or 'done': ")
		input := readInput()

		if input == "done" {
			break
		}

		if input == "" {
			continue
		}

		// Parse selected providers
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// Convert to index
			var providerIndex int
			if _, err := fmt.Sscanf(part, "%d", &providerIndex); err != nil {
				fmt.Printf("Invalid input: %s\n", part)
				continue
			}

			if providerIndex < 1 || providerIndex > len(availableProviders) {
				fmt.Printf("Invalid provider number: %d\n", providerIndex)
				continue
			}

			provider := availableProviders[providerIndex-1]
			selectedProviders[provider] = true
		}

		if len(selectedProviders) > 0 {
			break
		}
	}

	// Configure each selected provider
	for provider := range selectedProviders {
		if err := m.configureProvider(config, provider); err != nil {
			return fmt.Errorf("failed to configure provider %s: %w", provider, err)
		}
	}

	// Set default provider
	if len(selectedProviders) > 0 {
		if err := m.selectDefaultProvider(config, selectedProviders); err != nil {
			return fmt.Errorf("failed to select default provider: %w", err)
		}
	}

	return nil
}

// configureProvider configures a specific provider
func (m *Manager) configureProvider(config *types.Config, provider string) error {
	fmt.Printf("\n--- Configuring %s ---\n", provider)

	providerConfig := types.ProviderConfig{
		Timeout: 30 * time.Second,
	}

	// Get existing config if available
	if existing, exists := config.Providers[provider]; exists {
		providerConfig = existing
	}

	// Configure API key
	fmt.Printf("API Key for %s: ", provider)
	if providerConfig.APIKey != "" {
		fmt.Print("(current key is set, press Enter to keep) ")
	}

	apiKey := readInput()
	if apiKey != "" {
		// Store in secure credential storage
		if err := m.credentialManager.Store(provider, "api_key", apiKey); err != nil {
			return fmt.Errorf("failed to store API key: %w", err)
		}
		providerConfig.APIKey = "" // Don't store in config file
	}

	// Configure base URL (optional)
	fmt.Printf("Base URL for %s (optional, press Enter for default): ", provider)
	baseURL := readInput()
	if baseURL != "" {
		providerConfig.BaseURL = baseURL
	}

	// Configure default model
	defaultModels := getDefaultModels(provider)
	if len(defaultModels) > 0 {
		fmt.Printf("Available models for %s:\n", provider)
		for i, model := range defaultModels {
			fmt.Printf("%d. %s\n", i+1, model)
		}
		fmt.Print("Select default model (number) or enter custom model name: ")
		modelInput := readInput()

		if modelInput != "" {
			// Try to parse as number first
			var modelIndex int
			if _, err := fmt.Sscanf(modelInput, "%d", &modelIndex); err == nil {
				if modelIndex >= 1 && modelIndex <= len(defaultModels) {
					providerConfig.DefaultModel = defaultModels[modelIndex-1]
				}
			} else {
				// Use as custom model name
				providerConfig.DefaultModel = modelInput
			}
		}
	}

	// Configure timeout
	fmt.Printf("Request timeout in seconds (default: 30): ")
	timeoutInput := readInput()
	if timeoutInput != "" {
		var timeoutSeconds int
		if _, err := fmt.Sscanf(timeoutInput, "%d", &timeoutSeconds); err == nil && timeoutSeconds > 0 {
			providerConfig.Timeout = time.Duration(timeoutSeconds) * time.Second
		}
	}

	// Save provider config
	config.Providers[provider] = providerConfig

	fmt.Printf("✓ %s configured successfully\n", provider)
	return nil
}

// selectDefaultProvider allows user to select the default provider
func (m *Manager) selectDefaultProvider(config *types.Config, selectedProviders map[string]bool) error {
	if len(selectedProviders) == 1 {
		// Only one provider, set it as default
		for provider := range selectedProviders {
			config.DefaultProvider = provider
			fmt.Printf("✓ Set %s as default provider\n", provider)
			return nil
		}
	}

	// Multiple providers, let user choose
	fmt.Println("\nSelect default provider:")
	providers := make([]string, 0, len(selectedProviders))
	for provider := range selectedProviders {
		providers = append(providers, provider)
	}

	for i, provider := range providers {
		fmt.Printf("%d. %s\n", i+1, provider)
	}

	for {
		fmt.Print("Enter provider number: ")
		input := readInput()

		var providerIndex int
		if _, err := fmt.Sscanf(input, "%d", &providerIndex); err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		if providerIndex < 1 || providerIndex > len(providers) {
			fmt.Printf("Invalid provider number: %d\n", providerIndex)
			continue
		}

		config.DefaultProvider = providers[providerIndex-1]
		fmt.Printf("✓ Set %s as default provider\n", config.DefaultProvider)
		break
	}

	return nil
}

// setupUserPreferences handles interactive user preferences configuration
func (m *Manager) setupUserPreferences(config *types.Config) error {
	fmt.Println("\n=== User Preferences ===")

	// Skip confirmation
	fmt.Printf("Skip confirmation prompts for safe commands? (y/N): ")
	if readYesNo(false) {
		config.UserPreferences.SkipConfirmation = true
	}

	// Verbose output
	fmt.Printf("Enable verbose output? (y/N): ")
	if readYesNo(false) {
		config.UserPreferences.VerboseOutput = true
	}

	// Default timeout
	fmt.Printf("Default command timeout in seconds (current: %.0f): ", config.UserPreferences.DefaultTimeout.Seconds())
	timeoutInput := readInput()
	if timeoutInput != "" {
		var timeoutSeconds int
		if _, err := fmt.Sscanf(timeoutInput, "%d", &timeoutSeconds); err == nil && timeoutSeconds > 0 {
			config.UserPreferences.DefaultTimeout = time.Duration(timeoutSeconds) * time.Second
		}
	}

	// Max file list size
	fmt.Printf("Maximum number of files to include in context (current: %d): ", config.UserPreferences.MaxFileListSize)
	maxFilesInput := readInput()
	if maxFilesInput != "" {
		var maxFiles int
		if _, err := fmt.Sscanf(maxFilesInput, "%d", &maxFiles); err == nil && maxFiles > 0 {
			config.UserPreferences.MaxFileListSize = maxFiles
		}
	}

	// Enable plugins
	fmt.Printf("Enable context plugins? (Y/n): ")
	if readYesNo(true) {
		config.UserPreferences.EnablePlugins = true
	} else {
		config.UserPreferences.EnablePlugins = false
	}

	// Auto update
	fmt.Printf("Enable automatic updates? (Y/n): ")
	if readYesNo(true) {
		config.UserPreferences.AutoUpdate = true
	} else {
		config.UserPreferences.AutoUpdate = false
	}

	fmt.Println("✓ User preferences configured")
	return nil
}

// setupUpdateSettings handles interactive update settings configuration
func (m *Manager) setupUpdateSettings(config *types.Config) error {
	fmt.Println("\n=== Update Settings ===")

	// Auto check
	fmt.Printf("Automatically check for updates? (Y/n): ")
	if readYesNo(true) {
		config.UpdateSettings.AutoCheck = true
	} else {
		config.UpdateSettings.AutoCheck = false
	}

	// Check interval
	if config.UpdateSettings.AutoCheck {
		fmt.Printf("Update check interval in hours (current: %.0f): ", config.UpdateSettings.CheckInterval.Hours())
		intervalInput := readInput()
		if intervalInput != "" {
			var intervalHours int
			if _, err := fmt.Sscanf(intervalInput, "%d", &intervalHours); err == nil && intervalHours > 0 {
				config.UpdateSettings.CheckInterval = time.Duration(intervalHours) * time.Hour
			}
		}
	}

	// Allow prerelease
	fmt.Printf("Allow prerelease versions? (y/N): ")
	if readYesNo(false) {
		config.UpdateSettings.AllowPrerelease = true
	}

	// Backup before update
	fmt.Printf("Backup configuration before updates? (Y/n): ")
	if readYesNo(true) {
		config.UpdateSettings.BackupBeforeUpdate = true
	} else {
		config.UpdateSettings.BackupBeforeUpdate = false
	}

	fmt.Println("✓ Update settings configured")
	return nil
}

// testConfiguration tests the configuration by attempting to validate provider credentials
func (m *Manager) testConfiguration(config *types.Config) error {
	fmt.Println("\n=== Testing Configuration ===")

	if config.DefaultProvider == "" {
		return fmt.Errorf("no default provider configured")
	}

	// Test default provider
	providerConfig, err := m.GetProviderConfig(config.DefaultProvider)
	if err != nil {
		return fmt.Errorf("failed to get provider config: %w", err)
	}

	if providerConfig.APIKey == "" {
		return fmt.Errorf("no API key configured for provider %s", config.DefaultProvider)
	}

	fmt.Printf("✓ Default provider %s has API key configured\n", config.DefaultProvider)

	// Test other configured providers
	for provider := range config.Providers {
		if provider == config.DefaultProvider {
			continue
		}

		providerConfig, err := m.GetProviderConfig(provider)
		if err != nil {
			fmt.Printf("⚠ Warning: Failed to get config for provider %s: %v\n", provider, err)
			continue
		}

		if providerConfig.APIKey == "" {
			fmt.Printf("⚠ Warning: No API key configured for provider %s\n", provider)
			continue
		}

		fmt.Printf("✓ Provider %s has API key configured\n", provider)
	}

	return nil
}

// Helper functions

// readInput reads a line of input from stdin
func readInput() string {
	var input string
	fmt.Scanln(&input)
	return strings.TrimSpace(input)
}

// readYesNo reads a yes/no response with a default value
func readYesNo(defaultValue bool) bool {
	input := readInput()
	input = strings.ToLower(input)

	if input == "" {
		return defaultValue
	}

	return input == "y" || input == "yes"
}

// getDefaultModels returns default models for a provider
func getDefaultModels(provider string) []string {
	switch provider {
	case "openai":
		return []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"}
	case "anthropic":
		return []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"}
	case "google":
		return []string{"gemini-pro", "gemini-pro-vision"}
	case "ollama":
		return []string{"llama2", "codellama", "mistral"}
	default:
		return []string{}
	}
}

// getConfigDirectory returns the appropriate configuration directory for the current platform
func getConfigDirectory() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		// Use %APPDATA% on Windows
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, appName)

	case "darwin":
		// Use ~/Library/Application Support on macOS
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, "Library", "Application Support", appName)

	default:
		// Use XDG Base Directory specification on Linux and other Unix-like systems
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			configDir = filepath.Join(xdgConfigHome, appName)
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			configDir = filepath.Join(homeDir, ".config", appName)
		}
	}

	return configDir, nil
}
