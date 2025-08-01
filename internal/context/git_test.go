package context

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestNewGitPlugin(t *testing.T) {
	plugin := NewGitPlugin()
	if plugin == nil {
		t.Fatal("NewGitPlugin() returned nil")
	}

	// Test that it implements the interface
	var _ interfaces.ContextPlugin = plugin
}

func TestGitPluginName(t *testing.T) {
	plugin := NewGitPlugin()
	if plugin.Name() != "git" {
		t.Errorf("Expected plugin name 'git', got '%s'", plugin.Name())
	}
}

func TestGitPluginPriority(t *testing.T) {
	plugin := NewGitPlugin()
	if plugin.Priority() != 100 {
		t.Errorf("Expected plugin priority 100, got %d", plugin.Priority())
	}
}

func TestIsGitRepository(t *testing.T) {
	plugin := &GitPlugin{}

	// Test with non-git directory
	tempDir := t.TempDir()
	if plugin.isGitRepository(tempDir) {
		t.Error("Non-git directory incorrectly identified as git repository")
	}

	// Test with git directory (if git is available)
	if isGitAvailable() {
		gitDir := createTestGitRepo(t)
		if !plugin.isGitRepository(gitDir) {
			t.Error("Git directory not correctly identified as git repository")
		}

		// Test subdirectory of git repo
		subDir := filepath.Join(gitDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		if !plugin.isGitRepository(subDir) {
			t.Error("Subdirectory of git repo not correctly identified")
		}
	}
}

func TestGatherGitInfoNonGitDirectory(t *testing.T) {
	plugin := &GitPlugin{}
	ctx := context.Background()
	tempDir := t.TempDir()

	gitInfo, err := plugin.gatherGitInfo(ctx, tempDir)
	if err != nil {
		t.Errorf("gatherGitInfo failed for non-git directory: %v", err)
	}

	if gitInfo.IsRepository {
		t.Error("Non-git directory incorrectly marked as repository")
	}

	if gitInfo.CurrentBranch != "" {
		t.Error("Non-git directory should have empty branch")
	}

	if gitInfo.WorkingTreeStatus != "" {
		t.Error("Non-git directory should have empty status")
	}

	if gitInfo.HasUncommittedChanges {
		t.Error("Non-git directory should not have uncommitted changes")
	}
}

func TestGatherGitInfoGitDirectory(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("Git not available, skipping git repository tests")
	}

	plugin := &GitPlugin{}
	ctx := context.Background()
	gitDir := createTestGitRepo(t)

	gitInfo, err := plugin.gatherGitInfo(ctx, gitDir)
	if err != nil {
		t.Errorf("gatherGitInfo failed for git directory: %v", err)
	}

	if !gitInfo.IsRepository {
		t.Error("Git directory not correctly marked as repository")
	}

	// Should have a branch (likely "main" or "master")
	if gitInfo.CurrentBranch == "" {
		t.Error("Git repository should have a current branch")
	}

	// Status should be set (either "clean" or some status)
	if gitInfo.WorkingTreeStatus == "" {
		t.Error("Git repository should have working tree status")
	}
}

func TestGatherContextPlugin(t *testing.T) {
	plugin := NewGitPlugin()
	ctx := context.Background()

	baseContext := &types.Context{
		WorkingDirectory: t.TempDir(),
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	data, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	if data == nil {
		t.Fatal("GatherContext returned nil data")
	}

	// Check that expected keys are present
	expectedKeys := []string{
		"is_repository",
		"current_branch",
		"working_tree_status",
		"has_uncommitted_changes",
	}

	for _, key := range expectedKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("Expected key '%s' not found in plugin data", key)
		}
	}

	// Check that base context was updated
	if baseContext.GitInfo == nil {
		t.Error("Base context GitInfo not updated")
	}
}

