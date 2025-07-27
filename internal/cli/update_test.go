package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestUpdateCheckCommand(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalVerbose := verbose

	// Set test values
	Version = "v1.0.0"
	verbose = false

	// Restore original values after test
	defer func() {
		Version = originalVersion
		verbose = originalVerbose
	}()

	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create a new command for testing
	cmd := &cobra.Command{
		Use:  "check",
		RunE: executeUpdateCheck,
	}
	cmd.Flags().Bool("prerelease", false, "Include prerelease versions")

	// Set the command's output to our buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute the command
	err := cmd.Execute()

	// The command will likely fail due to network issues in test environment,
	// but we can test that it doesn't panic and handles errors gracefully
	if err != nil {
		// Check that the error is related to network/API issues, not code issues
		if !strings.Contains(err.Error(), "failed to check for updates") {
			t.Errorf("Unexpected error type: %v", err)
		}
	}
}

func TestUpdateInstallCommand(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalVerbose := verbose

	// Set test values
	Version = "v1.0.0"
	verbose = false

	// Restore original values after test
	defer func() {
		Version = originalVersion
		verbose = originalVerbose
	}()

	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create a new command for testing
	cmd := &cobra.Command{
		Use:  "install",
		RunE: executeUpdateInstall,
	}
	cmd.Flags().Bool("prerelease", false, "Allow prerelease versions")
	cmd.Flags().Bool("no-backup", false, "Skip backup")

	// Set the command's output to our buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute the command
	err := cmd.Execute()

	// The command will likely fail due to network issues in test environment,
	// but we can test that it doesn't panic and handles errors gracefully
	if err != nil {
		// Check that the error is related to network/API issues, not code issues
		if !strings.Contains(err.Error(), "failed to check for updates") {
			t.Errorf("Unexpected error type: %v", err)
		}
	}
}

func TestUpdateCommandFlags(t *testing.T) {
	// Test that the update commands have the expected flags

	// Test check command flags
	if checkCmd.Flags().Lookup("prerelease") == nil {
		t.Error("checkCmd should have prerelease flag")
	}

	// Test install command flags
	if installCmd.Flags().Lookup("prerelease") == nil {
		t.Error("installCmd should have prerelease flag")
	}

	if installCmd.Flags().Lookup("no-backup") == nil {
		t.Error("installCmd should have no-backup flag")
	}
}

func TestUpdateCommandStructure(t *testing.T) {
	// Test that update commands are properly structured

	if updateCmd.Use != "update" {
		t.Errorf("Expected updateCmd.Use to be 'update', got %s", updateCmd.Use)
	}

	if checkCmd.Use != "check" {
		t.Errorf("Expected checkCmd.Use to be 'check', got %s", checkCmd.Use)
	}

	if installCmd.Use != "install" {
		t.Errorf("Expected installCmd.Use to be 'install', got %s", installCmd.Use)
	}

	// Check that commands have descriptions
	if checkCmd.Short == "" {
		t.Error("checkCmd should have a short description")
	}

	if installCmd.Short == "" {
		t.Error("installCmd should have a short description")
	}

	if checkCmd.Long == "" {
		t.Error("checkCmd should have a long description")
	}

	if installCmd.Long == "" {
		t.Error("installCmd should have a long description")
	}
}

func TestUpdateCommandsAddedToParent(t *testing.T) {
	// Test that update subcommands are properly added to the parent command

	found := false
	for _, cmd := range updateCmd.Commands() {
		if cmd.Use == "check" {
			found = true
			break
		}
	}
	if !found {
		t.Error("check command not found in update subcommands")
	}

	found = false
	for _, cmd := range updateCmd.Commands() {
		if cmd.Use == "install" {
			found = true
			break
		}
	}
	if !found {
		t.Error("install command not found in update subcommands")
	}
}

func TestVersionVariable(t *testing.T) {
	// Test that version variables are properly defined

	if Version == "" {
		t.Error("Version should not be empty")
	}

	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}

	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}

// Integration test that simulates the full update check flow
func TestUpdateCheckIntegration(t *testing.T) {
	// Skip this test if we're in a CI environment or don't have network access
	if os.Getenv("CI") != "" || os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network-dependent test")
	}

	// Save original values
	originalVersion := Version
	originalVerbose := verbose

	// Set test values
	Version = "v0.0.1" // Use a very old version to ensure update is available
	verbose = true

	// Restore original values after test
	defer func() {
		Version = originalVersion
		verbose = originalVerbose
	}()

	// Create a test command to pass to the function
	cmd := &cobra.Command{}
	cmd.Flags().Bool("prerelease", false, "Include prerelease versions")

	// This test will actually try to contact GitHub API
	// It should either succeed or fail gracefully with a network error
	err := executeUpdateCheck(cmd, []string{})

	if err != nil {
		// Network errors are acceptable in test environment
		if !strings.Contains(err.Error(), "failed to check for updates") &&
			!strings.Contains(err.Error(), "network") &&
			!strings.Contains(err.Error(), "timeout") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// Test the configuration integration
func TestUpdateConfigurationIntegration(t *testing.T) {
	// Test that update functions properly handle configuration loading

	// Save original values
	originalVersion := Version
	originalVerbose := verbose

	// Set test values
	Version = "v1.0.0"
	verbose = false

	// Restore original values after test
	defer func() {
		Version = originalVersion
		verbose = originalVerbose
	}()

	// Set prerelease flag
	checkCmd.Flags().Set("prerelease", "true")
	defer checkCmd.Flags().Set("prerelease", "false")

	// Create a test command to pass to the function
	cmd := &cobra.Command{}
	cmd.Flags().Bool("prerelease", true, "Include prerelease versions")

	// This should not panic even if configuration loading fails
	err := executeUpdateCheck(cmd, []string{})

	// We expect this to fail with a network error, but it should handle
	// configuration loading gracefully
	if err != nil && !strings.Contains(err.Error(), "failed to check for updates") {
		t.Errorf("Unexpected error type: %v", err)
	}
}
