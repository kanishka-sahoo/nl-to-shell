package executor

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestNewExecutor(t *testing.T) {
	executor := NewExecutor()
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}

	// Test that it implements the interface
	if _, ok := executor.(*Executor); !ok {
		t.Error("NewExecutor() did not return an *Executor")
	}
}

func TestNewExecutorWithTimeout(t *testing.T) {
	timeout := 10 * time.Second
	executor := NewExecutorWithTimeout(timeout)
	if executor == nil {
		t.Fatal("NewExecutorWithTimeout() returned nil")
	}

	exec := executor.(*Executor)
	if exec.defaultTimeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, exec.defaultTimeout)
	}
}

func TestExecutor_Execute_NilCommand(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	result, err := executor.Execute(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil command")
	}
	if result == nil {
		t.Error("Expected result even for error case")
	}
}

func TestExecutor_Execute_EmptyCommand(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	cmd := &types.Command{
		ID:        "test-1",
		Generated: "",
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err == nil {
		t.Error("Expected error for empty command")
	}
	if result == nil {
		t.Error("Expected result even for error case")
		return
	}
	if result.Success {
		t.Error("Expected command to fail")
	}
}

func TestExecutor_Execute_SimpleCommand(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	// Use a command that works on all platforms
	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "echo hello"
	} else {
		cmdStr = "echo hello"
	}

	cmd := &types.Command{
		ID:        "test-2",
		Generated: cmdStr,
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.Success {
		t.Errorf("Expected command to succeed, got exit code %d, stderr: %s", result.ExitCode, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Expected stdout to contain 'hello', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_CommandWithArgs(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	cmd := &types.Command{
		ID:        "test-3",
		Generated: "echo hello world",
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.Success {
		t.Errorf("Expected command to succeed, got exit code %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_CommandWithQuotes(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	cmd := &types.Command{
		ID:        "test-4",
		Generated: `echo "hello world"`,
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.Success {
		t.Errorf("Expected command to succeed, got exit code %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_NonExistentCommand(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	cmd := &types.Command{
		ID:        "test-5",
		Generated: "nonexistentcommand12345",
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	// Non-existent command should return an error during execution
	if err == nil && result != nil && result.Success {
		t.Error("Expected command to fail for non-existent command")
	}
	if result == nil {
		t.Error("Expected result even for error case")
	}
	if result != nil && result.Success {
		t.Error("Expected command to fail")
	}
}

func TestExecutor_Execute_WithTimeout(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	// Create a command that will timeout
	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "timeout 5" // Windows timeout command
	} else {
		cmdStr = "sleep 5" // Unix sleep command
	}

	cmd := &types.Command{
		ID:        "test-6",
		Generated: cmdStr,
		Timeout:   1 * time.Second, // Short timeout
		Timestamp: time.Now(),
	}

	start := time.Now()
	result, err := executor.Execute(ctx, cmd)
	duration := time.Since(start)

	// Should timeout within reasonable time
	if duration > 2*time.Second {
		t.Errorf("Command took too long to timeout: %v", duration)
	}

	// We expect either an error or a failed result due to timeout
	if err != nil {
		t.Logf("Command failed with error (expected): %v", err)
	}

	if result == nil {
		t.Fatal("Expected result")
	}
	if result.Success {
		t.Error("Expected command to fail due to timeout")
	}
}

func TestExecutor_Execute_WithWorkingDirectory(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	// Create a temporary directory
	tempDir := t.TempDir()

	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "cd"
	} else {
		cmdStr = "pwd"
	}

	cmd := &types.Command{
		ID:         "test-7",
		Generated:  cmdStr,
		WorkingDir: tempDir,
		Timestamp:  time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.Success {
		t.Errorf("Expected command to succeed, got exit code %d", result.ExitCode)
	}

	// Check that the command ran in the correct directory
	if !strings.Contains(result.Stdout, tempDir) {
		t.Errorf("Expected stdout to contain temp directory path %s, got: %s", tempDir, result.Stdout)
	}
}

func TestExecutor_Execute_WithEnvironment(t *testing.T) {
	executor := NewExecutor()
	ctx := context.Background()

	// Use a more reliable approach - use shell to expand variables
	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "cmd /c echo %TEST_VAR%"
	} else {
		cmdStr = "sh -c 'echo $TEST_VAR'"
	}

	cmd := &types.Command{
		ID:        "test-8",
		Generated: cmdStr,
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
		Timestamp: time.Now(),
	}

	result, err := executor.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.Success {
		t.Errorf("Expected command to succeed, got exit code %d, stderr: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("Expected stdout to contain 'test_value', got: %s", result.Stdout)
	}
}

func TestExecutor_DryRun_NilCommand(t *testing.T) {
	executor := NewExecutor()

	result, err := executor.DryRun(nil)
	if err == nil {
		t.Error("Expected error for nil command")
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}
}

func TestExecutor_DryRun_EmptyCommand(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:        "test-9",
		Generated: "",
		Timestamp: time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if !strings.Contains(result.Analysis, "parsing failed") && !strings.Contains(result.Analysis, "Empty command") {
		t.Errorf("Expected analysis to mention parsing failure or empty command, got: %s", result.Analysis)
	}
}

func TestExecutor_DryRun_ValidCommand(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:        "test-10",
		Generated: "echo hello",
		Timestamp: time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	if result.Command != cmd {
		t.Error("Expected result to reference the original command")
	}
	if result.Analysis == "" {
		t.Error("Expected non-empty analysis")
	}
	if len(result.Predictions) == 0 {
		t.Error("Expected predictions")
	}
}

func TestExecutor_DryRun_NonExistentCommand(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:        "test-11",
		Generated: "nonexistentcommand12345",
		Timestamp: time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	// Should predict that command won't be found
	found := false
	for _, prediction := range result.Predictions {
		if strings.Contains(prediction, "not found") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected prediction about command not being found")
	}
}

func TestExecutor_parseCommand(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name     string
		input    string
		expected []string
		hasError bool
	}{
		{
			name:     "simple command",
			input:    "echo hello",
			expected: []string{"echo", "hello"},
			hasError: false,
		},
		{
			name:     "command with quotes",
			input:    `echo "hello world"`,
			expected: []string{"echo", "hello world"},
			hasError: false,
		},
		{
			name:     "command with single quotes",
			input:    `echo 'hello world'`,
			expected: []string{"echo", "hello world"},
			hasError: false,
		},
		{
			name:     "empty command",
			input:    "",
			expected: nil,
			hasError: true,
		},
		{
			name:     "unclosed quotes",
			input:    `echo "hello`,
			expected: nil,
			hasError: true,
		},
		{
			name:     "multiple arguments",
			input:    "ls -la /tmp",
			expected: []string{"ls", "-la", "/tmp"},
			hasError: false,
		},
		{
			name:     "command with tabs",
			input:    "echo\thello\tworld",
			expected: []string{"echo", "hello", "world"},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.parseCommand(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d for input %q", len(tt.expected), len(result), tt.input)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected part %d to be %q, got %q for input %q", i, expected, result[i], tt.input)
				}
			}
		})
	}
}

func TestExecutor_analyzeCommand(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name     string
		cmdParts []string
		contains string
	}{
		{
			name:     "ls command",
			cmdParts: []string{"ls", "-la"},
			contains: "list directory contents",
		},
		{
			name:     "rm command",
			cmdParts: []string{"rm", "file.txt"},
			contains: "remove files",
		},
		{
			name:     "unknown command",
			cmdParts: []string{"unknowncmd"},
			contains: "execute the specified command",
		},
		{
			name:     "empty command",
			cmdParts: []string{},
			contains: "No command to analyze",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := executor.analyzeCommand(tt.cmdParts)
			if !strings.Contains(strings.ToLower(analysis), strings.ToLower(tt.contains)) {
				t.Errorf("Expected analysis to contain %q, got: %s", tt.contains, analysis)
			}
		})
	}
}

func TestExecutor_generatePredictions(t *testing.T) {
	executor := &Executor{}

	cmd := &types.Command{
		ID:        "test-12",
		Generated: "echo hello",
		Timestamp: time.Now(),
	}

	predictions := executor.generatePredictions([]string{"echo", "hello"}, cmd)

	if len(predictions) == 0 {
		t.Error("Expected at least one prediction")
	}

	// Should predict that echo is found in PATH
	found := false
	for _, prediction := range predictions {
		if strings.Contains(prediction, "found in PATH") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected prediction about command being found in PATH")
	}
}

func TestExecutor_Execute_ContextCancellation(t *testing.T) {
	executor := NewExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Use a longer running command and cancel after a short delay
	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "timeout 2"
	} else {
		cmdStr = "sleep 2"
	}

	cmd := &types.Command{
		ID:        "test-13",
		Generated: cmdStr,
		Timestamp: time.Now(),
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := executor.Execute(ctx, cmd)
	// Context cancellation should cause the command to fail
	if err == nil && result != nil && result.Success {
		t.Error("Expected command to fail due to context cancellation")
	}
	if result == nil {
		t.Error("Expected result even for error case")
		return
	}
	if result.Success {
		t.Error("Expected command to fail due to context cancellation")
	}
}

func TestExecutor_DryRun_DetailedAnalysis(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:         "test-detailed",
		Generated:  "ls -la /tmp",
		WorkingDir: "/tmp",
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
		Timeout:   5 * time.Second,
		Timestamp: time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	// Check that analysis includes validation results
	if !strings.Contains(result.Analysis, "Validation Results:") {
		t.Error("Expected analysis to include validation results section")
	}

	// Check for executable validation
	if !strings.Contains(result.Analysis, "found in PATH") {
		t.Error("Expected analysis to include executable path validation")
	}

	// Check for working directory validation
	if !strings.Contains(result.Analysis, "Working directory") {
		t.Error("Expected analysis to include working directory validation")
	}

	// Check for environment variables
	if !strings.Contains(result.Analysis, "environment variables") {
		t.Error("Expected analysis to include environment variables information")
	}

	// Check for timeout information
	if !strings.Contains(result.Analysis, "Timeout") {
		t.Error("Expected analysis to include timeout information")
	}
}

func TestExecutor_DryRun_CommandValidation_RM(t *testing.T) {
	executor := NewExecutor()

	tests := []struct {
		name     string
		command  string
		contains []string
	}{
		{
			name:     "rm without args",
			command:  "rm",
			contains: []string{"requires at least one argument"},
		},
		{
			name:     "rm with recursive force",
			command:  "rm -rf /tmp/test",
			contains: []string{"recursive force delete", "WARNING"},
		},
		{
			name:     "rm with files",
			command:  "rm file1.txt file2.txt",
			contains: []string{"will operate on 2 target(s)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test-rm",
				Generated: tt.command,
				Timestamp: time.Now(),
			}

			result, err := executor.DryRun(cmd)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result")
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result.Analysis, expected) {
					t.Errorf("Expected analysis to contain %q, got: %s", expected, result.Analysis)
				}
			}
		})
	}
}

