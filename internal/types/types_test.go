package types

import (
	"errors"
	"testing"
	"time"
)

func TestAuditAction_String(t *testing.T) {
	tests := []struct {
		name     string
		action   AuditAction
		expected string
	}{
		{"Validated", AuditActionValidated, "Validated"},
		{"Bypassed", AuditActionBypassed, "Bypassed"},
		{"Blocked", AuditActionBlocked, "Blocked"},
		{"Overridden", AuditActionOverridden, "Overridden"},
		{"Unknown", AuditAction(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("AuditAction.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDangerLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    DangerLevel
		expected string
	}{
		{"Safe", Safe, "Safe"},
		{"Warning", Warning, "Warning"},
		{"Dangerous", Dangerous, "Dangerous"},
		{"Critical", Critical, "Critical"},
		{"Unknown", DangerLevel(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("DangerLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		expected string
	}{
		{"Validation", ErrTypeValidation, "Validation"},
		{"Provider", ErrTypeProvider, "Provider"},
		{"Execution", ErrTypeExecution, "Execution"},
		{"Configuration", ErrTypeConfiguration, "Configuration"},
		{"Network", ErrTypeNetwork, "Network"},
		{"Permission", ErrTypePermission, "Permission"},
		{"Plugin", ErrTypePlugin, "Plugin"},
		{"Context", ErrTypeContext, "Context"},
		{"Update", ErrTypeUpdate, "Update"},
		{"Safety", ErrTypeSafety, "Safety"},
		{"Timeout", ErrTypeTimeout, "Timeout"},
		{"Auth", ErrTypeAuth, "Authentication"},
		{"Internal", ErrTypeInternal, "Internal"},
		{"Unknown", ErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.errType.String(); got != tt.expected {
				t.Errorf("ErrorType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{"Info", SeverityInfo, "Info"},
		{"Warning", SeverityWarning, "Warning"},
		{"Error", SeverityError, "Error"},
		{"Critical", SeverityCritical, "Critical"},
		{"Unknown", ErrorSeverity(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.expected {
				t.Errorf("ErrorSeverity.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNLShellError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NLShellError
		expected string
	}{
		{
			name: "error without cause",
			err: &NLShellError{
				Type:    ErrTypeValidation,
				Message: "validation failed",
			},
			expected: "[Validation] validation failed",
		},
		{
			name: "error with cause",
			err: &NLShellError{
				Type:    ErrTypeNetwork,
				Message: "network error",
				Cause:   errors.New("connection refused"),
			},
			expected: "[Network] network error: connection refused",
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
	cause := errors.New("underlying error")
	err := &NLShellError{
		Type:  ErrTypeInternal,
		Cause: cause,
	}

	if got := err.Unwrap(); got != cause {
		t.Errorf("NLShellError.Unwrap() = %v, want %v", got, cause)
	}

	// Test nil cause
	errNoCause := &NLShellError{Type: ErrTypeInternal}
	if got := errNoCause.Unwrap(); got != nil {
		t.Errorf("NLShellError.Unwrap() = %v, want nil", got)
	}
}

func TestNLShellError_Is(t *testing.T) {
	err1 := &NLShellError{Type: ErrTypeValidation}
	err2 := &NLShellError{Type: ErrTypeValidation}
	err3 := &NLShellError{Type: ErrTypeNetwork}
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
	err := &NLShellError{Type: ErrTypeInternal}

	result := err.WithContext("key1", "value1")
	if result != err {
		t.Error("WithContext should return the same error instance")
	}

	if err.Context == nil {
		t.Error("Context should be initialized")
	}

	if val, exists := err.Context["key1"]; !exists || val != "value1" {
		t.Error("Context value not set correctly")
	}

	// Test adding to existing context
	err.WithContext("key2", 42)
	if val, exists := err.Context["key2"]; !exists || val != 42 {
		t.Error("Second context value not set correctly")
	}
}

func TestNLShellError_WithComponent(t *testing.T) {
	err := &NLShellError{Type: ErrTypeInternal}
	result := err.WithComponent("test-component")

	if result != err {
		t.Error("WithComponent should return the same error instance")
	}

	if err.Component != "test-component" {
		t.Errorf("Component = %v, want test-component", err.Component)
	}
}

func TestNLShellError_WithOperation(t *testing.T) {
	err := &NLShellError{Type: ErrTypeInternal}
	result := err.WithOperation("test-operation")

	if result != err {
		t.Error("WithOperation should return the same error instance")
	}

	if err.Operation != "test-operation" {
		t.Errorf("Operation = %v, want test-operation", err.Operation)
	}
}

func TestNLShellError_WithUserID(t *testing.T) {
	err := &NLShellError{Type: ErrTypeInternal}
	result := err.WithUserID("user123")

	if result != err {
		t.Error("WithUserID should return the same error instance")
	}

	if err.UserID != "user123" {
		t.Errorf("UserID = %v, want user123", err.UserID)
	}
}

func TestNLShellError_WithSessionID(t *testing.T) {
	err := &NLShellError{Type: ErrTypeInternal}
	result := err.WithSessionID("session456")

	if result != err {
		t.Error("WithSessionID should return the same error instance")
	}

	if err.SessionID != "session456" {
		t.Errorf("SessionID = %v, want session456", err.SessionID)
	}
}

func TestNLShellError_GetContextValue(t *testing.T) {
	err := &NLShellError{Type: ErrTypeInternal}

	// Test with nil context
	val, exists := err.GetContextValue("key1")
	if exists || val != nil {
		t.Error("GetContextValue should return false for nil context")
	}

	// Test with existing context
	err.WithContext("key1", "value1")
	val, exists = err.GetContextValue("key1")
	if !exists || val != "value1" {
		t.Error("GetContextValue should return the correct value")
	}

	// Test with non-existent key
	val, exists = err.GetContextValue("nonexistent")
	if exists || val != nil {
		t.Error("GetContextValue should return false for non-existent key")
	}
}

func TestNLShellError_ToMap(t *testing.T) {
	timestamp := time.Now()
	err := &NLShellError{
		Type:      ErrTypeValidation,
		Severity:  SeverityError,
		Message:   "test message",
		Cause:     errors.New("test cause"),
		Timestamp: timestamp,
		Component: "test-component",
		Operation: "test-operation",
		UserID:    "user123",
		SessionID: "session456",
	}
	err.WithContext("key1", "value1")

	result := err.ToMap()

	// Check individual fields
	if result["type"] != "Validation" {
		t.Errorf("ToMap()[type] = %v, want Validation", result["type"])
	}
	if result["severity"] != "Error" {
		t.Errorf("ToMap()[severity] = %v, want Error", result["severity"])
	}
	if result["message"] != "test message" {
		t.Errorf("ToMap()[message] = %v, want test message", result["message"])
	}
	if result["timestamp"] != timestamp {
		t.Errorf("ToMap()[timestamp] = %v, want %v", result["timestamp"], timestamp)
	}
	if result["component"] != "test-component" {
		t.Errorf("ToMap()[component] = %v, want test-component", result["component"])
	}
	if result["operation"] != "test-operation" {
		t.Errorf("ToMap()[operation] = %v, want test-operation", result["operation"])
	}
	if result["user_id"] != "user123" {
		t.Errorf("ToMap()[user_id] = %v, want user123", result["user_id"])
	}
	if result["session_id"] != "session456" {
		t.Errorf("ToMap()[session_id] = %v, want session456", result["session_id"])
	}
	if result["cause"] != "test cause" {
		t.Errorf("ToMap()[cause] = %v, want test cause", result["cause"])
	}

	// Check context separately
	contextMap, ok := result["context"].(map[string]interface{})
	if !ok {
		t.Error("ToMap()[context] should be a map[string]interface{}")
	} else {
		if contextMap["key1"] != "value1" {
			t.Errorf("ToMap()[context][key1] = %v, want value1", contextMap["key1"])
		}
	}
}

func TestNLShellError_ToMap_MinimalError(t *testing.T) {
	timestamp := time.Now()
	err := &NLShellError{
		Type:      ErrTypeInternal,
		Severity:  SeverityInfo,
		Message:   "minimal error",
		Timestamp: timestamp,
	}

	result := err.ToMap()

	expected := map[string]interface{}{
		"type":      "Internal",
		"severity":  "Info",
		"message":   "minimal error",
		"timestamp": timestamp,
	}

	for key, expectedVal := range expected {
		if result[key] != expectedVal {
			t.Errorf("ToMap()[%s] = %v, want %v", key, result[key], expectedVal)
		}
	}

	// Check that optional fields are not present
	optionalFields := []string{"component", "operation", "user_id", "session_id", "cause", "context"}
	for _, field := range optionalFields {
		if _, exists := result[field]; exists {
			t.Errorf("ToMap() should not contain %s for minimal error", field)
		}
	}
}
