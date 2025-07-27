package plugins

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestDevToolsPlugin_Name(t *testing.T) {
	plugin := NewDevToolsPlugin()
	if plugin.Name() != "devtools" {
		t.Errorf("Expected plugin name 'devtools', got '%s'", plugin.Name())
	}
}

func TestDevToolsPlugin_Priority(t *testing.T) {
	plugin := NewDevToolsPlugin()
	if plugin.Priority() != 90 {
		t.Errorf("Expected plugin priority 90, got %d", plugin.Priority())
	}
}

func TestDevToolsPlugin_GatherContext(t *testing.T) {
	plugin := NewDevToolsPlugin()

	ctx := context.Background()
	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Fatalf("GatherContext failed: %v", err)
	}

	// Check that result has expected structure
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check that all expected sections are present
	expectedSections := []string{"tools", "runtimes", "containers", "databases"}
	for _, section := range expectedSections {
		if _, exists := result[section]; !exists {
			t.Errorf("Expected section '%s' to be present in result", section)
		}
	}

	// Check tools section
	tools, ok := result["tools"].(map[string]ToolInfo)
	if !ok {
		t.Fatal("Expected tools to be map[string]ToolInfo")
	}

	// Check that some common tools are checked (they may not be available)
	expectedTools := []string{"git", "docker", "node", "python", "go", "java"}
	for _, expectedTool := range expectedTools {
		if _, exists := tools[expectedTool]; !exists {
			t.Errorf("Expected tool '%s' to be checked", expectedTool)
		}
	}
}

func TestDevToolsPlugin_GatherContext_WithCancellation(t *testing.T) {
	plugin := NewDevToolsPlugin()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}

	if result != nil {
		t.Error("Expected nil result due to cancelled context")
	}
}

func TestDevToolsPlugin_detectTool(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Test with a tool that should exist on most systems
	toolInfo := plugin.detectTool("echo", "echo", "--help", "")

	if !toolInfo.Available {
		t.Error("Expected echo to be available")
	}

	if toolInfo.Name != "echo" {
		t.Errorf("Expected tool name 'echo', got '%s'", toolInfo.Name)
	}

	if toolInfo.Path == "" {
		t.Error("Expected non-empty path for available tool")
	}

	// Test with a tool that shouldn't exist
	nonExistentTool := plugin.detectTool("nonexistent", "nonexistent-tool-12345", "--version", "")

	if nonExistentTool.Available {
		t.Error("Expected nonexistent tool to not be available")
	}

	if nonExistentTool.Path != "" {
		t.Error("Expected empty path for non-available tool")
	}
}

func TestDevToolsPlugin_detectTool_WithVersion(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping version test")
	}

	toolInfo := plugin.detectTool("git", "git", "--version", `git version (.+)`)

	if !toolInfo.Available {
		t.Error("Expected git to be available")
	}

	if toolInfo.Version == "" {
		t.Error("Expected non-empty version for git")
	}
}