func TestGatherContextWithGitRepo(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("Git not available, skipping git repository tests")
	}

	plugin := NewGitPlugin()
	ctx := context.Background()
	gitDir := createTestGitRepo(t)

	baseContext := &types.Context{
		WorkingDirectory: gitDir,
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	data, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	// Check that repository is correctly identified
	if isRepo, ok := data["is_repository"].(bool); !ok || !isRepo {
		t.Error("Git repository not correctly identified in plugin data")
	}

	// Check that branch is set
	if branch, ok := data["current_branch"].(string); !ok || branch == "" {
		t.Error("Current branch not set in plugin data")
	}

	// Check that base context GitInfo is properly set
	if baseContext.GitInfo == nil {
		t.Fatal("Base context GitInfo not set")
	}

	if !baseContext.GitInfo.IsRepository {
		t.Error("Base context GitInfo not correctly set")
	}
}

func TestNewGitContextGatherer(t *testing.T) {
	gatherer := NewGitContextGatherer()
	if gatherer == nil {
		t.Fatal("NewGitContextGatherer() returned nil")
	}

	if gatherer.plugin == nil {
		t.Error("GitContextGatherer plugin not initialized")
	}
}

func TestGitContextGathererMethods(t *testing.T) {
	gatherer := NewGitContextGatherer()
	tempDir := t.TempDir()

	// Test IsGitRepository
	if gatherer.IsGitRepository(tempDir) {
		t.Error("Non-git directory incorrectly identified as git repository")
	}

	// Test GatherGitContext
	ctx := context.Background()
	gitInfo, err := gatherer.GatherGitContext(ctx, tempDir)
	if err != nil {
		t.Errorf("GatherGitContext failed: %v", err)
	}

	if gitInfo == nil {
		t.Fatal("GatherGitContext returned nil")
	}

	if gitInfo.IsRepository {
		t.Error("Non-git directory incorrectly marked as repository")
	}
}

func TestGitContextGathererWithGitRepo(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("Git not available, skipping git repository tests")
	}

	gatherer := NewGitContextGatherer()
	gitDir := createTestGitRepo(t)

	// Test IsGitRepository
	if !gatherer.IsGitRepository(gitDir) {
		t.Error("Git directory not correctly identified as git repository")
	}

	// Test GatherGitContext
	ctx := context.Background()
	gitInfo, err := gatherer.GatherGitContext(ctx, gitDir)
	if err != nil {
		t.Errorf("GatherGitContext failed: %v", err)
	}

	if !gitInfo.IsRepository {
		t.Error("Git directory not correctly marked as repository")
	}

	if gitInfo.CurrentBranch == "" {
		t.Error("Current branch not set")
	}
}

func TestGitCommandsWithCancellation(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("Git not available, skipping git command tests")
	}

	plugin := &GitPlugin{}
	gitDir := createTestGitRepo(t)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := plugin.getCurrentBranch(ctx, gitDir)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}

	_, _, err = plugin.getWorkingTreeStatus(ctx, gitDir)
	if err == nil {
		t.Error("Expected error with cancelled context for status")
	}
}

func TestGitIntegrationWithMainGatherer(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("Git not available, skipping integration test")
	}

	// Create a gatherer and register the git plugin
	gatherer := NewGatherer().(*Gatherer)
	gitPlugin := NewGitPlugin()

	err := gatherer.RegisterPlugin(gitPlugin)
	if err != nil {
		t.Errorf("Failed to register git plugin: %v", err)
	}

	// Create a git repository for testing
	gitDir := createTestGitRepo(t)

	// Change to the git directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(gitDir)

	ctx := context.Background()
	context, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	// Check that git info was gathered
	if context.GitInfo == nil {
		t.Error("Git info not gathered by main gatherer")
	}

	if !context.GitInfo.IsRepository {
		t.Error("Git repository not correctly identified by main gatherer")
	}

	// Check that plugin data was included
	if gitData, exists := context.PluginData["git"]; !exists {
		t.Error("Git plugin data not included in context")
	} else {
		gitMap := gitData.(map[string]interface{})
		if isRepo, ok := gitMap["is_repository"].(bool); !ok || !isRepo {
			t.Error("Git plugin data not correctly structured")
		}
	}
}

// Helper functions

func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func createTestGitRepo(t *testing.T) string {
	tempDir := t.TempDir()

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user (required for commits)
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}

	for _, cmdArgs := range configCmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to configure git: %v", err)
		}
	}

	// Create and commit a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add and commit the file
	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = tempDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add file to git: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = tempDir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit to git: %v", err)
	}

	return tempDir
}
