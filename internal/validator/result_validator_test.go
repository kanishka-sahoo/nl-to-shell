package validator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// MockLLMProvider implements the LLMProvider interface for testing
type MockLLMProvider struct {
	validateResultFunc  func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error)
	generateCommandFunc func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error)
	providerInfo        types.ProviderInfo
}

func (m *MockLLMProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if m.validateResultFunc != nil {
		return m.validateResultFunc(ctx, command, output, intent)
	}
	return &types.ValidationResponse{
		IsCorrect:   true,
		Explanation: "Mock validation passed",
		Suggestions: []string{"No suggestions"},
		Correction:  "",
	}, nil
}

func (m *MockLLMProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if m.generateCommandFunc != nil {
		return m.generateCommandFunc(ctx, prompt, context)
	}
	return &types.CommandResponse{
		Command:      "echo corrected",
		Explanation:  "Mock correction",
		Confidence:   0.8,
		Alternatives: []string{},
	}, nil
}

func (m *MockLLMProvider) GetProviderInfo() types.ProviderInfo {
	return m.providerInfo
}

func TestNewResultValidator(t *testing.T) {
	provider := &MockLLMProvider{}
	validator := NewResultValidator(provider)

	if validator == nil {
		t.Fatal("NewResultValidator returned nil")
	}

	// Verify it implements the interface
	if _, ok := validator.(*ResultValidator); !ok {
		t.Error("NewResultValidator did not return a *ResultValidator")
	}
}

func TestResultValidator_ValidateResult_NilResult(t *testing.T) {
	provider := &MockLLMProvider{}
	validator := NewResultValidator(provider)
	ctx := context.Background()

	result, err := validator.ValidateResult(ctx, nil, "test intent")
	if err == nil {
		t.Error("Expected error for nil result")
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}
}

func TestResultValidator_ValidateResult_NilProvider(t *testing.T) {
	validator := NewResultValidator(nil)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "echo hello",
		},
		ExitCode: 0,
		Success:  true,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "test intent")
	if err == nil {
		t.Error("Expected error for nil provider")
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}
}

func TestResultValidator_ValidateResult_Success(t *testing.T) {
	provider := &MockLLMProvider{
		validateResultFunc: func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
			return &types.ValidationResponse{
				IsCorrect:   true,
				Explanation: "Command executed successfully and achieved the intent",
				Suggestions: []string{"Great job!"},
				Correction:  "",
			}, nil
		},
	}

	validator := NewResultValidator(provider)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "echo hello",
		},
		ExitCode: 0,
		Stdout:   "hello\n",
		Success:  true,
		Duration: 100 * time.Millisecond,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "print hello")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result")
	}
	if !result.IsCorrect {
		t.Error("Expected validation to pass")
	}
	if result.Explanation == "" {
		t.Error("Expected explanation")
	}
	if len(result.Suggestions) == 0 {
		t.Error("Expected suggestions")
	}
}

func TestResultValidator_ValidateResult_Failure(t *testing.T) {
	provider := &MockLLMProvider{
		validateResultFunc: func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
			return &types.ValidationResponse{
				IsCorrect:   false,
				Explanation: "Command failed to achieve the intent",
				Suggestions: []string{"Try using a different approach"},
				Correction:  "ls -la",
			}, nil
		},
	}

	validator := NewResultValidator(provider)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "ls /nonexistent",
		},
		ExitCode: 2,
		Stderr:   "ls: /nonexistent: No such file or directory\n",
		Success:  false,
		Duration: 50 * time.Millisecond,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "list files")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result")
	}
	if result.IsCorrect {
		t.Error("Expected validation to fail")
	}
	if result.CorrectedCommand == "" {
		t.Error("Expected corrected command")
	}
}

func TestResultValidator_ValidateResult_ProviderError(t *testing.T) {
	provider := &MockLLMProvider{
		validateResultFunc: func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
			return nil, errors.New("provider error")
		},
	}

	validator := NewResultValidator(provider)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "echo hello",
		},
		ExitCode: 0,
		Success:  true,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "test intent")
	if err == nil {
		t.Error("Expected error from provider")
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}
}

func TestResultValidator_ValidateResult_AutoCorrection(t *testing.T) {
	provider := &MockLLMProvider{
		validateResultFunc: func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
			return &types.ValidationResponse{
				IsCorrect:   false,
				Explanation: "Command failed",
				Suggestions: []string{"Try again"},
				Correction:  "", // No correction provided
			}, nil
		},
		generateCommandFunc: func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
			return &types.CommandResponse{
				Command:     "ls -la",
				Explanation: "Corrected command",
				Confidence:  0.9,
			}, nil
		},
	}

	validator := NewResultValidator(provider)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "ls /nonexistent",
			Context:   &types.Context{},
		},
		ExitCode: 2,
		Success:  false,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "list files")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result")
	}
	if result.CorrectedCommand == "" {
		t.Error("Expected auto-generated correction")
	}
}

func TestResultValidator_buildValidationPrompt(t *testing.T) {
	validator := &ResultValidator{}

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "echo hello",
		},
		ExitCode: 0,
		Stdout:   "hello\n",
		Success:  true,
		Duration: 100 * time.Millisecond,
	}

	prompt := validator.buildValidationPrompt(executionResult, "print hello")

	expectedContents := []string{
		"User Intent: print hello",
		"Command Executed: echo hello",
		"Exit Code: 0",
		"Success: true",
		"Standard Output:",
		"hello",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Expected prompt to contain %q, got: %s", expected, prompt)
		}
	}
}