func TestDevToolsPlugin_detectNodeRuntime(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test without package.json
	nodeInfo := plugin.detectNodeRuntime()
	if nodeInfo != nil {
		t.Error("Expected nil nodeInfo when no package.json exists")
	}

	// Create package.json
	packageJSON := `{"name": "test", "version": "1.0.0"}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	nodeInfo = plugin.detectNodeRuntime()
	if nodeInfo == nil {
		t.Fatal("Expected non-nil nodeInfo when package.json exists")
	}

	if !nodeInfo["has_package_json"].(bool) {
		t.Error("Expected has_package_json to be true")
	}

	// Create node_modules directory
	err = os.Mkdir("node_modules", 0755)
	if err != nil {
		t.Fatalf("Failed to create node_modules: %v", err)
	}

	nodeInfo = plugin.detectNodeRuntime()
	if !nodeInfo["has_node_modules"].(bool) {
		t.Error("Expected has_node_modules to be true")
	}

	// Create yarn.lock
	err = os.WriteFile("yarn.lock", []byte("# yarn lockfile"), 0644)
	if err != nil {
		t.Fatalf("Failed to create yarn.lock: %v", err)
	}

	nodeInfo = plugin.detectNodeRuntime()
	configFiles, ok := nodeInfo["config_files"].([]string)
	if !ok {
		t.Fatal("Expected config_files to be []string")
	}

	found := false
	for _, file := range configFiles {
		if file == "yarn.lock" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected yarn.lock to be in config_files")
	}
}

func TestDevToolsPlugin_detectPythonRuntime(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test without Python files
	pythonInfo := plugin.detectPythonRuntime()
	if pythonInfo != nil {
		t.Error("Expected nil pythonInfo when no Python files exist")
	}

	// Create requirements.txt
	requirements := "requests==2.25.1\nflask==1.1.2"
	err = os.WriteFile("requirements.txt", []byte(requirements), 0644)
	if err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	pythonInfo = plugin.detectPythonRuntime()
	if pythonInfo == nil {
		t.Fatal("Expected non-nil pythonInfo when requirements.txt exists")
	}

	configFiles, ok := pythonInfo["config_files"].([]string)
	if !ok {
		t.Fatal("Expected config_files to be []string")
	}

	found := false
	for _, file := range configFiles {
		if file == "requirements.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected requirements.txt to be in config_files")
	}

	// Create virtual environment directory
	err = os.Mkdir("venv", 0755)
	if err != nil {
		t.Fatalf("Failed to create venv: %v", err)
	}

	pythonInfo = plugin.detectPythonRuntime()
	if pythonInfo["virtual_env"] != "venv" {
		t.Errorf("Expected virtual_env to be 'venv', got '%v'", pythonInfo["virtual_env"])
	}
}

func TestDevToolsPlugin_detectJavaRuntime(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test without Java files
	javaInfo := plugin.detectJavaRuntime()
	if javaInfo != nil {
		t.Error("Expected nil javaInfo when no Java files exist")
	}

	// Create pom.xml
	pomXML := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>`
	err = os.WriteFile("pom.xml", []byte(pomXML), 0644)
	if err != nil {
		t.Fatalf("Failed to create pom.xml: %v", err)
	}

	javaInfo = plugin.detectJavaRuntime()
	if javaInfo == nil {
		t.Fatal("Expected non-nil javaInfo when pom.xml exists")
	}

	configFiles, ok := javaInfo["config_files"].([]string)
	if !ok {
		t.Fatal("Expected config_files to be []string")
	}

	found := false
	for _, file := range configFiles {
		if file == "pom.xml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pom.xml to be in config_files")
	}

	// Create src/main/java directory
	err = os.MkdirAll("src/main/java", 0755)
	if err != nil {
		t.Fatalf("Failed to create src/main/java: %v", err)
	}

	javaInfo = plugin.detectJavaRuntime()
	projectDirs, ok := javaInfo["project_dirs"].([]string)
	if !ok {
		t.Fatal("Expected project_dirs to be []string")
	}

	found = false
	for _, dir := range projectDirs {
		if dir == "src/main/java" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected src/main/java to be in project_dirs")
	}
}

