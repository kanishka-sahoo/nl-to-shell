package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Logger defines the interface for structured error logging
type Logger interface {
	LogError(err *types.NLShellError)
	LogErrorWithContext(ctx context.Context, err *types.NLShellError)
	SetLevel(level LogLevel)
	Close() error
}

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelCritical
)

// String returns the string representation of LogLevel
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// StructuredLogger implements structured error logging
type StructuredLogger struct {
	logger   *log.Logger
	level    LogLevel
	jsonMode bool
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(jsonMode bool) *StructuredLogger {
	return &StructuredLogger{
		logger:   log.New(os.Stderr, "", 0),
		level:    LogLevelInfo,
		jsonMode: jsonMode,
	}
}

// NewFileLogger creates a logger that writes to a file
func NewFileLogger(filename string, jsonMode bool) (*StructuredLogger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &StructuredLogger{
		logger:   log.New(file, "", 0),
		level:    LogLevelInfo,
		jsonMode: jsonMode,
	}, nil
}

// SetLevel sets the minimum logging level
func (l *StructuredLogger) SetLevel(level LogLevel) {
	l.level = level
}

// LogError logs a structured error
func (l *StructuredLogger) LogError(err *types.NLShellError) {
	l.LogErrorWithContext(context.Background(), err)
}

// LogErrorWithContext logs a structured error with context
func (l *StructuredLogger) LogErrorWithContext(ctx context.Context, err *types.NLShellError) {
	if err == nil {
		return
	}

	// Convert severity to log level
	logLevel := l.severityToLogLevel(err.Severity)
	if logLevel < l.level {
		return
	}

	if l.jsonMode {
		l.logJSON(ctx, err, logLevel)
	} else {
		l.logText(ctx, err, logLevel)
	}
}

// Close closes the logger (no-op for stderr, closes file for file logger)
func (l *StructuredLogger) Close() error {
	// If the logger is writing to a file, we should close it
	// For now, we'll assume it's stderr and do nothing
	return nil
}

// severityToLogLevel converts error severity to log level
func (l *StructuredLogger) severityToLogLevel(severity types.ErrorSeverity) LogLevel {
	switch severity {
	case types.SeverityInfo:
		return LogLevelInfo
	case types.SeverityWarning:
		return LogLevelWarn
	case types.SeverityError:
		return LogLevelError
	case types.SeverityCritical:
		return LogLevelCritical
	default:
		return LogLevelError
	}
}

// logJSON logs the error in JSON format
func (l *StructuredLogger) logJSON(ctx context.Context, err *types.NLShellError, level LogLevel) {
	logEntry := err.ToMap()
	logEntry["level"] = level.String()

	// Add caller information
	if pc, file, line, ok := runtime.Caller(3); ok {
		logEntry["caller"] = map[string]interface{}{
			"file":     file,
			"line":     line,
			"function": runtime.FuncForPC(pc).Name(),
		}
	}

	// Add context values if available
	if ctx != nil {
		// Try both string keys and custom contextKey type for compatibility
		if userID := ctx.Value("user_id"); userID != nil {
			logEntry["context_user_id"] = userID
		} else if userID := ctx.Value(contextKey("user_id")); userID != nil {
			logEntry["context_user_id"] = userID
		}

		if sessionID := ctx.Value("session_id"); sessionID != nil {
			logEntry["context_session_id"] = sessionID
		} else if sessionID := ctx.Value(contextKey("session_id")); sessionID != nil {
			logEntry["context_session_id"] = sessionID
		}

		if requestID := ctx.Value("request_id"); requestID != nil {
			logEntry["context_request_id"] = requestID
		} else if requestID := ctx.Value(contextKey("request_id")); requestID != nil {
			logEntry["context_request_id"] = requestID
		}
	}

	jsonData, jsonErr := json.Marshal(logEntry)
	if jsonErr != nil {
		// Fallback to text logging if JSON marshaling fails
		l.logger.Printf("ERROR: Failed to marshal log entry to JSON: %v", jsonErr)
		l.logText(ctx, err, level)
		return
	}

	l.logger.Println(string(jsonData))
}

// logText logs the error in human-readable text format
func (l *StructuredLogger) logText(_ context.Context, err *types.NLShellError, level LogLevel) {
	timestamp := err.Timestamp.Format(time.RFC3339)

	logMsg := fmt.Sprintf("[%s] %s %s", timestamp, level.String(), err.Error())

	if err.Component != "" {
		logMsg += fmt.Sprintf(" component=%s", err.Component)
	}
	if err.Operation != "" {
		logMsg += fmt.Sprintf(" operation=%s", err.Operation)
	}
	if err.UserID != "" {
		logMsg += fmt.Sprintf(" user_id=%s", err.UserID)
	}
	if err.SessionID != "" {
		logMsg += fmt.Sprintf(" session_id=%s", err.SessionID)
	}

	// Add context information
	if len(err.Context) > 0 {
		contextStr, _ := json.Marshal(err.Context)
		logMsg += fmt.Sprintf(" context=%s", string(contextStr))
	}

	l.logger.Println(logMsg)
}

// Global logger instance
var globalLogger Logger = NewStructuredLogger(false)

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() Logger {
	return globalLogger
}

// Helper functions for creating common error types

// NewValidationError creates a new validation error
func NewValidationError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewProviderError creates a new provider error
func NewProviderError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeProvider,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewExecutionError creates a new execution error
func NewExecutionError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeExecution,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeConfiguration,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeNetwork,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewPermissionError creates a new permission error
func NewPermissionError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypePermission,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewPluginError creates a new plugin error
func NewPluginError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypePlugin,
		Severity:  types.SeverityWarning, // Plugin errors are typically warnings
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewContextError creates a new context error
func NewContextError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeContext,
		Severity:  types.SeverityWarning, // Context errors are typically warnings
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewUpdateError creates a new update error
func NewUpdateError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeUpdate,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewSafetyError creates a new safety error
func NewSafetyError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeSafety,
		Severity:  types.SeverityCritical, // Safety errors are critical
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeTimeout,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewAuthError creates a new authentication error
func NewAuthError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeAuth,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, cause error) *types.NLShellError {
	return &types.NLShellError{
		Type:      types.ErrTypeInternal,
		Severity:  types.SeverityCritical, // Internal errors are critical
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with additional context
func WrapError(err error, errorType types.ErrorType, message string) *types.NLShellError {
	if err == nil {
		return nil
	}

	// If it's already an NLShellError, preserve the original type unless explicitly overridden
	if nlErr, ok := err.(*types.NLShellError); ok {
		return &types.NLShellError{
			Type:      errorType,
			Severity:  nlErr.Severity,
			Message:   message,
			Cause:     nlErr,
			Timestamp: time.Now(),
			Context:   make(map[string]interface{}),
		}
	}

	return &types.NLShellError{
		Type:      errorType,
		Severity:  types.SeverityError,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// LogAndReturn logs an error and returns it
func LogAndReturn(err *types.NLShellError) *types.NLShellError {
	if err != nil {
		globalLogger.LogError(err)
	}
	return err
}

// LogAndReturnWithContext logs an error with context and returns it
func LogAndReturnWithContext(ctx context.Context, err *types.NLShellError) *types.NLShellError {
	if err != nil {
		globalLogger.LogErrorWithContext(ctx, err)
	}
	return err
}
