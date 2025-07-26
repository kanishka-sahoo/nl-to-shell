package context

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

const (
	// MaxFileListSize limits the number of files to scan for performance
	DefaultMaxFileListSize = 1000
	// MaxDirectoryDepth limits how deep we scan directories
	DefaultMaxDirectoryDepth = 3
)

// Gatherer implements the ContextGatherer interface
type Gatherer struct {
	pluginManager   interfaces.PluginManager
	maxFileListSize int
	maxDepth        int
}

// NewGatherer creates a new context gatherer
func NewGatherer() interfaces.ContextGatherer {
	return &Gatherer{
		pluginManager:   NewPluginManager(),
		maxFileListSize: DefaultMaxFileListSize,
		maxDepth:        DefaultMaxDirectoryDepth,
	}
}

// NewGathererWithLimits creates a new context gatherer with custom limits
func NewGathererWithLimits(maxFiles, maxDepth int) interfaces.ContextGatherer {
	return &Gatherer{
		pluginManager:   NewPluginManager(),
		maxFileListSize: maxFiles,
		maxDepth:        maxDepth,
	}
}

// GatherContext collects environmental context information
func (g *Gatherer) GatherContext(ctx context.Context) (*types.Context, error) {
	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to get working directory",
			Cause:   err,
		}
	}

	// Create base context
	contextData := &types.Context{
		WorkingDirectory: workingDir,
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	// Check for cancellation before file scanning
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Gather file system information
	files, err := g.scanFileSystem(ctx, workingDir)
	if err != nil {
		// Check if it's a cancellation error
		if err == context.Canceled || err == context.DeadlineExceeded {
			return nil, err
		}
		// Don't fail completely if file scanning fails, just log and continue
		contextData.Files = []types.FileInfo{}
	} else {
		contextData.Files = files
	}

	// Gather environment variables (filtered for relevant ones)
	contextData.Environment = g.gatherEnvironment()

	// Run plugins to gather additional context
	pluginData := g.pluginManager.ExecutePlugins(ctx, contextData)
	for pluginName, data := range pluginData {
		contextData.PluginData[pluginName] = data
	}

	return contextData, nil
}

// RegisterPlugin registers a context plugin
func (g *Gatherer) RegisterPlugin(plugin interfaces.ContextPlugin) error {
	return g.pluginManager.RegisterPlugin(plugin)
}

// GetPluginManager returns the plugin manager for advanced plugin operations
func (g *Gatherer) GetPluginManager() interfaces.PluginManager {
	return g.pluginManager
}

// LoadPlugins loads plugins from a directory
func (g *Gatherer) LoadPlugins(pluginDir string) error {
	return g.pluginManager.LoadPlugins(pluginDir)
}

// scanFileSystem scans the file system starting from the given directory
func (g *Gatherer) scanFileSystem(ctx context.Context, rootDir string) ([]types.FileInfo, error) {
	var files []types.FileInfo
	fileCount := 0

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip files/directories we can't access
			return nil
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if we've hit our file limit
		if fileCount >= g.maxFileListSize {
			return filepath.SkipAll
		}

		// Check depth limit
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		depth := len(filepath.SplitList(relPath))
		if depth > g.maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files and directories (starting with .)
		if filepath.Base(path) != "." && filepath.Base(path)[0] == '.' {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		fileInfo := types.FileInfo{
			Name:    d.Name(),
			Path:    path,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		files = append(files, fileInfo)
		fileCount++

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to scan file system",
			Cause:   err,
		}
	}

	return files, nil
}

// gatherEnvironment gathers relevant environment variables
func (g *Gatherer) gatherEnvironment() map[string]string {
	env := make(map[string]string)

	// List of environment variables that are commonly useful for shell commands
	relevantVars := []string{
		"PATH",
		"HOME",
		"USER",
		"SHELL",
		"PWD",
		"OLDPWD",
		"TERM",
		"LANG",
		"LC_ALL",
		"EDITOR",
		"PAGER",
		"TMPDIR",
		"TMP",
		"TEMP",
		// Development-related
		"GOPATH",
		"GOROOT",
		"NODE_ENV",
		"PYTHON_PATH",
		"JAVA_HOME",
		"MAVEN_HOME",
		"GRADLE_HOME",
		// Container-related
		"DOCKER_HOST",
		"KUBERNETES_SERVICE_HOST",
		// Cloud-related
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"GOOGLE_CLOUD_PROJECT",
		"AZURE_SUBSCRIPTION_ID",
	}

	for _, varName := range relevantVars {
		if value := os.Getenv(varName); value != "" {
			env[varName] = value
		}
	}

	return env
}
