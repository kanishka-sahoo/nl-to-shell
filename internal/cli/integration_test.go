package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestCommandGenerationIntegration(t *testing.T) {
	// Skip integration tests if we don't have proper configuration
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	tests := []struct {
		name          string
		args          []string
		expectedError bool
		checkOutput   func(string) bool
	}{
		{
			name: "help command works",
			args: []string{"--help"},
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Convert natural language to shell commands")
			},
		},
		{
			name: "version command works",
			args: []string{"version"},
			checkOutput: func(output string) bool {
				return strings.Contains(output, "nl-to-shell version")
			},
		},
		{
			name: "dry run mode works",
			args: []string{"--dry-run", "list files"},
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Generated command:") &&
					strings.Contains(output, "Dry Run Analysis")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			resetGlobalFlags()

			cmd := createTestRootCmd()
			cmd.SetArgs(tt.args)

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()

			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			output := buf.String()
			if tt.checkOutput != nil && !tt.checkOutput(output) {
				t.Errorf("output check failed for output: %s", output)
			}
		})
	}
}

func TestExecuteCommandGeneration(t *testing.T) {
	// Test that the function handles missing configuration gracefully
	// This will fail because we don't have proper provider configuration
	err := executeCommandGeneration("test command")
	if err == nil {
		t.Errorf("expected error due to missing configuration, but got none")
	}

	// The error should be related to configuration or provider setup
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "provider") && !strings.Contains(errMsg, "config") {
		t.Errorf("expected configuration or provider error, got: %v", err)
	}
}

func TestDisplayResults(t *testing.T) {
	tests := []struct {
		name           string
		result         *types.FullResult
		originalInput  string
		expectedOutput []string
	}{
		{
			name: "basic command result",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls -la",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
					Confidence: 0.9,
				},
			},
			originalInput:  "list files",
			expectedOutput: []string{"Generated command: ls -la"},
		},
		{
			name: "dangerous command with warnings",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "rm -rf /tmp/*",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Dangerous,
						Warnings:    []string{"This command will delete files"},
					},
					Confidence: 0.8,
				},
			},
			originalInput: "delete temp files",
			expectedOutput: []string{
				"Generated command: rm -rf /tmp/*",
				"Safety level: Dangerous",
				"This command will delete files",
			},
		},
		{
			name: "dry run result",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls -la",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				DryRunResult: &types.DryRunResult{
					Analysis:    "This will list files",
					Predictions: []string{"Shows file details", "Includes hidden files"},
				},
			},
			originalInput: "list files",
			expectedOutput: []string{
				"Generated command: ls -la",
				"Dry Run Analysis",
				"This will list files",
				"Shows file details",
			},
		},
		{
			name: "requires confirmation",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "rm file.txt",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Dangerous,
					},
				},
				RequiresConfirmation: true,
			},
			originalInput: "delete file",
			expectedOutput: []string{
				"Generated command: rm file.txt",
				"requires confirmation",
			},
		},
		{
			name: "execution result with validation",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls -la",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				ExecutionResult: &types.ExecutionResult{
					ExitCode: 0,
					Stdout:   "file1.txt\nfile2.txt",
					Duration: time.Millisecond * 100,
				},
				ValidationResult: &types.ValidationResult{
					IsCorrect:   true,
					Explanation: "Command executed successfully",
				},
			},
			originalInput: "list files",
			expectedOutput: []string{
				"Generated command: ls -la",
				"Execution Results",
				"Exit code: 0",
				"file1.txt",
				"Validation Results",
				"executed successfully",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := displayResults(tt.result, tt.originalInput)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Restore stdout and read output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got: %s", expected, output)
				}
			}
		})
	}
}

func TestGlobalFlagsIntegration(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T)
	}{
		{
			name: "verbose flag affects version output",
			args: []string{"--verbose", "version"},
			check: func(t *testing.T) {
				if !verbose {
					t.Error("verbose flag should be set")
				}
			},
		},
		{
			name: "dry-run flag is set",
			args: []string{"--dry-run", "version"},
			check: func(t *testing.T) {
				if !dryRun {
					t.Error("dry-run flag should be set")
				}
			},
		},
		{
			name: "provider flag is set",
			args: []string{"--provider", "openai", "version"},
			check: func(t *testing.T) {
				if provider != "openai" {
					t.Errorf("provider should be 'openai', got %q", provider)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()

			cmd := createTestRootCmd()
			cmd.SetArgs(tt.args)

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			tt.check(t)
		})
	}
}

// Helper function to test command parsing
func TestCommandParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid generate command",
			args:        []string{"generate", "list files"},
			expectError: true, // Expected to fail due to missing API credentials in test environment
		},
		{
			name:        "generate without argument",
			args:        []string{"generate"},
			expectError: true,
		},
		{
			name:        "root command with argument",
			args:        []string{"list files"},
			expectError: false, // Test root command doesn't actually call executeCommandGeneration
		},
		{
			name:        "invalid command",
			args:        []string{"invalid"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCmd()
			cmd.SetArgs(tt.args)

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
