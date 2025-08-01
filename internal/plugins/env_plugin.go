package plugins

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// EnvPlugin collects relevant environment variables while filtering sensitive information
type EnvPlugin struct{}

// NewEnvPlugin creates a new environment variable plugin
func NewEnvPlugin() interfaces.ContextPlugin {
	return &EnvPlugin{}
}

// Name returns the plugin name
func (p *EnvPlugin) Name() string {
	return "environment"
}

// Priority returns the plugin priority (higher values execute first)
func (p *EnvPlugin) Priority() int {
	return 100 // High priority as environment variables are fundamental context
}

// GatherContext collects relevant environment variables with sensitive information filtering
func (p *EnvPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	envVars := make(map[string]string)

	// Get all environment variables
	allEnvVars := os.Environ()

	for _, envVar := range allEnvVars {
		// Split key=value
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Filter out sensitive variables
		if p.isSensitive(key) {
			continue
		}

		// Only include relevant variables
		if p.isRelevant(key) {
			envVars[key] = value
		}
	}

	// Add shell information
	shellInfo := p.getShellInfo()

	result := map[string]interface{}{
		"variables": envVars,
		"shell":     shellInfo,
		"user":      p.getUserInfo(),
		"system":    p.getSystemInfo(),
	}

	return result, nil
}

// isSensitive checks if an environment variable contains sensitive information
func (p *EnvPlugin) isSensitive(key string) bool {
	key = strings.ToUpper(key)

	// Patterns for sensitive variables
	sensitivePatterns := []string{
		".*PASSWORD.*",
		".*SECRET.*",
		".*KEY.*",
		".*TOKEN.*",
		".*CREDENTIAL.*",
		".*AUTH.*",
		".*API_KEY.*",
		".*PRIVATE.*",
		".*CERT.*",
		".*SSH.*",
		".*GPG.*",
		".*OAUTH.*",
		".*JWT.*",
		".*BEARER.*",
		".*COOKIE.*",
		".*SESSION.*",
	}

	for _, pattern := range sensitivePatterns {
		matched, _ := regexp.MatchString(pattern, key)
		if matched {
			return true
		}
	}

	return false
}

// isRelevant checks if an environment variable is relevant for command generation
func (p *EnvPlugin) isRelevant(key string) bool {
	key = strings.ToUpper(key)

	// Always include these important variables
	importantVars := []string{
		"PATH",
		"HOME",
		"USER",
		"USERNAME",
		"SHELL",
		"TERM",
		"LANG",
		"LC_ALL",
		"PWD",
		"OLDPWD",
		"EDITOR",
		"VISUAL",
		"PAGER",
		"BROWSER",
		"TMPDIR",
		"TMP",
		"TEMP",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_CACHE_HOME",
	}

	for _, importantVar := range importantVars {
		if key == importantVar {
			return true
		}
	}

	// Include development-related variables
	devPatterns := []string{
		".*_HOME$",    // JAVA_HOME, PYTHON_HOME, etc.
		".*_PATH$",    // PYTHON_PATH, etc.
		".*_VERSION$", // NODE_VERSION, etc.
		".*_ENV$",     // NODE_ENV, etc.
		"GO.*",        // GOPATH, GOROOT, etc.
		"PYTHON.*",    // PYTHONPATH, etc.
		"NODE.*",      // NODE_ENV, etc.
		"JAVA.*",      // JAVA_HOME, etc.
		"DOCKER.*",    // DOCKER_HOST, etc.
		"KUBE.*",      // KUBECONFIG, etc.
		"AWS.*",       // AWS_REGION, etc. (non-sensitive ones)
		"GCP.*",       // GCP_PROJECT, etc.
		"AZURE.*",     // AZURE_SUBSCRIPTION, etc.
		"CI.*",        // CI environment variables
		"BUILD.*",     // Build-related variables
		"DEPLOY.*",    // Deployment-related variables
	}

	for _, pattern := range devPatterns {
		matched, _ := regexp.MatchString(pattern, key)
		if matched && !p.isSensitive(key) {
			return true
		}
	}

	return false
}

// getShellInfo gathers shell-specific information
func (p *EnvPlugin) getShellInfo() map[string]interface{} {
	shellInfo := map[string]interface{}{}

	if shell := os.Getenv("SHELL"); shell != "" {
		shellInfo["current"] = shell
		shellInfo["name"] = p.extractShellName(shell)
	}

	if term := os.Getenv("TERM"); term != "" {
		shellInfo["terminal"] = term
	}

	return shellInfo
}

// extractShellName extracts the shell name from the full path
func (p *EnvPlugin) extractShellName(shellPath string) string {
	parts := strings.Split(shellPath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return shellPath
}

// getUserInfo gathers user-related information
func (p *EnvPlugin) getUserInfo() map[string]interface{} {
	userInfo := map[string]interface{}{}

	if user := os.Getenv("USER"); user != "" {
		userInfo["name"] = user
	} else if user := os.Getenv("USERNAME"); user != "" {
		userInfo["name"] = user
	}

	if home := os.Getenv("HOME"); home != "" {
		userInfo["home"] = home
	}

	return userInfo
}

// getSystemInfo gathers system-related information
func (p *EnvPlugin) getSystemInfo() map[string]interface{} {
	systemInfo := map[string]interface{}{}

	if lang := os.Getenv("LANG"); lang != "" {
		systemInfo["language"] = lang
	}

	if lcAll := os.Getenv("LC_ALL"); lcAll != "" {
		systemInfo["locale"] = lcAll
	}

	// Add temporary directory information
	tempDirs := []string{}
	if tmpdir := os.Getenv("TMPDIR"); tmpdir != "" {
		tempDirs = append(tempDirs, tmpdir)
	}
	if tmp := os.Getenv("TMP"); tmp != "" {
		tempDirs = append(tempDirs, tmp)
	}
	if temp := os.Getenv("TEMP"); temp != "" {
		tempDirs = append(tempDirs, temp)
	}
	if len(tempDirs) > 0 {
		systemInfo["temp_dirs"] = tempDirs
	}

	// Add XDG directories
	xdgDirs := map[string]string{}
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		xdgDirs["config"] = xdgConfig
	}
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		xdgDirs["data"] = xdgData
	}
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		xdgDirs["cache"] = xdgCache
	}
	if len(xdgDirs) > 0 {
		systemInfo["xdg_dirs"] = xdgDirs
	}

	return systemInfo
}
