package llm

import (
	"strings"
	"testing"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestPromptBuilder_BuildSystemPrompt(t *testing.T) {
	builder := NewPromptBuilder()

	context := &types.Context{
		WorkingDirectory: "/home/user/project",
		Files: []types.FileInfo{
			{Name: "main.go", IsDir: false, Size: 1024},
			{Name: "src", IsDir: true},
		},
		GitInfo: &types.GitContext{
			IsRepository:          true,
			CurrentBranch:         "main",
			HasUncommittedChanges: true,
		},
	}

	prompt := builder.BuildSystemPrompt(context)

	// Check that the prompt contains expected elements
	if !strings.Contains(prompt, "expert shell command generator") {
		t.Error("Prompt should contain role description")
	}

	if !strings.Contains(prompt, "/home/user/project") {
		t.Error("Prompt should contain working directory")
	}

	if !strings.Contains(prompt, "main.go") {
		t.Error("Prompt should contain file information")
	}

	if !strings.Contains(prompt, "branch 'main'") {
		t.Error("Prompt should contain git information")
	}

	if !strings.Contains(prompt, "has uncommitted changes") {
		t.Error("Prompt should contain git status")
	}

	if !strings.Contains(prompt, "JSON object") {
		t.Error("Prompt should specify JSON response format")
	}
}

func TestPromptBuilder_BuildSystemPrompt_NilContext(t *testing.T) {
	builder := NewPromptBuilder()

	prompt := builder.BuildSystemPrompt(nil)

	// Should still have basic structure
	if !strings.Contains(prompt, "expert shell command generator") {
		t.Error("Prompt should contain role description even with nil context")
	}

	if !strings.Contains(prompt, "JSON object") {
		t.Error("Prompt should specify JSON response format even with nil context")
	}

	// Should not contain context-specific information
	if strings.Contains(prompt, "Current directory") {
		t.Error("Prompt should not contain directory info with nil context")
	}
}

func TestPromptBuilder_BuildValidationPrompt(t *testing.T) {
	builder := NewPromptBuilder()

	command := "ls -la"
	output := "total 8\n-rw-r--r-- 1 user user 100 Jan 1 12:00 test.txt"
	intent := "list all files"

	prompt := builder.BuildValidationPrompt(command, output, intent)

	// Check that all parameters are included
	if !strings.Contains(prompt, command) {
		t.Error("Prompt should contain command")
	}

	if !strings.Contains(prompt, output) {
		t.Error("Prompt should contain output")
	}

	if !strings.Contains(prompt, intent) {
		t.Error("Prompt should contain intent")
	}

	// Check analysis criteria
	if !strings.Contains(prompt, "execute successfully") {
		t.Error("Prompt should contain execution criteria")
	}

	if !strings.Contains(prompt, "is_correct") {
		t.Error("Prompt should specify expected JSON fields")
	}
}

func TestPromptBuilder_BuildValidationSystemPrompt(t *testing.T) {
	builder := NewPromptBuilder()

	prompt := builder.BuildValidationSystemPrompt()

	// Check basic requirements
	if !strings.Contains(prompt, "expert system administrator") {
		t.Error("Prompt should contain role description")
	}

	if !strings.Contains(prompt, "JSON object") {
		t.Error("Prompt should specify JSON response format")
	}

	if !strings.Contains(prompt, "is_correct") {
		t.Error("Prompt should specify required fields")
	}
}
