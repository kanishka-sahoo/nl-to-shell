package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// Executor implements the CommandExecutor interface
type Executor struct {
	defaultTimeout time.Duration
	workingDir     string
}

// NewExecutor creates a new command executor with default settings
func NewExecutor() interfaces.CommandExecutor {
	return &Executor{
		defaultTimeout: 30 * time.Second,
		workingDir:     "",
	}
}

// NewExecutorWithTimeout creates a new command executor with a custom timeout
func NewExecutorWithTimeout(timeout time.Duration) interfaces.CommandExecutor {
	return &Executor{
		defaultTimeout: timeout,
		workingDir:     "",
	}
}

// Execute runs the given command and returns the execution result
func (e *Executor) Execute(ctx context.Context, cmd *types.Command) (*types.ExecutionResult, error) {
	if cmd == nil {
		err := &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "command cannot be nil",
		}
		return &types.ExecutionResult{
			Command:  nil,
			ExitCode: -1,
			Success:  false,
			Duration: 0,
			Error:    err,
		}, err
	}

	startTime := time.Now()

	// Determine timeout - use command-specific timeout if set, otherwise use default
	timeout := e.defaultTimeout
	if cmd.Timeout > 0 {
		timeout = cmd.Timeout
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Parse the command and arguments
	cmdParts, err := e.parseCommand(cmd.Generated)
	if err != nil {
		return &types.ExecutionResult{
			Command:  cmd,
			ExitCode: -1,
			Success:  false,
			Duration: time.Since(startTime),
			Error:    err,
		}, err
	}

	if len(cmdParts) == 0 {
		return &types.ExecutionResult{
			Command:  cmd,
			ExitCode: -1,
			Success:  false,
			Duration: time.Since(startTime),
			Error: &types.NLShellError{
				Type:    types.ErrTypeExecution,
				Message: "empty command",
			},
		}, nil
	}

	// Create the exec.Cmd
	execCmd := exec.CommandContext(execCtx, cmdParts[0], cmdParts[1:]...)

	// Set working directory
	workingDir := cmd.WorkingDir
	if workingDir == "" {
		workingDir = e.workingDir
	}
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		}
	}
	execCmd.Dir = workingDir

	// Set environment variables
	if len(cmd.Environment) > 0 {
		env := os.Environ()
		for key, value := range cmd.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		execCmd.Env = env
	}

	// Execute the command
	stdout, stderr, exitCode, err := e.runCommand(execCmd)
	duration := time.Since(startTime)

	result := &types.ExecutionResult{
		Command:  cmd,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
		Success:  exitCode == 0 && err == nil,
		Error:    err,
	}

	return result, nil
}

// DryRun analyzes the command without executing it
func (e *Executor) DryRun(cmd *types.Command) (*types.DryRunResult, error) {
	if cmd == nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "command cannot be nil",
		}
	}

	// Parse the command to validate syntax
	cmdParts, err := e.parseCommand(cmd.Generated)
	if err != nil {
		return &types.DryRunResult{
			Command:  cmd,
			Analysis: fmt.Sprintf("Command parsing failed: %v", err),
			Predictions: []string{
				"Command will fail due to syntax error",
			},
		}, nil
	}

	if len(cmdParts) == 0 {
		return &types.DryRunResult{
			Command:  cmd,
			Analysis: "Empty command detected",
			Predictions: []string{
				"Command will fail - no executable specified",
			},
		}, nil
	}

	// Perform comprehensive command analysis
	analysis := e.analyzeCommand(cmdParts)
	predictions := e.generatePredictions(cmdParts, cmd)

	// Validate command structure and arguments
	validationResults := e.validateCommandStructure(cmdParts, cmd)

	// Combine analysis with validation results
	fullAnalysis := fmt.Sprintf("%s\n\nValidation Results:\n%s", analysis, validationResults)

	return &types.DryRunResult{
		Command:     cmd,
		Analysis:    fullAnalysis,
		Predictions: predictions,
	}, nil
}

