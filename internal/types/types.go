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
	ErrTypePlugin
	ErrTypeContext
	ErrTypeUpdate
	ErrTypeSafety
	ErrTypeTimeout
	ErrTypeAuth
	ErrTypeInternal
)

// String returns the string representation of ErrorType
func (e ErrorType) String() string {
	switch e {
	case ErrTypeValidation:
		return "Validation"
	case ErrTypeProvider:
		return "Provider"
	case ErrTypeExecution:
		return "Execution"
	case ErrTypeConfiguration:
		return "Configuration"
	case ErrTypeNetwork:
		return "Network"
	case ErrTypePermission:
		return "Permission"
	case ErrTypePlugin:
		return "Plugin"
	case ErrTypeContext:
		return "Context"
	case ErrTypeUpdate:
		return "Update"
	case ErrTypeSafety:
		return "Safety"
	case ErrTypeTimeout:
		return "Timeout"
	case ErrTypeAuth:
		return "Authentication"
	case ErrTypeInternal:
		return "Internal"
	default:
		return "Unknown"
	}
}

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// String returns the string representation of ErrorSeverity
func (s ErrorSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	case SeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// NLShellError represents a structured error with context
type NLShellError struct {
	Type      ErrorType
	Severity  ErrorSeverity
	Message   string
	Cause     error
	Context   map[string]interface{}
	Timestamp time.Time
	Component string // Component where the error occurred
	Operation string // Operation being performed when error occurred
	UserID    string // User ID if available
	SessionID string // Session ID if available
}

// Error implements the error interface
func (e *NLShellError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

// Unwrap implements the error unwrapping interface
func (e *NLShellError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for errors.Is
func (e *NLShellError) Is(target error) bool {
	if t, ok := target.(*NLShellError); ok {
		return e.Type == t.Type
	}
	return false
}

// WithContext adds context information to the error
func (e *NLShellError) WithContext(key string, value interface{}) *NLShellError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithComponent sets the component where the error occurred
func (e *NLShellError) WithComponent(component string) *NLShellError {
	e.Component = component
	return e
}

// WithOperation sets the operation being performed when error occurred
func (e *NLShellError) WithOperation(operation string) *NLShellError {
	e.Operation = operation
	return e
}

// WithUserID sets the user ID associated with the error
func (e *NLShellError) WithUserID(userID string) *NLShellError {
	e.UserID = userID
	return e
}

// WithSessionID sets the session ID associated with the error
func (e *NLShellError) WithSessionID(sessionID string) *NLShellError {
	e.SessionID = sessionID
	return e
}

// GetContextValue retrieves a context value by key
func (e *NLShellError) GetContextValue(key string) (interface{}, bool) {
	if e.Context == nil {
		return nil, false
	}
	value, exists := e.Context[key]
	return value, exists
}

// ToMap converts the error to a map for structured logging
func (e *NLShellError) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"type":      e.Type.String(),
		"severity":  e.Severity.String(),
		"message":   e.Message,
		"timestamp": e.Timestamp,
	}

	if e.Component != "" {
		result["component"] = e.Component
	}
	if e.Operation != "" {
		result["operation"] = e.Operation
	}
	if e.UserID != "" {
		result["user_id"] = e.UserID
	}
	if e.SessionID != "" {
		result["session_id"] = e.SessionID
	}
	if e.Cause != nil {
		result["cause"] = e.Cause.Error()
	}
	if e.Context != nil && len(e.Context) > 0 {
		result["context"] = e.Context
	}

	return result
}