func TestExecutor_DryRun_CommandValidation_CP(t *testing.T) {
	executor := NewExecutor()

	tests := []struct {
		name     string
		command  string
		contains string
	}{
		{
			name:     "cp without enough args",
			command:  "cp file1.txt",
			contains: "requires source and destination",
		},
		{
			name:     "cp with valid args",
			command:  "cp file1.txt file2.txt",
			contains: "has source and destination specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test-cp",
				Generated: tt.command,
				Timestamp: time.Now(),
			}

			result, err := executor.DryRun(cmd)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result")
			}

			if !strings.Contains(result.Analysis, tt.contains) {
				t.Errorf("Expected analysis to contain %q, got: %s", tt.contains, result.Analysis)
			}
		})
	}
}

func TestExecutor_DryRun_CommandValidation_SUDO(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:        "test-sudo",
		Generated: "sudo ls -la",
		Timestamp: time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	// Should warn about elevated privileges
	if !strings.Contains(result.Analysis, "elevated privileges") {
		t.Error("Expected analysis to warn about elevated privileges")
	}
	if !strings.Contains(result.Analysis, "WARNING") {
		t.Error("Expected analysis to include WARNING for sudo")
	}
}

func TestExecutor_DryRun_NonExistentWorkingDirectory(t *testing.T) {
	executor := NewExecutor()

	cmd := &types.Command{
		ID:         "test-nonexistent-dir",
		Generated:  "ls",
		WorkingDir: "/nonexistent/directory/path",
		Timestamp:  time.Now(),
	}

	result, err := executor.DryRun(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	// Should indicate that working directory doesn't exist
	if !strings.Contains(result.Analysis, "does not exist") {
		t.Error("Expected analysis to indicate working directory doesn't exist")
	}
}

func TestExecutor_validateCommandStructure(t *testing.T) {
	executor := &Executor{defaultTimeout: 30 * time.Second}

	cmd := &types.Command{
		ID:         "test-validate",
		Generated:  "echo hello",
		WorkingDir: "/tmp",
		Environment: map[string]string{
			"TEST": "value",
		},
		Timeout:   10 * time.Second,
		Timestamp: time.Now(),
	}

	result := executor.validateCommandStructure([]string{"echo", "hello"}, cmd)

	// Should contain various validation checks
	expectedChecks := []string{
		"found in PATH",
		"Working directory",
		"environment variables",
		"Timeout",
	}

	for _, check := range expectedChecks {
		if !strings.Contains(result, check) {
			t.Errorf("Expected validation to contain %q, got: %s", check, result)
		}
	}
}

func TestExecutor_validateCommandArguments(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name       string
		executable string
		args       []string
		contains   string
	}{
		{
			name:       "mkdir with args",
			executable: "mkdir",
			args:       []string{"dir1", "dir2"},
			contains:   "will create 2 director(ies)",
		},
		{
			name:       "mkdir without args",
			executable: "mkdir",
			args:       []string{},
			contains:   "requires at least one directory name",
		},
		{
			name:       "cd with one arg",
			executable: "cd",
			args:       []string{"/tmp"},
			contains:   "will change to '/tmp'",
		},
		{
			name:       "cd without args",
			executable: "cd",
			args:       []string{},
			contains:   "will change to home directory",
		},
		{
			name:       "kill with process ids",
			executable: "kill",
			args:       []string{"1234", "5678"},
			contains:   "will target 2 process(es)",
		},
		{
			name:       "unknown command",
			executable: "unknowncmd",
			args:       []string{"arg1"},
			contains:   "structure appears valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateCommandArguments(tt.executable, tt.args)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected validation to contain %q, got: %s", tt.contains, result)
			}
		})
	}
}
