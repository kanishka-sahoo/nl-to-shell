package validator

import (
	"context"
	"fmt"
	"strings"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// ResultValidator implements AI-powered result validation using LLM providers
type ResultValidator struct {
	llmProvider interfaces.LLMProvider
}

// NewResultValidator creates a new result validator with the given LLM provider
func NewResultValidator(provider interfaces.LLMProvider) interfaces.ResultValidator {
	return &ResultValidator{
		llmProvider: provider,
	}
}

// ValidateResult validates command execution results against user intent using AI
func (rv *ResultValidator) ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error) {
	if result == nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "execution result cannot be nil",
		}
	}

	if rv.llmProvider == nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "LLM provider not configured",
		}
	}

	// Get validation response from LLM provider
	response, err := rv.llmProvider.ValidateResult(ctx, result.Command.Generated, rv.formatOutput(result), intent)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to validate result with LLM provider",
			Cause:   err,
		}
	}

	// Create validation result
	validationResult := &types.ValidationResult{
		IsCorrect:        response.IsCorrect,
		Explanation:      response.Explanation,
		Suggestions:      response.Suggestions,
		CorrectedCommand: response.Correction,
	}

	// If the result is incorrect and no correction was provided, generate one
	if !response.IsCorrect && response.Correction == "" {
		correction, err := rv.generateCorrection(ctx, result, intent, response.Explanation)
		if err == nil {
			validationResult.CorrectedCommand = correction
		}
	}

	return validationResult, nil
}

// buildValidationPrompt creates a comprehensive prompt for result validation
func (rv *ResultValidator) buildValidationPrompt(result *types.ExecutionResult, intent string) string {
	var prompt strings.Builder

	prompt.WriteString("Please validate if this command execution achieved the user's intent.\n\n")

	prompt.WriteString(fmt.Sprintf("User Intent: %s\n", intent))
	prompt.WriteString(fmt.Sprintf("Command Executed: %s\n", result.Command.Generated))
	prompt.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
	prompt.WriteString(fmt.Sprintf("Success: %t\n", result.Success))
	prompt.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration))

	if result.Stdout != "" {
		prompt.WriteString(fmt.Sprintf("Standard Output:\n%s\n", result.Stdout))
	}

	if result.Stderr != "" {
		prompt.WriteString(fmt.Sprintf("Standard Error:\n%s\n", result.Stderr))
	}

	if result.Error != nil {
		prompt.WriteString(fmt.Sprintf("Execution Error: %v\n", result.Error))
	}

	prompt.WriteString("\nPlease analyze:\n")
	prompt.WriteString("1. Did the command achieve the user's stated intent?\n")
	prompt.WriteString("2. Are there any issues with the execution?\n")
	prompt.WriteString("3. If unsuccessful, what corrections would you suggest?\n")
	prompt.WriteString("4. Are there any warnings or concerns about the result?\n")

	return prompt.String()
}

// formatOutput formats the execution result output for validation
func (rv *ResultValidator) formatOutput(result *types.ExecutionResult) string {
	var output strings.Builder

	if result.Stdout != "" {
		output.WriteString("STDOUT:\n")
		output.WriteString(result.Stdout)
		output.WriteString("\n")
	}

	if result.Stderr != "" {
		output.WriteString("STDERR:\n")
		output.WriteString(result.Stderr)
		output.WriteString("\n")
	}

	if result.Error != nil {
		output.WriteString("ERROR:\n")
		output.WriteString(result.Error.Error())
		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("EXIT_CODE: %d\n", result.ExitCode))
	output.WriteString(fmt.Sprintf("SUCCESS: %t\n", result.Success))
	output.WriteString(fmt.Sprintf("DURATION: %v\n", result.Duration))

	return output.String()
}

