package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "no arguments shows help",
			args:           []string{},
			expectedOutput: "nl-to-shell is a CLI utility",
			expectError:    false,
		},
		{
			name:           "help flag shows help",
			args:           []string{"--help"},
			expectedOutput: "nl-to-shell is a CLI utility",
			expectError:    false,
		},
		{
			name:           "version command shows version",
			args:           []string{"version"},
			expectedOutput: "nl-to-shell version",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new root command for each test to avoid state pollution
			cmd := createTestRootCmd()
			cmd.SetArgs(tt.args)

			// Capture output
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

			output := buf.String()
			if !strings.Contains(output, tt.expectedOutput) {
				t.Errorf("expected output to contain %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestGlobalFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		checkFn func(t *testing.T)
	}{
		{
			name: "dry-run flag sets global variable",
			args: []string{"--dry-run", "version"},
			checkFn: func(t *testing.T) {
				if !dryRun {
					t.Error("expected dryRun to be true")
				}
			},
		},
		{
			name: "verbose flag sets global variable",
			args: []string{"--verbose", "version"},
			checkFn: func(t *testing.T) {
				if !verbose {
					t.Error("expected verbose to be true")
				}
			},
		},
		{
			name: "provider flag sets global variable",
			args: []string{"--provider", "openai", "version"},
			checkFn: func(t *testing.T) {
				if provider != "openai" {
					t.Errorf("expected provider to be 'openai', got %q", provider)
				}
			},
		},
		{
			name: "model flag sets global variable",
			args: []string{"--model", "gpt-4", "version"},
			checkFn: func(t *testing.T) {
				if model != "gpt-4" {
					t.Errorf("expected model to be 'gpt-4', got %q", model)
				}
			},
		},
		{
			name: "skip-confirmation flag sets global variable",
			args: []string{"--skip-confirmation", "version"},
			checkFn: func(t *testing.T) {
				if !skipConfirmation {
					t.Error("expected skipConfirmation to be true")
				}
			},
		},
		{
			name: "validate-results flag sets global variable",
			args: []string{"--validate-results=false", "version"},
			checkFn: func(t *testing.T) {
				if validateResults {
					t.Error("expected validateResults to be false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
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

			tt.checkFn(t)
		})
	}
}

func TestSubcommands(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "generate command exists",
			args:        []string{"generate", "--help"},
			expectError: false,
		},
		{
			name:        "config command exists",
			args:        []string{"config", "--help"},
			expectError: false,
		},
		{
			name:        "config setup subcommand exists",
			args:        []string{"config", "setup", "--help"},
			expectError: false,
		},
		{
			name:        "config show subcommand exists",
			args:        []string{"config", "show", "--help"},
			expectError: false,
		},
		{
			name:        "config reset subcommand exists",
			args:        []string{"config", "reset", "--help"},
			expectError: false,
		},
		{
			name:        "session command exists",
			args:        []string{"session", "--help"},
			expectError: false,
		},
		{
			name:        "update command exists",
			args:        []string{"update", "--help"},
			expectError: false,
		},
		{
			name:        "update check subcommand exists",
			args:        []string{"update", "check", "--help"},
			expectError: false,
		},
		{
			name:        "update install subcommand exists",
			args:        []string{"update", "install", "--help"},
			expectError: false,
		},
		{
			name:        "version command exists",
			args:        []string{"version", "--help"},
			expectError: false,
		},
		{
			name:        "completion command exists",
			args:        []string{"completion", "--help"},
			expectError: false,
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

func TestGetGlobalFlags(t *testing.T) {
	// Reset and set some flags
	resetGlobalFlags()
	dryRun = true
	verbose = true
	provider = "openai"
	model = "gpt-4"
	skipConfirmation = true
	validateResults = false

	flags := GetGlobalFlags()

	if flags.DryRun != true {
		t.Error("expected DryRun to be true")
	}
	if flags.Verbose != true {
		t.Error("expected Verbose to be true")
	}
	if flags.Provider != "openai" {
		t.Errorf("expected Provider to be 'openai', got %q", flags.Provider)
	}
	if flags.Model != "gpt-4" {
		t.Errorf("expected Model to be 'gpt-4', got %q", flags.Model)
	}
	if flags.SkipConfirmation != true {
		t.Error("expected SkipConfirmation to be true")
	}
	if flags.ValidateResults != false {
		t.Error("expected ValidateResults to be false")
	}
}

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		verbose        bool
	}{
		{
			name:           "version command shows basic version",
			args:           []string{"version"},
			expectedOutput: "nl-to-shell version",
			verbose:        false,
		},
		{
			name:           "version command with verbose shows build info",
			args:           []string{"--verbose", "version"},
			expectedOutput: "Git commit:",
			verbose:        true,
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

			output := buf.String()
			if !strings.Contains(output, tt.expectedOutput) {
				t.Errorf("expected output to contain %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

// Helper functions for testing

func createTestRootCmd() *cobra.Command {
	// Create a fresh root command for testing
	testRootCmd := &cobra.Command{
		Use:   "nl-to-shell",
		Short: "Convert natural language to shell commands",
		Long: `nl-to-shell is a CLI utility that converts natural language descriptions 
into executable shell commands using Large Language Models (LLMs).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
		SilenceUsage: true,
	}

	// Add flags
	testRootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview the command without executing it")
	testRootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output for detailed information")
	testRootCmd.PersistentFlags().StringVar(&provider, "provider", "", "LLM provider to use")
	testRootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to use for the specified provider")
	testRootCmd.PersistentFlags().BoolVar(&skipConfirmation, "skip-confirmation", false, "Skip confirmation prompts")
	testRootCmd.PersistentFlags().BoolVar(&validateResults, "validate-results", true, "Validate command results using AI")

	// Create test version command that captures output properly
	testVersionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display the current version of nl-to-shell along with build information.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("nl-to-shell version %s\n", Version)
			if verbose {
				cmd.Printf("Git commit: %s\n", GitCommit)
				cmd.Printf("Build date: %s\n", BuildDate)
			}
		},
	}

	// Add all subcommands
	testRootCmd.AddCommand(generateCmd)
	testRootCmd.AddCommand(configCmd)
	testRootCmd.AddCommand(sessionCmd)
	testRootCmd.AddCommand(updateCmd)
	testRootCmd.AddCommand(testVersionCmd)
	testRootCmd.AddCommand(completionCmd)

	return testRootCmd
}

func resetGlobalFlags() {
	dryRun = false
	verbose = false
	provider = ""
	model = ""
	skipConfirmation = false
	validateResults = true
	sessionMode = false
}
