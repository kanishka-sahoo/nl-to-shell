package plugins

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// DevToolsPlugin detects development tools and their versions
type DevToolsPlugin struct{}

// NewDevToolsPlugin creates a new development tools detection plugin
func NewDevToolsPlugin() interfaces.ContextPlugin {
	return &DevToolsPlugin{}
}

// Name returns the plugin name
func (p *DevToolsPlugin) Name() string {
	return "devtools"
}

// Priority returns the plugin priority
func (p *DevToolsPlugin) Priority() int {
	return 90 // High priority as dev tools are important context
}

// ToolInfo represents information about a detected development tool
type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// GatherContext detects development tools and their versions
func (p *DevToolsPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	tools := make(map[string]ToolInfo)

	// Define tools to detect
	toolsToDetect := []struct {
		name           string
		command        string
		versionFlag    string
		versionPattern string
	}{
		{"docker", "docker", "--version", `Docker version ([^\s,]+)`},
		{"node", "node", "--version", `v?(.+)`},
		{"npm", "npm", "--version", `(.+)`},
		{"yarn", "yarn", "--version", `(.+)`},
		{"python", "python", "--version", `Python (.+)`},
		{"python3", "python3", "--version", `Python (.+)`},
		{"pip", "pip", "--version", `pip ([^\s]+)`},
		{"pip3", "pip3", "--version", `pip ([^\s]+)`},
		{"go", "go", "version", `go version go([^\s]+)`},
		{"java", "java", "-version", `version "([^"]+)"`},
		{"javac", "javac", "-version", `javac (.+)`},
		{"mvn", "mvn", "--version", `Apache Maven ([^\s]+)`},
		{"gradle", "gradle", "--version", `Gradle ([^\s]+)`},
		{"ruby", "ruby", "--version", `ruby ([^\s]+)`},
		{"gem", "gem", "--version", `(.+)`},
		{"php", "php", "--version", `PHP ([^\s]+)`},
		{"composer", "composer", "--version", `Composer version ([^\s]+)`},
		{"rust", "rustc", "--version", `rustc ([^\s]+)`},
		{"cargo", "cargo", "--version", `cargo ([^\s]+)`},
		{"git", "git", "--version", `git version (.+)`},
		{"kubectl", "kubectl", "version", `Client Version: version\.Info\{Major:"([^"]+)", Minor:"([^"]+)"`},
		{"helm", "helm", "version", `version\.BuildInfo\{Version:"v?([^"]+)"`},
		{"terraform", "terraform", "--version", `Terraform v(.+)`},
		{"ansible", "ansible", "--version", `ansible \[core ([^\]]+)\]`},
		{"make", "make", "--version", `GNU Make (.+)`},
		{"cmake", "cmake", "--version", `cmake version (.+)`},
		{"gcc", "gcc", "--version", `gcc \([^)]+\) (.+)`},
		{"clang", "clang", "--version", `clang version (.+)`},
		{"vim", "vim", "--version", `VIM - Vi IMproved (.+)`},
		{"emacs", "emacs", "--version", `GNU Emacs (.+)`},
		{"code", "code", "--version", `(.+)`},
		{"curl", "curl", "--version", `curl (.+)`},
		{"wget", "wget", "--version", `GNU Wget (.+)`},
		{"jq", "jq", "--version", `jq-(.+)`},
		{"aws", "aws", "--version", `aws-cli/([^\s]+)`},
		{"gcloud", "gcloud", "version", `Google Cloud SDK (.+)`},
		{"az", "az", "--version", `azure-cli\s+(.+)`},
	}

	// Detect each tool
	for _, tool := range toolsToDetect {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		toolInfo := p.detectTool(tool.name, tool.command, tool.versionFlag, tool.versionPattern)
		tools[tool.name] = toolInfo
	}

	// Detect additional context
	runtimes := p.detectRuntimes(ctx)
	containers := p.detectContainers(ctx)
	databases := p.detectDatabases(ctx)

	result := map[string]interface{}{
		"tools":      tools,
		"runtimes":   runtimes,
		"containers": containers,
		"databases":  databases,
	}

	return result, nil
}

// detectTool detects a specific tool and its version
func (p *DevToolsPlugin) detectTool(name, command, versionFlag, versionPattern string) ToolInfo {
	toolInfo := ToolInfo{
		Name:      name,
		Available: false,
	}

	// Check if command exists in PATH
	path, err := exec.LookPath(command)
	if err != nil {
		return toolInfo
	}

	toolInfo.Path = path
	toolInfo.Available = true

	// Get version information
	cmd := exec.Command(command, versionFlag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return toolInfo
	}

	// Extract version using regex
	if versionPattern != "" {
		re := regexp.MustCompile(versionPattern)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			if name == "kubectl" && len(matches) > 2 {
				// Special case for kubectl which has major.minor
				toolInfo.Version = matches[1] + "." + matches[2]
			} else {
				toolInfo.Version = strings.TrimSpace(matches[1])
			}
		}
	}

	return toolInfo
}