func TestResultValidator_formatOutput(t *testing.T) {
	validator := &ResultValidator{}

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "echo hello",
		},
		ExitCode: 0,
		Stdout:   "hello\n",
		Stderr:   "warning\n",
		Success:  true,
		Duration: 100 * time.Millisecond,
		Error:    errors.New("test error"),
	}

	output := validator.formatOutput(executionResult)

	expectedContents := []string{
		"STDOUT:",
		"hello",
		"STDERR:",
		"warning",
		"ERROR:",
		"test error",
		"EXIT_CODE: 0",
		"SUCCESS: true",
		"DURATION:",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, got: %s", expected, output)
		}
	}
}

func TestNewAdvancedResultValidator(t *testing.T) {
	provider := &MockLLMProvider{}
	validator := NewAdvancedResultValidator(provider, true)

	if validator == nil {
		t.Fatal("NewAdvancedResultValidator returned nil")
	}

	// Verify it implements the interface
	if _, ok := validator.(*AdvancedResultValidator); !ok {
		t.Error("NewAdvancedResultValidator did not return an *AdvancedResultValidator")
	}
}

func TestAdvancedResultValidator_ValidateResult_WithAutoCorrection(t *testing.T) {
	provider := &MockLLMProvider{
		validateResultFunc: func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
			return &types.ValidationResponse{
				IsCorrect:   false,
				Explanation: "Command failed to achieve intent",
				Suggestions: []string{"Try a different approach"},
				Correction:  "",
			}, nil
		},
		generateCommandFunc: func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
			return &types.CommandResponse{
				Command:     "ls -la",
				Explanation: "Corrected command",
				Confidence:  0.9,
			}, nil
		},
	}

	validator := NewAdvancedResultValidator(provider, true)
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "ls /nonexistent",
			Context:   &types.Context{},
		},
		ExitCode: 2,
		Success:  false,
	}

	result, err := validator.ValidateResult(ctx, executionResult, "list files")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result")
	}
	if result.CorrectedCommand == "" {
		t.Error("Expected auto-generated correction")
	}

	// Should have added a suggestion about the auto-correction
	found := false
	for _, suggestion := range result.Suggestions {
		if strings.Contains(suggestion, "Auto-generated correction") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about auto-generated correction")
	}
}

func TestAdvancedResultValidator_isReasonableCorrection(t *testing.T) {
	validator := &AdvancedResultValidator{}

	tests := []struct {
		name       string
		original   *types.Command
		corrected  *types.Command
		intent     string
		reasonable bool
	}{
		{
			name: "valid correction",
			original: &types.Command{
				Generated: "ls /nonexistent",
			},
			corrected: &types.Command{
				Generated: "ls -la",
			},
			intent:     "list files",
			reasonable: true,
		},
		{
			name: "identical commands",
			original: &types.Command{
				Generated: "ls -la",
			},
			corrected: &types.Command{
				Generated: "ls -la",
			},
			intent:     "list files",
			reasonable: false,
		},
		{
			name: "empty correction",
			original: &types.Command{
				Generated: "ls /nonexistent",
			},
			corrected: &types.Command{
				Generated: "",
			},
			intent:     "list files",
			reasonable: false,
		},
		{
			name: "dangerous correction",
			original: &types.Command{
				Generated: "ls /tmp",
			},
			corrected: &types.Command{
				Generated: "rm -rf /",
			},
			intent:     "clean directory",
			reasonable: false,
		},
		{
			name: "excessively long correction",
			original: &types.Command{
				Generated: "ls",
			},
			corrected: &types.Command{
				Generated: strings.Repeat("echo ", 100) + "hello",
			},
			intent:     "list files",
			reasonable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isReasonableCorrection(tt.original, tt.corrected, tt.intent)
			if result != tt.reasonable {
				t.Errorf("Expected isReasonableCorrection to return %v, got %v", tt.reasonable, result)
			}
		})
	}
}

func TestResultValidator_generateCorrection(t *testing.T) {
	provider := &MockLLMProvider{
		generateCommandFunc: func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
			// Verify the prompt contains expected elements
			if !strings.Contains(prompt, "corrected command") {
				t.Error("Expected prompt to request corrected command")
			}
			return &types.CommandResponse{
				Command:     "ls -la",
				Explanation: "Corrected command",
				Confidence:  0.9,
			}, nil
		},
	}

	validator := &ResultValidator{llmProvider: provider}
	ctx := context.Background()

	executionResult := &types.ExecutionResult{
		Command: &types.Command{
			ID:        "test",
			Generated: "ls /nonexistent",
			Context:   &types.Context{},
		},
		ExitCode: 2,
		Stderr:   "No such file or directory",
		Error:    errors.New("command failed"),
	}

	correction, err := validator.generateCorrection(ctx, executionResult, "list files", "Directory not found")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if correction == "" {
		t.Error("Expected non-empty correction")
	}
	if correction != "ls -la" {
		t.Errorf("Expected correction 'ls -la', got %q", correction)
	}
}
