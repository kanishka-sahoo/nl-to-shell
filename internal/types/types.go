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
	Bypassed             bool
	AuditEntry           *AuditEntry
}

// ValidationOptions controls how safety validation is performed
type ValidationOptions struct {
	SkipConfirmation bool
	BypassLevel      DangerLevel // Commands at or below this level can be bypassed
	AuditLogger      AuditLogger // Logger for audit events
	UserID           string      // User performing the bypass
	Reason           string      // Reason for bypass
}

// AuditEntry represents a security audit log entry
type AuditEntry struct {
	Timestamp   time.Time
	Command     string
	UserID      string
	Action      AuditAction
	DangerLevel DangerLevel
	Reason      string
	SourceIP    string
	SessionID   string
}

// AuditAction represents the type of audit action
type AuditAction int

const (
	AuditActionValidated AuditAction = iota
	AuditActionBypassed
	AuditActionBlocked
	AuditActionOverridden
)

// String returns the string representation of AuditAction
func (a AuditAction) String() string {
	switch a {
	case AuditActionValidated:
		return "Validated"
	case AuditActionBypassed:
		return "Bypassed"
	case AuditActionBlocked:
		return "Blocked"
	case AuditActionOverridden:
		return "Overridden"
	default:
		return "Unknown"
	}
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	LogAuditEvent(entry *AuditEntry) error
	GetAuditLog(filter *AuditFilter) ([]*AuditEntry, error)
}

// AuditFilter represents filtering criteria for audit logs
type AuditFilter struct {
	StartTime   *time.Time
	EndTime     *time.Time
	UserID      string
	Action      *AuditAction
	DangerLevel *DangerLevel
}

// BypassConfig represents bypass configuration for safety validation
type BypassConfig struct {
	Enabled       bool
	MaxLevel      DangerLevel // Maximum danger level that can be bypassed
	RequireReason bool        // Whether a reason is required for bypass
	AuditAll      bool        // Whether to audit all commands or just bypassed ones
	TrustedUsers  []string    // List of users allowed to bypass
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

// String returns the string representation of DangerLevel
func (d DangerLevel) String() string {
	switch d {
	case Safe:
		return "Safe"
	case Warning:
		return "Warning"
	case Dangerous:
		return "Dangerous"
	case Critical:
		return "Critical"
	default:
		return "Unknown"
	}
}

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
	Bypass           BypassConfig
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

// ExecutionOptions controls how commands are executed
type ExecutionOptions struct {
	DryRun           bool
	SkipConfirmation bool
	ValidateResults  bool
	Timeout          time.Duration
}

// FullResult represents the complete result of command generation and execution
type FullResult struct {
	CommandResult        *CommandResult
	ExecutionResult      *ExecutionResult
	ValidationResult     *ValidationResult
	DryRunResult         *DryRunResult
	RequiresConfirmation bool
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