// parseCommand parses a shell command string into command and arguments
func (e *Executor) parseCommand(cmdStr string) ([]string, error) {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "empty command string",
		}
	}

	// Simple shell parsing - split by spaces but respect quotes
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(cmdStr); i++ {
		char := cmdStr[i]

		switch char {
		case '"', '\'':
			if !inQuotes {
				inQuotes = true
				quoteChar = char
			} else if char == quoteChar {
				inQuotes = false
				quoteChar = 0
			} else {
				current.WriteByte(char)
			}
		case ' ', '\t':
			if inQuotes {
				current.WriteByte(char)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if inQuotes {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "unclosed quote in command",
		}
	}

	return parts, nil
}

// runCommand executes the command and captures output
func (e *Executor) runCommand(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	// Capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", -1, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "failed to create stdout pipe",
			Cause:   err,
		}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "failed to create stderr pipe",
			Cause:   err,
		}
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", "", -1, &types.NLShellError{
			Type:    types.ErrTypeExecution,
			Message: "failed to start command",
			Cause:   err,
		}
	}

	// Read output concurrently
	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		buf := make([]byte, 4096)
		var output strings.Builder
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		stdoutChan <- output.String()
	}()

	go func() {
		buf := make([]byte, 4096)
		var output strings.Builder
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		stderrChan <- output.String()
	}()

	// Wait for command to complete
	err = cmd.Wait()

	// Get the outputs
	stdout = <-stdoutChan
	stderr = <-stderrChan

	// Determine exit code
	exitCode = 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
		} else {
			// Command failed to start or other error
			return stdout, stderr, -1, &types.NLShellError{
				Type:    types.ErrTypeExecution,
				Message: "command execution failed",
				Cause:   err,
			}
		}
	}

	return stdout, stderr, exitCode, nil
}

// analyzeCommand provides analysis of what the command will do
func (e *Executor) analyzeCommand(cmdParts []string) string {
	if len(cmdParts) == 0 {
		return "No command to analyze"
	}

	executable := cmdParts[0]
	args := cmdParts[1:]

	var analysis strings.Builder
	analysis.WriteString(fmt.Sprintf("Command: %s\n", executable))

	if len(args) > 0 {
		analysis.WriteString(fmt.Sprintf("Arguments: %s\n", strings.Join(args, " ")))
	}

	// Basic command analysis
	switch executable {
	case "ls":
		analysis.WriteString("Will list directory contents")
	case "cd":
		analysis.WriteString("Will change current directory")
	case "mkdir":
		analysis.WriteString("Will create directories")
	case "rm":
		analysis.WriteString("Will remove files/directories")
	case "cp":
		analysis.WriteString("Will copy files/directories")
	case "mv":
		analysis.WriteString("Will move/rename files/directories")
	case "cat":
		analysis.WriteString("Will display file contents")
	case "grep":
		analysis.WriteString("Will search for patterns in text")
	case "find":
		analysis.WriteString("Will search for files/directories")
	case "ps":
		analysis.WriteString("Will list running processes")
	case "kill":
		analysis.WriteString("Will terminate processes")
	default:
		analysis.WriteString("Will execute the specified command")
	}

	return analysis.String()
}

// generatePredictions generates predictions about command execution
func (e *Executor) generatePredictions(cmdParts []string, cmd *types.Command) []string {
	var predictions []string

	if len(cmdParts) == 0 {
		return []string{"Command will fail - no executable specified"}
	}

	executable := cmdParts[0]

	// Check if executable exists in PATH
	if _, err := exec.LookPath(executable); err != nil {
		predictions = append(predictions, fmt.Sprintf("Command may fail - '%s' not found in PATH", executable))
	} else {
		predictions = append(predictions, fmt.Sprintf("Executable '%s' found in PATH", executable))
	}

	// Working directory predictions
	workingDir := cmd.WorkingDir
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		}
	}
	if workingDir != "" {
		if _, err := os.Stat(workingDir); err != nil {
			predictions = append(predictions, fmt.Sprintf("Working directory '%s' may not exist", workingDir))
		} else {
			predictions = append(predictions, fmt.Sprintf("Will execute in directory: %s", workingDir))
		}
	}

	// Timeout predictions
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = e.defaultTimeout
	}
	predictions = append(predictions, fmt.Sprintf("Command will timeout after %v if not completed", timeout))

	// Command-specific predictions
	switch executable {
	case "rm":
		predictions = append(predictions, "WARNING: This command will permanently delete files")
	case "dd":
		predictions = append(predictions, "WARNING: This command can overwrite data")
	case "sudo":
		predictions = append(predictions, "Command will require elevated privileges")
	case "ssh":
		predictions = append(predictions, "Command will establish remote connection")
	}

	return predictions
}

