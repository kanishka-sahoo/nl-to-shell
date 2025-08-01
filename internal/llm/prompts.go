package llm

import (
	"fmt"
	"strings"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// PromptBuilder provides common prompt building functionality for all LLM providers
type PromptBuilder struct{}

// NewPromptBuilder creates a new prompt builder instance
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildSystemPrompt creates the system prompt for command generation
func (pb *PromptBuilder) BuildSystemPrompt(context *types.Context) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert shell command generator. Convert natural language requests into safe, accurate shell commands.\n\n")
	prompt.WriteString("Rules:\n")
	prompt.WriteString("1. Generate only the shell command, no explanations unless asked\n")
	prompt.WriteString("2. Prefer safe, non-destructive commands\n")
	prompt.WriteString("3. Use standard Unix/Linux commands when possible\n")
	prompt.WriteString("4. Consider the current context when generating commands\n\n")

	if context != nil {
		if context.WorkingDirectory != "" {
			prompt.WriteString(fmt.Sprintf("Current directory: %s\n", context.WorkingDirectory))
		}

		if len(context.Files) > 0 {
			prompt.WriteString("Files in current directory:\n")
			for _, file := range context.Files {
				if file.IsDir {
					prompt.WriteString(fmt.Sprintf("  %s/ (directory)\n", file.Name))
				} else {
					prompt.WriteString(fmt.Sprintf("  %s (file, %d bytes)\n", file.Name, file.Size))
				}
			}
		}

		if context.GitInfo != nil && context.GitInfo.IsRepository {
			prompt.WriteString(fmt.Sprintf("Git repository: branch '%s'", context.GitInfo.CurrentBranch))
			if context.GitInfo.HasUncommittedChanges {
				prompt.WriteString(" (has uncommitted changes)")
			}
			prompt.WriteString("\n")
		}
	}

	prompt.WriteString("\nRespond with a JSON object containing:\n")
	prompt.WriteString("- 'command': the shell command\n")
	prompt.WriteString("- 'explanation': brief explanation of what the command does\n")
	prompt.WriteString("- 'confidence': confidence level (0.0-1.0)\n")
	prompt.WriteString("- 'alternatives': array of alternative commands (optional)\n")

	return prompt.String()
}

// BuildValidationPrompt creates the prompt for result validation
func (pb *PromptBuilder) BuildValidationPrompt(command, output, intent string) string {
	return fmt.Sprintf(`Analyze this command execution:

Intent: %s
Command: %s
Output: %s

Determine if the command output matches the user's intent. Consider:
1. Did the command execute successfully?
2. Does the output indicate the intended action was completed?
3. Are there any error messages or warnings?
4. Does the result align with what the user wanted to achieve?

Respond with JSON containing:
- is_correct: boolean indicating if the result matches intent
- explanation: detailed explanation of your analysis
- suggestions: array of improvement suggestions (if any)
- correction: corrected command if the original was wrong (optional)`, intent, command, output)
}

// BuildValidationSystemPrompt creates the system prompt for result validation
func (pb *PromptBuilder) BuildValidationSystemPrompt() string {
	return "You are an expert system administrator. Analyze command execution results and determine if they match the user's intent. Respond with a JSON object containing 'is_correct' (boolean), 'explanation' (string), 'suggestions' (array of strings), and 'correction' (string if needed)."
}