// detectRuntimes detects runtime environments
func (p *DevToolsPlugin) detectRuntimes(_ context.Context) map[string]interface{} {
	runtimes := make(map[string]interface{})

	// Node.js runtime detection
	if nodeInfo := p.detectNodeRuntime(); nodeInfo != nil {
		runtimes["nodejs"] = nodeInfo
	}

	// Python runtime detection
	if pythonInfo := p.detectPythonRuntime(); pythonInfo != nil {
		runtimes["python"] = pythonInfo
	}

	// Java runtime detection
	if javaInfo := p.detectJavaRuntime(); javaInfo != nil {
		runtimes["java"] = javaInfo
	}

	// Go runtime detection
	if goInfo := p.detectGoRuntime(); goInfo != nil {
		runtimes["go"] = goInfo
	}

	return runtimes
}

// detectNodeRuntime detects Node.js runtime information
func (p *DevToolsPlugin) detectNodeRuntime() map[string]interface{} {
	nodeInfo := make(map[string]interface{})
	hasNodeProject := false

	// Check for package.json
	if _, err := os.Stat("package.json"); err == nil {
		nodeInfo["has_package_json"] = true
		hasNodeProject = true

		// Check for node_modules
		if _, err := os.Stat("node_modules"); err == nil {
			nodeInfo["has_node_modules"] = true
		}

		// Check for common Node.js files
		nodeFiles := []string{"yarn.lock", "package-lock.json", ".nvmrc", ".node-version"}
		foundFiles := []string{}
		for _, file := range nodeFiles {
			if _, err := os.Stat(file); err == nil {
				foundFiles = append(foundFiles, file)
			}
		}
		if len(foundFiles) > 0 {
			nodeInfo["config_files"] = foundFiles
		}
	}

	// Check for other Node.js indicators
	nodeFiles := []string{".nvmrc", ".node-version"}
	for _, file := range nodeFiles {
		if _, err := os.Stat(file); err == nil {
			hasNodeProject = true
			break
		}
	}

	// Only add NVM info if we have a Node.js project
	if hasNodeProject {
		if nvmDir := os.Getenv("NVM_DIR"); nvmDir != "" {
			nodeInfo["nvm_dir"] = nvmDir
		}
	}

	if hasNodeProject && len(nodeInfo) > 0 {
		return nodeInfo
	}
	return nil
}

// detectPythonRuntime detects Python runtime information
func (p *DevToolsPlugin) detectPythonRuntime() map[string]interface{} {
	pythonInfo := make(map[string]interface{})

	// Check for Python project files
	pythonFiles := []string{
		"requirements.txt", "setup.py", "pyproject.toml", "Pipfile",
		"environment.yml", "conda.yml", ".python-version", "runtime.txt",
	}
	foundFiles := []string{}
	for _, file := range pythonFiles {
		if _, err := os.Stat(file); err == nil {
			foundFiles = append(foundFiles, file)
		}
	}
	if len(foundFiles) > 0 {
		pythonInfo["config_files"] = foundFiles
	}

	// Check for virtual environments
	venvDirs := []string{"venv", ".venv", "env", ".env", "virtualenv"}
	for _, dir := range venvDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			pythonInfo["virtual_env"] = dir
			break
		}
	}

	// Check for conda
	if condaEnv := os.Getenv("CONDA_DEFAULT_ENV"); condaEnv != "" {
		pythonInfo["conda_env"] = condaEnv
	}

	if len(pythonInfo) > 0 {
		return pythonInfo
	}
	return nil
}

// detectJavaRuntime detects Java runtime information
func (p *DevToolsPlugin) detectJavaRuntime() map[string]interface{} {
	javaInfo := make(map[string]interface{})
	hasJavaProject := false

	// Check for Java project files
	javaFiles := []string{"pom.xml", "build.gradle", "build.gradle.kts", "build.xml", ".java-version"}
	foundFiles := []string{}
	for _, file := range javaFiles {
		if _, err := os.Stat(file); err == nil {
			foundFiles = append(foundFiles, file)
			hasJavaProject = true
		}
	}
	if len(foundFiles) > 0 {
		javaInfo["config_files"] = foundFiles
	}

	// Check for common Java directories
	javaDirs := []string{"src/main/java", "src/test/java", "target", "build"}
	foundDirs := []string{}
	for _, dir := range javaDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			foundDirs = append(foundDirs, dir)
			hasJavaProject = true
		}
	}
	if len(foundDirs) > 0 {
		javaInfo["project_dirs"] = foundDirs
	}

	// Only add JAVA_HOME if we have a Java project
	if hasJavaProject {
		if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
			javaInfo["java_home"] = javaHome
		}
	}

	if hasJavaProject && len(javaInfo) > 0 {
		return javaInfo
	}
	return nil
}

