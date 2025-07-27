package plugins

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestEnvPlugin_Name(t *testing.T) {
	plugin := NewEnvPlugin()
	if plugin.Name() != "environment" {
		t.Errorf("Expected plugin name 'environment', got '%s'", plugin.Name())
	}
}

func TestEnvPlugin_Priority(t *testing.T) {
	plugin := NewEnvPlugin()
	if plugin.Priority() != 100 {
		t.Errorf("Expected plugin priority 100, got %d", plugin.Priority())
	}
}

func TestEnvPlugin_GatherContext(t *testing.T) {
	plugin := NewEnvPlugin()

	// Set up test environment variables
	testEnvVars := map[string]string{
		"PATH":           "/usr/bin:/bin",
		"HOME":           "/home/testuser",
		"USER":           "testuser",
		"SHELL":          "/bin/bash",
		"TERM":           "xterm-256color",
		"LANG":           "en_US.UTF-8",
		"EDITOR":         "vim",
		"GOPATH":         "/home/testuser/go",
		"NODE_ENV":       "development",
		"PYTHON_VERSION": "3.9.0",
		// Sensitive variables that should be filtered out
		"API_KEY":         "secret123",
		"PASSWORD":        "supersecret",
		"SECRET_TOKEN":    "token123",
		"SSH_PRIVATE_KEY": "privatekey",
	}

	// Set environment variables
	for key, value := range testEnvVars {
		os.Setenv(key, value)
	}
	defer func() {
		// Clean up
		for key := range testEnvVars {
			os.Unsetenv(key)
		}
	}()

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

	// Check variables section
	variables, ok := result["variables"].(map[string]string)
	if !ok {
		t.Fatal("Expected variables to be map[string]string")
	}

	// Check that important variables are included
	expectedVars := []string{"PATH", "HOME", "USER", "SHELL", "TERM", "LANG", "EDITOR", "GOPATH", "NODE_ENV", "PYTHON_VERSION"}
	for _, expectedVar := range expectedVars {
		if _, exists := variables[expectedVar]; !exists {
			t.Errorf("Expected variable %s to be included", expectedVar)
		}
	}

	// Check that sensitive variables are excluded
	sensitiveVars := []string{"API_KEY", "PASSWORD", "SECRET_TOKEN", "SSH_PRIVATE_KEY"}
	for _, sensitiveVar := range sensitiveVars {
		if _, exists := variables[sensitiveVar]; exists {
			t.Errorf("Expected sensitive variable %s to be excluded", sensitiveVar)
		}
	}

	// Check shell section
	shell, ok := result["shell"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected shell to be map[string]interface{}")
	}

	if shell["current"] != "/bin/bash" {
		t.Errorf("Expected shell current to be '/bin/bash', got '%v'", shell["current"])
	}

	if shell["name"] != "bash" {
		t.Errorf("Expected shell name to be 'bash', got '%v'", shell["name"])
	}

	// Check user section
	user, ok := result["user"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected user to be map[string]interface{}")
	}

	if user["name"] != "testuser" {
		t.Errorf("Expected user name to be 'testuser', got '%v'", user["name"])
	}

	if user["home"] != "/home/testuser" {
		t.Errorf("Expected user home to be '/home/testuser', got '%v'", user["home"])
	}

	// Check system section
	system, ok := result["system"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected system to be map[string]interface{}")
	}

	if system["language"] != "en_US.UTF-8" {
		t.Errorf("Expected system language to be 'en_US.UTF-8', got '%v'", system["language"])
	}
}