// validateCommandStructure performs detailed validation of command structure
func (e *Executor) validateCommandStructure(cmdParts []string, cmd *types.Command) string {
	var validation strings.Builder

	if len(cmdParts) == 0 {
		validation.WriteString("✗ No command specified\n")
		return validation.String()
	}

	executable := cmdParts[0]
	args := cmdParts[1:]

	// Check executable existence
	if _, err := exec.LookPath(executable); err != nil {
		validation.WriteString(fmt.Sprintf("✗ Executable '%s' not found in PATH\n", executable))
	} else {
		validation.WriteString(fmt.Sprintf("✓ Executable '%s' found in PATH\n", executable))
	}

	// Validate arguments based on common command patterns
	validation.WriteString(e.validateCommandArguments(executable, args))

	// Check working directory
	workingDir := cmd.WorkingDir
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		}
	}
	if workingDir != "" {
		if _, err := os.Stat(workingDir); err != nil {
			validation.WriteString(fmt.Sprintf("✗ Working directory '%s' does not exist\n", workingDir))
		} else {
			validation.WriteString(fmt.Sprintf("✓ Working directory '%s' exists\n", workingDir))
		}
	}

	// Check environment variables
	if len(cmd.Environment) > 0 {
		validation.WriteString(fmt.Sprintf("✓ %d environment variables will be set\n", len(cmd.Environment)))
		for key, value := range cmd.Environment {
			validation.WriteString(fmt.Sprintf("  - %s=%s\n", key, value))
		}
	}

	// Check timeout
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = e.defaultTimeout
	}
	validation.WriteString(fmt.Sprintf("✓ Timeout set to %v\n", timeout))

	return validation.String()
}

// validateCommandArguments validates arguments for specific commands
func (e *Executor) validateCommandArguments(executable string, args []string) string {
	var validation strings.Builder

	switch executable {
	case "rm":
		if len(args) == 0 {
			validation.WriteString("✗ rm command requires at least one argument\n")
		} else {
			validation.WriteString(fmt.Sprintf("✓ rm will operate on %d target(s)\n", len(args)))
			// Check for dangerous flags
			for _, arg := range args {
				if arg == "-rf" || arg == "-fr" {
					validation.WriteString("⚠ WARNING: Using recursive force delete (-rf)\n")
				}
			}
		}
	case "cp":
		if len(args) < 2 {
			validation.WriteString("✗ cp command requires source and destination\n")
		} else {
			validation.WriteString("✓ cp has source and destination specified\n")
		}
	case "mv":
		if len(args) < 2 {
			validation.WriteString("✗ mv command requires source and destination\n")
		} else {
			validation.WriteString("✓ mv has source and destination specified\n")
		}
	case "mkdir":
		if len(args) == 0 {
			validation.WriteString("✗ mkdir command requires at least one directory name\n")
		} else {
			validation.WriteString(fmt.Sprintf("✓ mkdir will create %d director(ies)\n", len(args)))
		}
	case "cd":
		if len(args) == 0 {
			validation.WriteString("✓ cd will change to home directory\n")
		} else if len(args) == 1 {
			validation.WriteString(fmt.Sprintf("✓ cd will change to '%s'\n", args[0]))
		} else {
			validation.WriteString("✗ cd command accepts only one argument\n")
		}
	case "ls":
		validation.WriteString("✓ ls command structure is valid\n")
	case "cat":
		if len(args) == 0 {
			validation.WriteString("⚠ cat without arguments will read from stdin\n")
		} else {
			validation.WriteString(fmt.Sprintf("✓ cat will display %d file(s)\n", len(args)))
		}
	case "grep":
		if len(args) < 1 {
			validation.WriteString("✗ grep requires at least a pattern argument\n")
		} else {
			validation.WriteString("✓ grep command structure is valid\n")
		}
	case "find":
		validation.WriteString("✓ find command structure appears valid\n")
	case "ps":
		validation.WriteString("✓ ps command structure is valid\n")
	case "kill":
		if len(args) == 0 {
			validation.WriteString("✗ kill command requires at least one process ID\n")
		} else {
			validation.WriteString(fmt.Sprintf("✓ kill will target %d process(es)\n", len(args)))
		}
	case "sudo":
		if len(args) == 0 {
			validation.WriteString("✗ sudo requires a command to execute\n")
		} else {
			validation.WriteString("✓ sudo command structure is valid\n")
			validation.WriteString("⚠ WARNING: Command will run with elevated privileges\n")
		}
	default:
		validation.WriteString(fmt.Sprintf("✓ Command '%s' structure appears valid\n", executable))
	}

	return validation.String()
}
