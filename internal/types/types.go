package types

import (
	"fmt"
	"regexp"
	"time"
)

// Command represents a shell command to be executed
type Command struct {
	ID          string
	Original    string   // Original natural language input
	Generated   string   // Generated shell command
	Validated   bool     // Whether it passed safety validation
	Context     *Context // Context used for generation
	Timestamp   time.Time
	Args        []string
	WorkingDir  string
	Environment map[string]string
	Timeout     time.Duration
}

// Context holds environmental information for command generation
type Context struct {
	WorkingDirectory string
	Files            []FileInfo
	GitInfo          *GitContext
	Environment      map[string]string
	PluginData       map[string]interface{}
}

// FileInfo represents file system information
type FileInfo struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// GitContext holds git repository information
type GitContext struct {
	IsRepository          bool
	CurrentBranch         string
	WorkingTreeStatus     string
	HasUncommittedChanges bool
}

// CommandResult represents the complete result of command generation
type CommandResult struct {
	Command      *Command
	Safety       *SafetyResult
	Confidence   float64
	Alternatives []string
}

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
	Command  *Command
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Success  bool
	Error    error
}

// ValidationResult represents AI validation of execution results
type ValidationResult struct {
	IsCorrect        bool
	Explanation      string
	Suggestions      []string
	CorrectedCommand string
}

// SafetyResult represents the result of safety validation
type SafetyResult struct {
	IsSafe               bool
	DangerLevel          DangerLevel
	Warnings             []string
	RequiresConfirmation bool
}

// DangerousPattern represents a pattern for dangerous command detection
type DangerousPattern struct {
	Pattern     *regexp.Regexp
	Description string
	Level       DangerLevel
}

// DangerLevel represents the danger level of a command
type DangerLevel int

const (
	Safe DangerLevel = iota
	Warning
	Dangerous
	Critical
)

// DryRunResult represents the result of a dry run
type DryRunResult struct {
	Command     *Command
	Analysis    string
	Predictions []string
	Safety      *SafetyResult
}

// CommandResponse represents the response from an LLM provider
type CommandResponse struct {
	Command      string
	Explanation  string
	Confidence   float64
	Alternatives []string
}

// ValidationResponse represents the response from result validation
type ValidationResponse struct {
	IsCorrect   bool
	Explanation string
	Suggestions []string
	Correction  string
}

// ProviderInfo contains information about an LLM provider
type ProviderInfo struct {
	Name            string
	RequiresAuth    bool
	SupportedModels []string
}

// Config represents the application configuration
type Config struct {
	DefaultProvider string
	Providers       map[string]ProviderConfig
	UserPreferences UserPreferences
	UpdateSettings  UpdateSettings
}

// ProviderConfig represents configuration for a specific provider
type ProviderConfig struct {
	APIKey       string
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
}

// UserPreferences stores user-specific settings
type UserPreferences struct {
	SkipConfirmation bool
	VerboseOutput    bool
	DefaultTimeout   time.Duration
	MaxFileListSize  int
	EnablePlugins    bool
	AutoUpdate       bool
}

// UpdateSettings controls update behavior
type UpdateSettings struct {
	AutoCheck          bool
	CheckInterval      time.Duration
	AllowPrerelease    bool
	BackupBeforeUpdate bool
}

// UpdateInfo represents information about available updates
type UpdateInfo struct {
	Available      bool
	LatestVersion  string
	CurrentVersion string
	ReleaseNotes   string
	DownloadURL    string
	Checksum       string
}

// CLIConfig represents CLI-specific configuration
type CLIConfig struct {
	DryRun           bool
	Verbose          bool
	Provider         string
	Model            string
	SkipConfirmation bool
	SessionMode      bool
	ValidateResults  bool
}

// ErrorType represents different types of errors
type ErrorType int

const (
	ErrTypeValidation ErrorType = iota
	ErrTypeProvider
	ErrTypeExecution
	ErrTypeConfiguration
	ErrTypeNetwork
	ErrTypePermission
)

// NLShellError represents a structured error with context
type NLShellError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

func (e *NLShellError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *NLShellError) Unwrap() error {
	return e.Cause
}
