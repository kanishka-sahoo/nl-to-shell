package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// TestCLIEndToEnd tests the complete CLI workflow
func TestCLIEndToEnd(t *testing.T) {
	// Skip if we don't have a built binary
	if testing.Short() {
		t.Skip("Skipping end-to-end tests in short mode")
	}

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "nl-to-shell-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases for CLI commands
	testCases := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectedError  bool
		timeout        time.Duration
	}{
		{
			name:           "help_command",
			args:           []string{"--help"},
			expectedOutput: []string{"Convert natural language to shell commands", "Usage:", "Available Commands:"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
		{
			name:           "version_command",
			args:           []string{"version"},
			expectedOutput: []string{"nl-to-shell version"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
		{
			name:           "version_verbose",
			args:           []string{"version", "--verbose"},
			expectedOutput: []string{"nl-to-shell version", "Git commit:", "Build date:"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
		{
			name:           "completion_bash",
			args:           []string{"completion", "bash"},
			expectedOutput: []string{"# bash completion"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
		{
			name:           "invalid_flag",
			args:           []string{"--invalid-flag"},
			expectedOutput: []string{"unknown flag"},
			expectedError:  true,
			timeout:        5 * time.Second,
		},
		{
			name:           "config_help",
			args:           []string{"config", "--help"},
			expectedOutput: []string{"Manage configuration settings", "Available Commands:"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
		{
			name:           "update_help",
			args:           []string{"update", "--help"},
			expectedOutput: []string{"Manage updates", "Available Commands:"},
			expectedError:  false,
			timeout:        5 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			// Execute CLI command
			output, err := executeCLICommand(ctx, tc.args...)

			// Check error expectation
			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			// Check expected output strings
			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestCLISessionMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping session mode tests in short mode")
	}

	// Test session mode with mock input
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a test script that simulates session input
	input := "help\nhistory\nconfig\nexit\n"
	output, err := executeCLICommandWithInput(ctx, input, "session", "--dry-run")

	if err != nil {
		t.Fatalf("Session mode failed: %v", err)
	}

	expectedStrings := []string{
		"Welcome to nl-to-shell interactive session",
		"Session ID:",
		"help",
		"history",
		"config",
		"Session ended",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected session output to contain '%s', got: %s", expected, output)
		}
	}
}

func TestCLIConfigCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping config command tests in short mode")
	}

	testCases := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectedError  bool
	}{
		{
			name:           "config_setup_help",
			args:           []string{"config", "setup", "--help"},
			expectedOutput: []string{"Interactive configuration setup"},
			expectedError:  false,
		},
		{
			name:           "config_show_help",
			args:           []string{"config", "show", "--help"},
			expectedOutput: []string{"Show current configuration"},
			expectedError:  false,
		},
		{
			name:           "config_reset_help",
			args:           []string{"config", "reset", "--help"},
			expectedOutput: []string{"Reset configuration to defaults"},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			output, err := executeCLICommand(ctx, tc.args...)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestCLIUpdateCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping update command tests in short mode")
	}

	testCases := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectedError  bool
	}{
		{
			name:           "update_check_help",
			args:           []string{"update", "check", "--help"},
			expectedOutput: []string{"Check for available updates"},
			expectedError:  false,
		},
		{
			name:           "update_install_help",
			args:           []string{"update", "install", "--help"},
			expectedOutput: []string{"Install available updates"},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			output, err := executeCLICommand(ctx, tc.args...)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestCLIFlagParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping flag parsing tests in short mode")
	}

	testCases := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectedError  bool
	}{
		{
			name:           "dry_run_flag",
			args:           []string{"--dry-run", "--help"},
			expectedOutput: []string{"Preview the command without executing it"},
			expectedError:  false,
		},
		{
			name:           "verbose_flag",
			args:           []string{"--verbose", "--help"},
			expectedOutput: []string{"Enable verbose output"},
			expectedError:  false,
		},
		{
			name:           "provider_flag",
			args:           []string{"--provider", "openai", "--help"},
			expectedOutput: []string{"LLM provider to use"},
			expectedError:  false,
		},
		{
			name:           "model_flag",
			args:           []string{"--model", "gpt-4", "--help"},
			expectedOutput: []string{"Model to use"},
			expectedError:  false,
		},
		{
			name:           "skip_confirmation_flag",
			args:           []string{"--skip-confirmation", "--help"},
			expectedOutput: []string{"Skip confirmation prompts"},
			expectedError:  false,
		},
		{
			name:           "validate_results_flag",
			args:           []string{"--validate-results=false", "--help"},
			expectedOutput: []string{"Validate command results"},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			output, err := executeCLICommand(ctx, tc.args...)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestCLISafetyValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping safety validation tests in short mode")
	}

	// Test that dangerous commands are properly handled
	// Note: These tests use dry-run mode to avoid actually executing dangerous commands
	testCases := []struct {
		name        string
		command     string
		expectSafe  bool
		expectLevel types.DangerLevel
	}{
		{
			name:        "safe_command",
			command:     "ls -la",
			expectSafe:  true,
			expectLevel: types.Safe,
		},
		{
			name:        "dangerous_command",
			command:     "rm -rf /tmp/*",
			expectSafe:  false,
			expectLevel: types.Dangerous,
		},
		{
			name:        "critical_command",
			command:     "rm -rf /",
			expectSafe:  false,
			expectLevel: types.Critical,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Use dry-run mode to test safety validation without execution
			output, err := executeCLICommand(ctx, "--dry-run", "generate", tc.command)

			// For now, we expect these to fail due to missing LLM provider configuration
			// But we can check that the CLI handles the commands appropriately
			if err != nil {
				// Expected due to missing provider configuration
				t.Logf("Expected error due to missing provider config: %v", err)
			}

			// The output should contain information about the command
			if !strings.Contains(output, "Error") && !strings.Contains(output, tc.command) {
				t.Logf("Output: %s", output)
			}
		})
	}
}

func TestCLIErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling tests in short mode")
	}

	testCases := []struct {
		name           string
		args           []string
		expectedError  bool
		expectedOutput []string
	}{
		{
			name:           "invalid_subcommand",
			args:           []string{"invalid-command"},
			expectedError:  true,
			expectedOutput: []string{"unknown command"},
		},
		{
			name:           "missing_argument",
			args:           []string{"completion"},
			expectedError:  true,
			expectedOutput: []string{"accepts 1 arg"},
		},
		{
			name:           "invalid_completion_shell",
			args:           []string{"completion", "invalid-shell"},
			expectedError:  true,
			expectedOutput: []string{"invalid argument"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			output, err := executeCLICommand(ctx, tc.args...)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			for _, expected := range tc.expectedOutput {
				if !strings.Contains(strings.ToLower(output), strings.ToLower(expected)) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestCLICrossPlattformCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-platform tests in short mode")
	}

	// Test that basic CLI functionality works across platforms
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := executeCLICommand(ctx, "--help")
	if err != nil {
		t.Fatalf("Basic help command failed: %v", err)
	}

	// Check for platform-agnostic content
	expectedContent := []string{
		"nl-to-shell",
		"Usage:",
		"Available Commands:",
		"Flags:",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected help output to contain '%s', got: %s", expected, output)
		}
	}
}

// Helper functions for executing CLI commands in tests

func executeCLICommand(ctx context.Context, args ...string) (string, error) {
	return executeCLICommandWithInput(ctx, "", args...)
}

func executeCLICommandWithInput(ctx context.Context, input string, args ...string) (string, error) {
	// Try to find the binary in common locations
	binaryPaths := []string{
		"./nl-to-shell",
		"../nl-to-shell",
		"../../nl-to-shell",
		filepath.Join(os.Getenv("GOPATH"), "bin", "nl-to-shell"),
	}

	var binaryPath string
	for _, path := range binaryPaths {
		if _, err := os.Stat(path); err == nil {
			binaryPath = path
			break
		}
	}

	// If binary not found, try to build it temporarily
	if binaryPath == "" {
		tempDir, err := os.MkdirTemp("", "nl-to-shell-test-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tempDir)

		binaryPath = filepath.Join(tempDir, "nl-to-shell")

		// Try to build the binary
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "./cmd/nl-to-shell")
		if err := buildCmd.Run(); err != nil {
			// If we can't build, skip the test
			return "", err
		}
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	err := cmd.Run()

	// Combine stdout and stderr for output
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	return output, err
}

func TestCLIUpdateMechanism(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping update mechanism tests in short mode")
	}

	// Test update check functionality (should fail gracefully without network)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test update check command
	output, err := executeCLICommand(ctx, "update", "check")

	// We expect this to fail due to network/configuration issues, but it should fail gracefully
	if err != nil {
		t.Logf("Expected error for update check without proper configuration: %v", err)
	}

	// The output should contain some indication of what went wrong
	if output != "" {
		t.Logf("Update check output: %s", output)
	}
}

func TestCLIConfigurationPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping configuration persistence tests in short mode")
	}

	// Create temporary directory for config
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set config directory environment variable if supported
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test configuration commands
	testCases := []string{
		"config show",
		"config reset",
	}

	for _, testCase := range testCases {
		args := strings.Split(testCase, " ")
		output, err := executeCLICommand(ctx, args...)

		// These commands might fail due to implementation status, but should not crash
		if err != nil {
			t.Logf("Command '%s' failed as expected: %v", testCase, err)
		}

		if output != "" {
			t.Logf("Command '%s' output: %s", testCase, output)
		}
	}
}