// detectGoRuntime detects Go runtime information
func (p *DevToolsPlugin) detectGoRuntime() map[string]interface{} {
	goInfo := make(map[string]interface{})
	hasGoProject := false

	// Check for Go project files
	goFiles := []string{"go.mod", "go.sum", "Gopkg.toml", "Gopkg.lock"}
	foundFiles := []string{}
	for _, file := range goFiles {
		if _, err := os.Stat(file); err == nil {
			foundFiles = append(foundFiles, file)
			hasGoProject = true
		}
	}
	if len(foundFiles) > 0 {
		goInfo["config_files"] = foundFiles
	}

	// Check for Go source files
	matches, _ := filepath.Glob("*.go")
	if len(matches) > 0 {
		goInfo["has_go_files"] = true
		hasGoProject = true
	}

	// Only add environment variables if we have a Go project
	if hasGoProject {
		if goPath := os.Getenv("GOPATH"); goPath != "" {
			goInfo["gopath"] = goPath
		}
		if goRoot := os.Getenv("GOROOT"); goRoot != "" {
			goInfo["goroot"] = goRoot
		}
	}

	if hasGoProject && len(goInfo) > 0 {
		return goInfo
	}
	return nil
}

// detectContainers detects container-related tools and environments
func (p *DevToolsPlugin) detectContainers(_ context.Context) map[string]interface{} {
	containers := make(map[string]interface{})

	// Check for Docker
	dockerInfo := make(map[string]interface{})
	if _, err := os.Stat("Dockerfile"); err == nil {
		dockerInfo["has_dockerfile"] = true
	}
	if _, err := os.Stat("docker-compose.yml"); err == nil {
		dockerInfo["has_compose"] = true
	}
	if _, err := os.Stat("docker-compose.yaml"); err == nil {
		dockerInfo["has_compose"] = true
	}
	if _, err := os.Stat(".dockerignore"); err == nil {
		dockerInfo["has_dockerignore"] = true
	}
	if len(dockerInfo) > 0 {
		containers["docker"] = dockerInfo
	}

	// Check for Kubernetes
	k8sInfo := make(map[string]interface{})
	k8sFiles := []string{"deployment.yaml", "deployment.yml", "service.yaml", "service.yml", "kustomization.yaml", "kustomization.yml"}
	foundK8sFiles := []string{}
	for _, file := range k8sFiles {
		if _, err := os.Stat(file); err == nil {
			foundK8sFiles = append(foundK8sFiles, file)
		}
	}
	if len(foundK8sFiles) > 0 {
		k8sInfo["config_files"] = foundK8sFiles
	}
	if kubeConfig := os.Getenv("KUBECONFIG"); kubeConfig != "" {
		k8sInfo["kubeconfig"] = kubeConfig
	}
	if len(k8sInfo) > 0 {
		containers["kubernetes"] = k8sInfo
	}

	return containers
}

// detectDatabases detects database-related tools and configurations
func (p *DevToolsPlugin) detectDatabases(_ context.Context) map[string]interface{} {
	databases := make(map[string]interface{})

	// Check for database configuration files
	dbFiles := map[string][]string{
		"mysql":      {"my.cnf", ".my.cnf"},
		"postgresql": {"postgresql.conf", "pg_hba.conf"},
		"mongodb":    {"mongod.conf", ".mongorc.js"},
		"redis":      {"redis.conf"},
		"sqlite":     {"*.db", "*.sqlite", "*.sqlite3"},
	}

	for dbType, files := range dbFiles {
		dbInfo := make(map[string]interface{})
		foundFiles := []string{}

		for _, file := range files {
			if strings.Contains(file, "*") {
				// Handle glob patterns
				matches, _ := filepath.Glob(file)
				foundFiles = append(foundFiles, matches...)
			} else {
				if _, err := os.Stat(file); err == nil {
					foundFiles = append(foundFiles, file)
				}
			}
		}

		if len(foundFiles) > 0 {
			dbInfo["config_files"] = foundFiles
			databases[dbType] = dbInfo
		}
	}

	return databases
}
