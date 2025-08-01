package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// Manager implements the CommandManager interface and orchestrates the command generation pipeline
type Manager struct {
	contextGatherer interfaces.ContextGatherer
	llmProvider     interfaces.LLMProvider
	safetyValidator interfaces.SafetyValidator
	executor        interfaces.CommandExecutor
	resultValidator interfaces.ResultValidator
	config          *types.Config
}

// NewManager creates a new command manager with the provided dependencies
func NewManager(
	contextGatherer interfaces.ContextGatherer,
	llmProvider interfaces.LLMProvider,
	safetyValidator interfaces.SafetyValidator,
	executor interfaces.CommandExecutor,
	resultValidator interfaces.ResultValidator,
	config *types.Config,
) *Manager {
	return &Manager{
		contextGatherer: contextGatherer,
		llmProvider:     llmProvider,
		safetyValidator: safetyValidator,
		executor:        executor,
		resultValidator: resultValidator,
		config:          config,
	}
}

// GenerateCommand implements the main command generation pipeline
func (m *Manager) GenerateCommand(ctx context.Context, input string) (*types.CommandResult, error) {
	// Step 1: Gather context
	context, err := m.contextGatherer.GatherContext(ctx)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to gather context",
			Cause:   err,
			Context: map[string]interface{}{
				"input": input,
			},
		}
	}

	// Step 2: Generate command using LLM
	response, err := m.llmProvider.GenerateCommand(ctx, input, context)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to generate command",
			Cause:   err,
			Context: map[string]interface{}{
				"input":   input,
				"context": context,
			},
		}
	}

	// Step 3: Create command object
	command := &types.Command{
		ID:          generateCommandID(),
		Original:    input,
		Generated:   response.Command,
		Context:     context,
		Timestamp:   time.Now(),
		WorkingDir:  context.WorkingDirectory,
		Environment: context.Environment,
		Timeout:     m.getCommandTimeout(),
	}

	// Step 4: Validate command safety
	safetyResult, err := m.safetyValidator.ValidateCommand(command)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to validate command safety",
			Cause:   err,
			Context: map[string]interface{}{
				"command": command.Generated,
			},
		}
	}

	// Mark command as validated
	command.Validated = safetyResult.IsSafe

	// Step 5: Create and return command result
	result := &types.CommandResult{
		Command:      command,
		Safety:       safetyResult,
		Confidence:   response.Confidence,
		Alternatives: response.Alternatives,
	}

	return result, nil
}

// ExecuteCommand executes a validated command
func (m *Manager) ExecuteCommand(ctx context.Context, cmd *types.Command) (*types.ExecutionResult, error) {
	if !cmd.Validated {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "command has not been validated for safety",
			Context: map[string]interface{}{
				"command": cmd.Generated,
			},
		}
	}

	// Execute the command
	result, err := m.executor.Execute(ctx, cmd)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "failed to execute command",
			Cause:   err,
			Context: map[string]interface{}{
				"command": cmd.Generated,
			},
		}
	}

	return result, nil
}

// ValidateResult validates the execution result using AI
func (m *Manager) ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error) {
	validation, err := m.resultValidator.ValidateResult(ctx, result, intent)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to validate execution result",
			Cause:   err,
			Context: map[string]interface{}{
				"command":   result.Command.Generated,
				"exit_code": result.ExitCode,
				"intent":    intent,
			},
		}
	}

	return validation, nil
}

// GenerateAndExecute is a convenience method that runs the full pipeline
func (m *Manager) GenerateAndExecute(ctx context.Context, input string, options *types.ExecutionOptions) (*types.FullResult, error) {
	// Step 1: Generate command
	commandResult, err := m.GenerateCommand(ctx, input)
	if err != nil {
		return nil, err
	}

	// Step 2: Check if we should skip execution (dry run)
	if options != nil && options.DryRun {
		dryRunResult, err := m.executor.DryRun(commandResult.Command)
		if err != nil {
			return nil, &types.NLShellError{
				Type:    types.ErrTypeExecution,
				Message: "failed to perform dry run",
				Cause:   err,
			}
		}

		return &types.FullResult{
			CommandResult: commandResult,
			DryRunResult:  dryRunResult,
		}, nil
	}

	// Step 3: Check safety requirements
	if commandResult.Safety.RequiresConfirmation && (options == nil || !options.SkipConfirmation) {
		return &types.FullResult{
			CommandResult:        commandResult,
			RequiresConfirmation: true,
		}, nil
	}

	// If command is not safe and we're not skipping confirmation, we need to mark it as validated
	// when the user explicitly skips confirmation (this simulates user approval)
	if !commandResult.Command.Validated && options != nil && options.SkipConfirmation {
		commandResult.Command.Validated = true
	}

	// Step 4: Execute command
	executionResult, err := m.ExecuteCommand(ctx, commandResult.Command)
	if err != nil {
		return nil, err
	}

	// Step 5: Validate results if requested
	var validationResult *types.ValidationResult
	if options == nil || options.ValidateResults {
		validationResult, err = m.ValidateResult(ctx, executionResult, input)
		if err != nil {
			// Don't fail the entire operation if validation fails
			// Just log the error and continue
			validationResult = &types.ValidationResult{
				IsCorrect:   false,
				Explanation: fmt.Sprintf("Validation failed: %v", err),
			}
		}
	}

	return &types.FullResult{
		CommandResult:    commandResult,
		ExecutionResult:  executionResult,
		ValidationResult: validationResult,
	}, nil
}

// Helper functions

func generateCommandID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}

func (m *Manager) getCommandTimeout() time.Duration {
	if m.config != nil && m.config.UserPreferences.DefaultTimeout > 0 {
		return m.config.UserPreferences.DefaultTimeout
	}
	return 30 * time.Second // Default timeout
}