func TestEnvPlugin_GatherContext_WithCancellation(t *testing.T) {
	plugin := NewEnvPlugin()

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

func TestEnvPlugin_GatherContext_WithTimeout(t *testing.T) {
	plugin := NewEnvPlugin()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(2 * time.Millisecond)

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

func TestEnvPlugin_isSensitive(t *testing.T) {
	plugin := &EnvPlugin{}

	sensitiveVars := []string{
		"PASSWORD",
		"SECRET",
		"API_KEY",
		"TOKEN",
		"CREDENTIAL",
		"AUTH_TOKEN",
		"PRIVATE_KEY",
		"CERT_KEY",
		"SSH_KEY",
		"GPG_KEY",
		"OAUTH_TOKEN",
		"JWT_SECRET",
		"BEARER_TOKEN",
		"COOKIE_SECRET",
		"SESSION_KEY",
		"MY_PASSWORD",
		"DB_SECRET",
		"app_key", // lowercase
	}

	for _, sensitiveVar := range sensitiveVars {
		if !plugin.isSensitive(sensitiveVar) {
			t.Errorf("Expected %s to be identified as sensitive", sensitiveVar)
		}
	}

	nonSensitiveVars := []string{
		"PATH",
		"HOME",
		"USER",
		"SHELL",
		"TERM",
		"LANG",
		"EDITOR",
		"GOPATH",
		"NODE_ENV",
		"PYTHON_VERSION",
	}

	for _, nonSensitiveVar := range nonSensitiveVars {
		if plugin.isSensitive(nonSensitiveVar) {
			t.Errorf("Expected %s to NOT be identified as sensitive", nonSensitiveVar)
		}
	}
}

func TestEnvPlugin_isRelevant(t *testing.T) {
	plugin := &EnvPlugin{}

	relevantVars := []string{
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
		"JAVA_HOME",
		"PYTHON_PATH",
		"NODE_VERSION",
		"GOPATH",
		"GOROOT",
		"PYTHONPATH",
		"NODE_ENV",
		"DOCKER_HOST",
		"KUBECONFIG",
		"AWS_REGION",
		"CI_BUILD_ID",
		"BUILD_NUMBER",
		"DEPLOY_ENV",
	}

	for _, relevantVar := range relevantVars {
		if !plugin.isRelevant(relevantVar) {
			t.Errorf("Expected %s to be identified as relevant", relevantVar)
		}
	}

	irrelevantVars := []string{
		"RANDOM_VAR",
		"SOME_OTHER_VAR",
		"UNRELATED_SETTING",
	}

	for _, irrelevantVar := range irrelevantVars {
		if plugin.isRelevant(irrelevantVar) {
			t.Errorf("Expected %s to NOT be identified as relevant", irrelevantVar)
		}
	}
}

func TestEnvPlugin_extractShellName(t *testing.T) {
	plugin := &EnvPlugin{}

	testCases := []struct {
		shellPath string
		expected  string
	}{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"/bin/sh", "sh"},
		{"fish", "fish"},
		{"", ""},
		{"/usr/local/bin/fish", "fish"},
	}

	for _, tc := range testCases {
		result := plugin.extractShellName(tc.shellPath)
		if result != tc.expected {
			t.Errorf("extractShellName(%s) = %s, expected %s", tc.shellPath, result, tc.expected)
		}
	}
}

func TestEnvPlugin_getShellInfo(t *testing.T) {
	plugin := &EnvPlugin{}

	// Set test environment variables
	os.Setenv("SHELL", "/bin/bash")
	os.Setenv("TERM", "xterm-256color")
	defer func() {
		os.Unsetenv("SHELL")
		os.Unsetenv("TERM")
	}()

	shellInfo := plugin.getShellInfo()

	if shellInfo["current"] != "/bin/bash" {
		t.Errorf("Expected shell current to be '/bin/bash', got '%v'", shellInfo["current"])
	}

	if shellInfo["name"] != "bash" {
		t.Errorf("Expected shell name to be 'bash', got '%v'", shellInfo["name"])
	}

	if shellInfo["terminal"] != "xterm-256color" {
		t.Errorf("Expected terminal to be 'xterm-256color', got '%v'", shellInfo["terminal"])
	}
}

func TestEnvPlugin_getUserInfo(t *testing.T) {
	plugin := &EnvPlugin{}

	// Set test environment variables
	os.Setenv("USER", "testuser")
	os.Setenv("HOME", "/home/testuser")
	defer func() {
		os.Unsetenv("USER")
		os.Unsetenv("HOME")
	}()

	userInfo := plugin.getUserInfo()

	if userInfo["name"] != "testuser" {
		t.Errorf("Expected user name to be 'testuser', got '%v'", userInfo["name"])
	}

	if userInfo["home"] != "/home/testuser" {
		t.Errorf("Expected user home to be '/home/testuser', got '%v'", userInfo["home"])
	}
}

func TestEnvPlugin_getUserInfo_WithUsername(t *testing.T) {
	plugin := &EnvPlugin{}

	// Unset USER and set USERNAME (Windows style)
	os.Unsetenv("USER")
	os.Setenv("USERNAME", "windowsuser")
	defer os.Unsetenv("USERNAME")

	userInfo := plugin.getUserInfo()

	if userInfo["name"] != "windowsuser" {
		t.Errorf("Expected user name to be 'windowsuser', got '%v'", userInfo["name"])
	}
}

func TestEnvPlugin_getSystemInfo(t *testing.T) {
	plugin := &EnvPlugin{}

	// Set test environment variables
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("LC_ALL", "en_US.UTF-8")
	os.Setenv("TMPDIR", "/tmp")
	os.Setenv("XDG_CONFIG_HOME", "/home/user/.config")
	os.Setenv("XDG_DATA_HOME", "/home/user/.local/share")
	os.Setenv("XDG_CACHE_HOME", "/home/user/.cache")

	defer func() {
		os.Unsetenv("LANG")
		os.Unsetenv("LC_ALL")
		os.Unsetenv("TMPDIR")
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_DATA_HOME")
		os.Unsetenv("XDG_CACHE_HOME")
	}()

	systemInfo := plugin.getSystemInfo()

	if systemInfo["language"] != "en_US.UTF-8" {
		t.Errorf("Expected language to be 'en_US.UTF-8', got '%v'", systemInfo["language"])
	}

	if systemInfo["locale"] != "en_US.UTF-8" {
		t.Errorf("Expected locale to be 'en_US.UTF-8', got '%v'", systemInfo["locale"])
	}

	tempDirs, ok := systemInfo["temp_dirs"].([]string)
	if !ok {
		t.Fatal("Expected temp_dirs to be []string")
	}

	if len(tempDirs) == 0 || tempDirs[0] != "/tmp" {
		t.Errorf("Expected temp_dirs to contain '/tmp', got %v", tempDirs)
	}

	xdgDirs, ok := systemInfo["xdg_dirs"].(map[string]string)
	if !ok {
		t.Fatal("Expected xdg_dirs to be map[string]string")
	}

	if xdgDirs["config"] != "/home/user/.config" {
		t.Errorf("Expected XDG config dir to be '/home/user/.config', got '%v'", xdgDirs["config"])
	}

	if xdgDirs["data"] != "/home/user/.local/share" {
		t.Errorf("Expected XDG data dir to be '/home/user/.local/share', got '%v'", xdgDirs["data"])
	}

	if xdgDirs["cache"] != "/home/user/.cache" {
		t.Errorf("Expected XDG cache dir to be '/home/user/.cache', got '%v'", xdgDirs["cache"])
	}
}

func TestEnvPlugin_Integration(t *testing.T) {
	plugin := NewEnvPlugin()

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
	expectedSections := []string{"variables", "shell", "user", "system"}
	for _, section := range expectedSections {
		if _, exists := result[section]; !exists {
			t.Errorf("Expected section '%s' to be present in result", section)
		}
	}
}