func TestDevToolsPlugin_detectGoRuntime(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test without Go files
	goInfo := plugin.detectGoRuntime()
	if goInfo != nil {
		t.Error("Expected nil goInfo when no Go files exist")
	}

	// Create go.mod
	goMod := `module example.com/test

go 1.19

require (
    github.com/gorilla/mux v1.8.0
)`
	err = os.WriteFile("go.mod", []byte(goMod), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	goInfo = plugin.detectGoRuntime()
	if goInfo == nil {
		t.Fatal("Expected non-nil goInfo when go.mod exists")
	}

	configFiles, ok := goInfo["config_files"].([]string)
	if !ok {
		t.Fatal("Expected config_files to be []string")
	}

	found := false
	for _, file := range configFiles {
		if file == "go.mod" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected go.mod to be in config_files")
	}

	// Create a Go source file
	goFile := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`
	err = os.WriteFile("main.go", []byte(goFile), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	goInfo = plugin.detectGoRuntime()
	if !goInfo["has_go_files"].(bool) {
		t.Error("Expected has_go_files to be true")
	}
}

func TestDevToolsPlugin_detectContainers(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without container files
	containers := plugin.detectContainers(ctx)
	if len(containers) > 0 {
		t.Error("Expected empty containers when no container files exist")
	}

	// Create Dockerfile
	dockerfile := `FROM node:14
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`
	err = os.WriteFile("Dockerfile", []byte(dockerfile), 0644)
	if err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	containers = plugin.detectContainers(ctx)
	dockerInfo, exists := containers["docker"]
	if !exists {
		t.Fatal("Expected docker info to exist")
	}

	dockerMap, ok := dockerInfo.(map[string]interface{})
	if !ok {
		t.Fatal("Expected docker info to be map[string]interface{}")
	}

	if !dockerMap["has_dockerfile"].(bool) {
		t.Error("Expected has_dockerfile to be true")
	}

	// Create docker-compose.yml
	dockerCompose := `version: '3.8'
services:
  web:
    build: .
    ports:
      - "3000:3000"`
	err = os.WriteFile("docker-compose.yml", []byte(dockerCompose), 0644)
	if err != nil {
		t.Fatalf("Failed to create docker-compose.yml: %v", err)
	}

	containers = plugin.detectContainers(ctx)
	dockerInfo = containers["docker"]
	dockerMap = dockerInfo.(map[string]interface{})

	if !dockerMap["has_compose"].(bool) {
		t.Error("Expected has_compose to be true")
	}
}

func TestDevToolsPlugin_detectDatabases(t *testing.T) {
	plugin := &DevToolsPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devtools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without database files
	databases := plugin.detectDatabases(ctx)
	if len(databases) > 0 {
		t.Error("Expected empty databases when no database files exist")
	}

	// Create SQLite database file
	err = os.WriteFile("test.db", []byte("SQLite format 3"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test.db: %v", err)
	}

	databases = plugin.detectDatabases(ctx)
	sqliteInfo, exists := databases["sqlite"]
	if !exists {
		t.Fatal("Expected sqlite info to exist")
	}

	sqliteMap, ok := sqliteInfo.(map[string]interface{})
	if !ok {
		t.Fatal("Expected sqlite info to be map[string]interface{}")
	}

	configFiles, ok := sqliteMap["config_files"].([]string)
	if !ok {
		t.Fatal("Expected config_files to be []string")
	}

	found := false
	for _, file := range configFiles {
		if file == "test.db" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test.db to be in config_files")
	}
}

func TestDevToolsPlugin_Integration(t *testing.T) {
	plugin := NewDevToolsPlugin()

	// Test that the plugin implements the ContextPlugin interface correctly
	if plugin.Name() == "" {
		t.Error("Plugin name should not be empty")
	}

	if plugin.Priority() < 0 {
		t.Error("Plugin priority should be non-negative")
	}

	ctx := context.Background()
	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Fatalf("GatherContext should not fail: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Check that all expected sections are present
	expectedSections := []string{"tools", "runtimes", "containers", "databases"}
	for _, section := range expectedSections {
		if _, exists := result[section]; !exists {
			t.Errorf("Expected section '%s' to be present in result", section)
		}
	}
}

func TestDevToolsPlugin_GatherContext_WithTimeout(t *testing.T) {
	plugin := NewDevToolsPlugin()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(1 * time.Millisecond)

	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err == nil {
		t.Error("Expected error due to timeout")
	}

	if result != nil {
		t.Error("Expected nil result due to timeout")
	}
}
