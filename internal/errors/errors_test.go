package errors

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType types.ErrorType
		expected  string
	}{
		{types.ErrTypeValidation, "Validation"},
		{types.ErrTypeProvider, "Provider"},
		{types.ErrTypeExecution, "Execution"},
		{types.ErrTypeConfiguration, "Configuration"},
		{types.ErrTypeNetwork, "Network"},
		{types.ErrTypePermission, "Permission"},
		{types.ErrTypePlugin, "Plugin"},
		{types.ErrTypeContext, "Context"},
		{types.ErrTypeUpdate, "Update"},
		{types.ErrTypeSafety, "Safety"},
		{types.ErrTypeTimeout, "Timeout"},
		{types.ErrTypeAuth, "Authentication"},
		{types.ErrTypeInternal, "Internal"},
		{types.ErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.errorType.String(); got != tt.expected {
				t.Errorf("ErrorType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		severity types.ErrorSeverity
		expected string
	}{
		{types.SeverityInfo, "Info"},
		{types.SeverityWarning, "Warning"},
		{types.SeverityError, "Error"},
		{types.SeverityCritical, "Critical"},
		{types.ErrorSeverity(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.expected {
				t.Errorf("ErrorSeverity.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNLShellError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *types.NLShellError
		expected string
	}{
		{
			name: "error without cause",
			err: &types.NLShellError{
				Type:    types.ErrTypeValidation,
				Message: "validation failed",
			},
			expected: "[Validation] validation failed",
		},
		{
			name: "error with cause",
			err: &types.NLShellError{
				Type:    types.ErrTypeNetwork,
				Message: "network request failed",
				Cause:   errors.New("connection timeout"),
			},
			expected: "[Network] network request failed: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("NLShellError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNLShellError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: "validation failed",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("NLShellError.Unwrap() = %v, want %v", unwrapped, cause)
	}

	errNoCause := &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: "validation failed",
	}

	if unwrapped := errNoCause.Unwrap(); unwrapped != nil {
		t.Errorf("NLShellError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestNLShellError_Is(t *testing.T) {
	err1 := &types.NLShellError{Type: types.ErrTypeValidation}
	err2 := &types.NLShellError{Type: types.ErrTypeValidation}
	err3 := &types.NLShellError{Type: types.ErrTypeNetwork}
	regularErr := errors.New("regular error")

	if !err1.Is(err2) {
		t.Error("Expected err1.Is(err2) to be true")
	}

	if err1.Is(err3) {
		t.Error("Expected err1.Is(err3) to be false")
	}

	if err1.Is(regularErr) {
		t.Error("Expected err1.Is(regularErr) to be false")
	}
}

func TestNLShellError_WithContext(t *testing.T) {
	err := &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: "test error",
	}

	err.WithContext("key1", "value1").WithContext("key2", 42)

	if value, exists := err.GetContextValue("key1"); !exists || value != "value1" {
		t.Errorf("Expected context key1 to be 'value1', got %v (exists: %v)", value, exists)
	}

	if value, exists := err.GetContextValue("key2"); !exists || value != 42 {
		t.Errorf("Expected context key2 to be 42, got %v (exists: %v)", value, exists)
	}

	if _, exists := err.GetContextValue("nonexistent"); exists {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestNLShellError_WithComponent(t *testing.T) {
	err := &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: "test error",
	}

	err.WithComponent("test-component")

	if err.Component != "test-component" {
		t.Errorf("Expected component to be 'test-component', got %v", err.Component)
	}
}

func TestNLShellError_WithOperation(t *testing.T) {
	err := &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: "test error",
	}

	err.WithOperation("test-operation")

	if err.Operation != "test-operation" {
		t.Errorf("Expected operation to be 'test-operation', got %v", err.Operation)
	}
}

func TestNLShellError_ToMap(t *testing.T) {
	timestamp := time.Now()
	err := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "test error",
		Cause:     errors.New("cause error"),
		Timestamp: timestamp,
		Component: "test-component",
		Operation: "test-operation",
		UserID:    "user123",
		SessionID: "session456",
		Context:   map[string]interface{}{"key": "value"},
	}

	result := err.ToMap()

	// Check individual fields
	if result["type"] != "Validation" {
		t.Errorf("Expected type to be 'Validation', got %v", result["type"])
	}
	if result["severity"] != "Error" {
		t.Errorf("Expected severity to be 'Error', got %v", result["severity"])
	}
	if result["message"] != "test error" {
		t.Errorf("Expected message to be 'test error', got %v", result["message"])
	}
	if result["timestamp"] != timestamp {
		t.Errorf("Expected timestamp to match, got %v", result["timestamp"])
	}
	if result["component"] != "test-component" {
		t.Errorf("Expected component to be 'test-component', got %v", result["component"])
	}
	if result["operation"] != "test-operation" {
		t.Errorf("Expected operation to be 'test-operation', got %v", result["operation"])
	}
	if result["user_id"] != "user123" {
		t.Errorf("Expected user_id to be 'user123', got %v", result["user_id"])
	}
	if result["session_id"] != "session456" {
		t.Errorf("Expected session_id to be 'session456', got %v", result["session_id"])
	}
	if result["cause"] != "cause error" {
		t.Errorf("Expected cause to be 'cause error', got %v", result["cause"])
	}

	// Check context separately
	if contextValue, exists := result["context"]; !exists {
		t.Error("Expected context to exist in result")
	} else {
		if contextMap, ok := contextValue.(map[string]interface{}); !ok {
			t.Error("Expected context to be a map[string]interface{}")
		} else if contextMap["key"] != "value" {
			t.Errorf("Expected context key to be 'value', got %v", contextMap["key"])
		}
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevelCritical, "CRITICAL"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStructuredLogger_SetLevel(t *testing.T) {
	logger := NewStructuredLogger(false)
	logger.SetLevel(LogLevelError)

	if logger.level != LogLevelError {
		t.Errorf("Expected level to be LogLevelError, got %v", logger.level)
	}
}

func TestStructuredLogger_severityToLogLevel(t *testing.T) {
	logger := NewStructuredLogger(false)

	tests := []struct {
		severity types.ErrorSeverity
		expected LogLevel
	}{
		{types.SeverityInfo, LogLevelInfo},
		{types.SeverityWarning, LogLevelWarn},
		{types.SeverityError, LogLevelError},
		{types.SeverityCritical, LogLevelCritical},
		{types.ErrorSeverity(999), LogLevelError},
	}

	for _, tt := range tests {
		t.Run(tt.severity.String(), func(t *testing.T) {
			if got := logger.severityToLogLevel(tt.severity); got != tt.expected {
				t.Errorf("severityToLogLevel(%v) = %v, want %v", tt.severity, got, tt.expected)
			}
		})
	}
}

func TestStructuredLogger_LogError(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger, err := NewFileLogger(tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	testErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "test error",
		Timestamp: time.Now(),
		Component: "test-component",
	}

	logger.LogError(testErr)

	// Read the log file content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "test error") {
		t.Errorf("Expected log to contain 'test error', got: %s", logContent)
	}
	if !strings.Contains(logContent, "component=test-component") {
		t.Errorf("Expected log to contain component info, got: %s", logContent)
	}
}

func TestStructuredLogger_LogError_JSON(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger, err := NewFileLogger(tmpFile.Name(), true) // JSON mode
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	testErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "test error",
		Timestamp: time.Now(),
		Component: "test-component",
	}

	logger.LogError(testErr)

	// Read the log file content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse as JSON to verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log as JSON: %v", err)
	}

	if logEntry["message"] != "test error" {
		t.Errorf("Expected message to be 'test error', got %v", logEntry["message"])
	}
	if logEntry["component"] != "test-component" {
		t.Errorf("Expected component to be 'test-component', got %v", logEntry["component"])
	}
}

func TestStructuredLogger_LogLevel_Filtering(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger, err := NewFileLogger(tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	// Set log level to ERROR
	logger.SetLevel(LogLevelError)

	// Log a warning (should be filtered out)
	warningErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityWarning,
		Message:   "warning message",
		Timestamp: time.Now(),
	}
	logger.LogError(warningErr)

	// Log an error (should be logged)
	errorErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "error message",
		Timestamp: time.Now(),
	}
	logger.LogError(errorErr)

	// Read the log file content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if strings.Contains(logContent, "warning message") {
		t.Errorf("Expected warning to be filtered out, but found in log: %s", logContent)
	}
	if !strings.Contains(logContent, "error message") {
		t.Errorf("Expected error to be logged, but not found in log: %s", logContent)
	}
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, error) *types.NLShellError
		errType  types.ErrorType
		severity types.ErrorSeverity
	}{
		{"NewValidationError", NewValidationError, types.ErrTypeValidation, types.SeverityError},
		{"NewProviderError", NewProviderError, types.ErrTypeProvider, types.SeverityError},
		{"NewExecutionError", NewExecutionError, types.ErrTypeExecution, types.SeverityError},
		{"NewConfigurationError", NewConfigurationError, types.ErrTypeConfiguration, types.SeverityError},
		{"NewNetworkError", NewNetworkError, types.ErrTypeNetwork, types.SeverityError},
		{"NewPermissionError", NewPermissionError, types.ErrTypePermission, types.SeverityError},
		{"NewPluginError", NewPluginError, types.ErrTypePlugin, types.SeverityWarning},
		{"NewContextError", NewContextError, types.ErrTypeContext, types.SeverityWarning},
		{"NewUpdateError", NewUpdateError, types.ErrTypeUpdate, types.SeverityError},
		{"NewSafetyError", NewSafetyError, types.ErrTypeSafety, types.SeverityCritical},
		{"NewTimeoutError", NewTimeoutError, types.ErrTypeTimeout, types.SeverityError},
		{"NewAuthError", NewAuthError, types.ErrTypeAuth, types.SeverityError},
		{"NewInternalError", NewInternalError, types.ErrTypeInternal, types.SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := errors.New("test cause")
			err := tt.fn("test message", cause)

			if err.Type != tt.errType {
				t.Errorf("Expected type %v, got %v", tt.errType, err.Type)
			}
			if err.Severity != tt.severity {
				t.Errorf("Expected severity %v, got %v", tt.severity, err.Severity)
			}
			if err.Message != "test message" {
				t.Errorf("Expected message 'test message', got %v", err.Message)
			}
			if err.Cause != cause {
				t.Errorf("Expected cause to be set correctly")
			}
			if err.Context == nil {
				t.Error("Expected context to be initialized")
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	t.Run("wrap nil error", func(t *testing.T) {
		result := WrapError(nil, types.ErrTypeValidation, "test message")
		if result != nil {
			t.Errorf("Expected nil result for nil error, got %v", result)
		}
	})

	t.Run("wrap regular error", func(t *testing.T) {
		originalErr := errors.New("original error")
		result := WrapError(originalErr, types.ErrTypeValidation, "wrapped message")

		if result.Type != types.ErrTypeValidation {
			t.Errorf("Expected type ErrTypeValidation, got %v", result.Type)
		}
		if result.Message != "wrapped message" {
			t.Errorf("Expected message 'wrapped message', got %v", result.Message)
		}
		if result.Cause != originalErr {
			t.Errorf("Expected cause to be original error")
		}
	})

	t.Run("wrap NLShellError", func(t *testing.T) {
		originalErr := &types.NLShellError{
			Type:     types.ErrTypeNetwork,
			Severity: types.SeverityCritical,
			Message:  "original message",
		}
		result := WrapError(originalErr, types.ErrTypeValidation, "wrapped message")

		if result.Type != types.ErrTypeValidation {
			t.Errorf("Expected type ErrTypeValidation, got %v", result.Type)
		}
		if result.Severity != types.SeverityCritical {
			t.Errorf("Expected severity to be preserved from original error, got %v", result.Severity)
		}
		if result.Message != "wrapped message" {
			t.Errorf("Expected message 'wrapped message', got %v", result.Message)
		}
		if result.Cause != originalErr {
			t.Errorf("Expected cause to be original error")
		}
	})
}

func TestGlobalLogger(t *testing.T) {
	// Test setting and getting global logger
	originalLogger := GetGlobalLogger()

	newLogger := NewStructuredLogger(true)
	SetGlobalLogger(newLogger)

	if GetGlobalLogger() != newLogger {
		t.Error("Expected global logger to be updated")
	}

	// Restore original logger
	SetGlobalLogger(originalLogger)
}

func TestLogAndReturn(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger, err := NewFileLogger(tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	// Set as global logger
	originalLogger := GetGlobalLogger()
	SetGlobalLogger(logger)
	defer SetGlobalLogger(originalLogger)

	testErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "test error for LogAndReturn",
		Timestamp: time.Now(),
	}

	result := LogAndReturn(testErr)

	// Should return the same error
	if result != testErr {
		t.Error("Expected LogAndReturn to return the same error")
	}

	// Should have logged the error
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test error for LogAndReturn") {
		t.Error("Expected error to be logged")
	}
}

func TestLogAndReturnWithContext(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	logger, err := NewFileLogger(tmpFile.Name(), true) // JSON mode to check context
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	// Set as global logger
	originalLogger := GetGlobalLogger()
	SetGlobalLogger(logger)
	defer SetGlobalLogger(originalLogger)

	ctx := context.WithValue(context.Background(), "user_id", "test-user")
	testErr := &types.NLShellError{
		Type:      types.ErrTypeValidation,
		Severity:  types.SeverityError,
		Message:   "test error with context",
		Timestamp: time.Now(),
	}

	result := LogAndReturnWithContext(ctx, testErr)

	// Should return the same error
	if result != testErr {
		t.Error("Expected LogAndReturnWithContext to return the same error")
	}

	// Should have logged the error with context
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log as JSON: %v", err)
	}

	if logEntry["context_user_id"] != "test-user" {
		t.Error("Expected context user_id to be logged")
	}
}
