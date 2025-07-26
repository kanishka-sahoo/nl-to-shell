package context

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// GitPlugin implements git repository context gathering
type GitPlugin struct{}

// NewGitPlugin creates a new git context plugin
func NewGitPlugin() interfaces.ContextPlugin {
	return &GitPlugin{}
}

// Name returns the plugin name
func (g *GitPlugin) Name() string {
	return "git"
}

// Priority returns the plugin priority (higher values run first)
func (g *GitPlugin) Priority() int {
	return 100 // High priority for git context
}

// GatherContext gathers git repository information
func (g *GitPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	gitInfo, err := g.gatherGitInfo(ctx, baseContext.WorkingDirectory)
	if err != nil {
		// If we can't gather git info, return empty data rather than failing
		return map[string]interface{}{}, nil
	}

	// Update the base context with git info
	baseContext.GitInfo = gitInfo

	// Return git info as plugin data as well
	return map[string]interface{}{
		"is_repository":           gitInfo.IsRepository,
		"current_branch":          gitInfo.CurrentBranch,
		"working_tree_status":     gitInfo.WorkingTreeStatus,
		"has_uncommitted_changes": gitInfo.HasUncommittedChanges,
	}, nil
}

// gatherGitInfo collects git repository information
func (g *GitPlugin) gatherGitInfo(ctx context.Context, workingDir string) (*types.GitContext, error) {
	gitInfo := &types.GitContext{
		IsRepository:          false,
		CurrentBranch:         "",
		WorkingTreeStatus:     "",
		HasUncommittedChanges: false,
	}

	// Check if we're in a git repository
	if !g.isGitRepository(workingDir) {
		return gitInfo, nil
	}

	gitInfo.IsRepository = true

	// Get current branch
	branch, err := g.getCurrentBranch(ctx, workingDir)
	if err == nil {
		gitInfo.CurrentBranch = branch
	}

	// Get working tree status
	status, hasChanges, err := g.getWorkingTreeStatus(ctx, workingDir)
	if err == nil {
		gitInfo.WorkingTreeStatus = status
		gitInfo.HasUncommittedChanges = hasChanges
	}

	return gitInfo, nil
}

// isGitRepository checks if the directory is part of a git repository
func (g *GitPlugin) isGitRepository(dir string) bool {
	// Walk up the directory tree looking for .git directory
	currentDir := dir
	for {
		gitDir := filepath.Join(currentDir, ".git")
		if info, err := os.Stat(gitDir); err == nil {
			// .git exists, check if it's a directory or file (for git worktrees)
			return info.IsDir() || info.Mode().IsRegular()
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root directory
			break
		}
		currentDir = parent
	}
	return false
}

// getCurrentBranch gets the current git branch
func (g *GitPlugin) getCurrentBranch(ctx context.Context, workingDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	return branch, nil
}

// getWorkingTreeStatus gets the working tree status
func (g *GitPlugin) getWorkingTreeStatus(ctx context.Context, workingDir string) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		return "", false, err
	}

	status := strings.TrimSpace(string(output))
	hasChanges := len(status) > 0

	// If there are changes, get a more readable status
	if hasChanges {
		readableCmd := exec.CommandContext(ctx, "git", "status", "--short")
		readableCmd.Dir = workingDir

		if readableOutput, err := readableCmd.Output(); err == nil {
			status = strings.TrimSpace(string(readableOutput))
		}
	} else {
		status = "clean"
	}

	return status, hasChanges, nil
}

// GitContextGatherer is a standalone git context gatherer that can be used independently
type GitContextGatherer struct {
	plugin *GitPlugin
}

// NewGitContextGatherer creates a new standalone git context gatherer
func NewGitContextGatherer() *GitContextGatherer {
	return &GitContextGatherer{
		plugin: &GitPlugin{},
	}
}

// GatherGitContext gathers git information for a specific directory
func (g *GitContextGatherer) GatherGitContext(ctx context.Context, workingDir string) (*types.GitContext, error) {
	return g.plugin.gatherGitInfo(ctx, workingDir)
}

// IsGitRepository checks if a directory is part of a git repository
func (g *GitContextGatherer) IsGitRepository(dir string) bool {
	return g.plugin.isGitRepository(dir)
}