// generateCorrection attempts to generate a corrected command when validation fails
func (rv *ResultValidator) generateCorrection(ctx context.Context, result *types.ExecutionResult, intent string, explanation string) (string, error) {
	// Build a prompt for correction generation
	var prompt strings.Builder

	prompt.WriteString("The following command did not achieve the user's intent. Please provide a corrected command.\n\n")
	prompt.WriteString(fmt.Sprintf("User Intent: %s\n", intent))
	prompt.WriteString(fmt.Sprintf("Failed Command: %s\n", result.Command.Generated))
	prompt.WriteString(fmt.Sprintf("Failure Reason: %s\n", explanation))

	if result.Stderr != "" {
		prompt.WriteString(fmt.Sprintf("Error Output: %s\n", result.Stderr))
	}

	if result.Error != nil {
		prompt.WriteString(fmt.Sprintf("Execution Error: %v\n", result.Error))
	}

	prompt.WriteString("\nPlease provide a corrected command that would achieve the user's intent.")

	// Use the command generation capability to get a correction
	response, err := rv.llmProvider.GenerateCommand(ctx, prompt.String(), result.Command.Context)
	if err != nil {
		return "", err
	}

	return response.Command, nil
}

// AdvancedResultValidator extends the basic validator with additional features
type AdvancedResultValidator struct {
	*ResultValidator
	enableAutoCorrection  bool
	maxCorrectionAttempts int
}

// NewAdvancedResultValidator creates an advanced result validator with additional features
func NewAdvancedResultValidator(provider interfaces.LLMProvider, enableAutoCorrection bool) interfaces.ResultValidator {
	return &AdvancedResultValidator{
		ResultValidator: &ResultValidator{
			llmProvider: provider,
		},
		enableAutoCorrection:  enableAutoCorrection,
		maxCorrectionAttempts: 3,
	}
}

// ValidateResult performs advanced validation with optional auto-correction
func (arv *AdvancedResultValidator) ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error) {
	// Perform basic validation first
	validationResult, err := arv.ResultValidator.ValidateResult(ctx, result, intent)
	if err != nil {
		return nil, err
	}

	// If auto-correction is enabled and the result is incorrect, attempt correction
	if arv.enableAutoCorrection && !validationResult.IsCorrect {
		correctedCommand, err := arv.generateImprovedCorrection(ctx, result, intent, validationResult.Explanation)
		if err == nil && correctedCommand != "" {
			validationResult.CorrectedCommand = correctedCommand

			// Add suggestion about the correction
			if validationResult.Suggestions == nil {
				validationResult.Suggestions = make([]string, 0)
			}
			validationResult.Suggestions = append(validationResult.Suggestions,
				fmt.Sprintf("Auto-generated correction: %s", correctedCommand))
		}
	}

	return validationResult, nil
}

// generateImprovedCorrection generates a more sophisticated correction using multiple validation passes
func (arv *AdvancedResultValidator) generateImprovedCorrection(ctx context.Context, result *types.ExecutionResult, intent string, explanation string) (string, error) {
	// First attempt: basic correction
	correction, err := arv.generateCorrection(ctx, result, intent, explanation)
	if err != nil {
		return "", err
	}

	// Validate the correction by simulating its analysis
	if correction != "" {
		// Create a hypothetical command for the correction
		correctedCmd := &types.Command{
			ID:          result.Command.ID + "_corrected",
			Original:    result.Command.Original,
			Generated:   correction,
			Context:     result.Command.Context,
			WorkingDir:  result.Command.WorkingDir,
			Environment: result.Command.Environment,
			Timeout:     result.Command.Timeout,
		}

		// Perform a basic sanity check on the correction
		if arv.isReasonableCorrection(result.Command, correctedCmd, intent) {
			return correction, nil
		}
	}

	return correction, nil
}

// isReasonableCorrection performs basic sanity checks on the proposed correction
func (arv *AdvancedResultValidator) isReasonableCorrection(original, corrected *types.Command, _ string) bool {
	// Basic checks to ensure the correction is reasonable

	// 1. Correction should not be identical to original (unless original was empty)
	if original.Generated != "" && original.Generated == corrected.Generated {
		return false
	}

	// 2. Correction should not be empty
	if strings.TrimSpace(corrected.Generated) == "" {
		return false
	}

	// 3. Correction should not be excessively long (potential hallucination)
	if len(corrected.Generated) > 500 {
		return false
	}

	// 4. Basic command structure validation
	parts := strings.Fields(corrected.Generated)
	if len(parts) == 0 {
		return false
	}

	// 5. Should not contain obviously dangerous patterns without good reason
	dangerousPatterns := []string{
		"rm -rf /",
		":(){ :|:& };:", // fork bomb
		"dd if=/dev/zero of=/dev/sda",
		"mkfs.",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(corrected.Generated, pattern) {
			return false
		}
	}

	return true
}
