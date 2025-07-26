package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	configPath string
	configDir  string
}

// NewManager creates a new configuration manager
func NewManager() interfaces.ConfigManager {
	configDir, err := getConfigDirectory()
	if err != nil {
		// Fallback to current directory if we can't determine config directory
		configDir = "."
	}

	return &Manager{
		configDir:  configDir,
		configPath: filepath.Join(configDir, configFileName),
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

	if providerConfig, exists := config.Providers[provider]; exists {
		return &providerConfig, nil
	}

	// Return default provider config if not found
	return &types.ProviderConfig{
		Timeout: 30 * time.Second,
	}, nil
}

// SetupInteractive runs interactive configuration setup
func (m *Manager) SetupInteractive() error {
	// Implementation will be added in task 2.3
	return fmt.Errorf("interactive setup not yet implemented")
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
