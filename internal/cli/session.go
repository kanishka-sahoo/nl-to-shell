package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/config"
	contextpkg "github.com/kanishka-sahoo/nl-to-shell/internal/context"
	"github.com/kanishka-sahoo/nl-to-shell/internal/errors"
	"github.com/kanishka-sahoo/nl-to-shell/internal/executor"
	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/manager"
	"github.com/kanishka-sahoo/nl-to-shell/internal/performance"
	"github.com/kanishka-sahoo/nl-to-shell/internal/safety"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
	"github.com/kanishka-sahoo/nl-to-shell/internal/validator"
)

// SessionState holds the state for an interactive session with monitoring
type SessionState struct {
	manager         interfaces.CommandManager
	config          *types.Config
	contextGatherer interfaces.ContextGatherer
	sessionID       string
	startTime       time.Time
	commandHistory  []string
	monitor         *performance.Monitor
	logger          errors.Logger
	commandCount    int
	successCount    int
	errorCount      int
}

// NewSessionState creates a new session state
func NewSessionState() (*SessionState, error) {
	// Load configuration
	configManager := config.NewManager()
	cfg, err := configManager.Load()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
		}
		// Use default configuration
		cfg = &types.Config{
			DefaultProvider: "openai",
			Providers:       make(map[string]types.ProviderConfig),
			UserPreferences: types.UserPreferences{
				DefaultTimeout:  30 * time.Second,
				MaxFileListSize: 100,
				EnablePlugins:   true,
			},
		}
	}

	// Override configuration with CLI flags
	if provider != "" {
		cfg.DefaultProvider = provider
	}

	// Create components
	contextGatherer := contextpkg.NewGatherer()
	safetyValidator := safety.NewValidator()
	commandExecutor := executor.NewExecutor()

	// Create LLM provider
	llmProvider, err := createLLMProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	resultValidator := validator.NewResultValidator(llmProvider)

	// Create command manager
	commandManager := manager.NewManager(
		contextGatherer,
		llmProvider,
		safetyValidator,
		commandExecutor,
		resultValidator,
		cfg,
	)

	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	// Initialize session-specific monitoring
	sessionMonitor := performance.NewMonitor(&performance.MonitorConfig{
		Enabled:              true,
		MaxMetrics:           1000,
		CollectionInterval:   0, // No automatic collection for sessions
		EnableMemoryStats:    false,
		EnableGoroutineStats: false,
	})

	// Initialize session logger
	sessionLogger := errors.NewStructuredLogger(false)

	session := &SessionState{
		manager:         commandManager,
		config:          cfg,
		contextGatherer: contextGatherer,
		sessionID:       sessionID,
		startTime:       time.Now(),
		commandHistory:  make([]string, 0),
		monitor:         sessionMonitor,
		logger:          sessionLogger,
		commandCount:    0,
		successCount:    0,
		errorCount:      0,
	}

	// Record session start
	session.monitor.RecordCounter("session.started", 1, map[string]string{
		"session_id": sessionID,
		"provider":   cfg.DefaultProvider,
	})

	return session, nil
}

// RunInteractiveSession starts and runs an interactive session with comprehensive monitoring
func RunInteractiveSession() error {
	session, err := NewSessionState()
	if err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	// Start session timer
	sessionTimer := session.monitor.StartTimer("session.total_duration", map[string]string{
		"session_id": session.sessionID,
	})
	defer sessionTimer.Stop()

	fmt.Println("🚀 Welcome to nl-to-shell interactive session!")
	fmt.Printf("Session ID: %s\n", session.sessionID)
	fmt.Printf("Started at: %s\n", session.startTime.Format(time.RFC3339))
	fmt.Println()
	fmt.Println("Type your natural language commands, or use these special commands:")
	fmt.Println("  help    - Show this help message")
	fmt.Println("  history - Show command history")
	fmt.Println("  clear   - Clear command history")
	fmt.Println("  config  - Show current configuration")
	fmt.Println("  stats   - Show session statistics")
	fmt.Println("  exit    - Exit the session")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("nl-to-shell> ")

		if !scanner.Scan() {
			// EOF or error
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Record input received
		session.monitor.RecordCounter("session.inputs_received", 1, map[string]string{
			"session_id": session.sessionID,
		})

		// Handle special commands
		if handled, shouldExit := session.handleSpecialCommand(input); handled {
			if shouldExit {
				break
			}
			continue
		}

		// Process natural language command with timing
		commandTimer := session.monitor.StartTimer("session.command_processing", map[string]string{
			"session_id": session.sessionID,
		})

		err := session.processCommand(input)
		commandTimer.Stop()

		session.commandCount++

		if err != nil {
			session.errorCount++

			// Log the error
			nlErr, ok := err.(*types.NLShellError)
			if !ok {
				nlErr = &types.NLShellError{
					Type:      types.ErrTypeValidation,
					Message:   "session command processing failed",
					Cause:     err,
					Severity:  types.SeverityError,
					Timestamp: time.Now(),
					Context: map[string]interface{}{
						"session_id":    session.sessionID,
						"input":         input,
						"command_count": session.commandCount,
					},
				}
			}
			session.logger.LogError(nlErr)

			session.monitor.RecordCounter("session.command_errors", 1, map[string]string{
				"session_id": session.sessionID,
				"error_type": nlErr.Type.String(),
			})

			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			session.successCount++
			session.monitor.RecordCounter("session.command_success", 1, map[string]string{
				"session_id": session.sessionID,
			})
		}

		// Add to history
		session.commandHistory = append(session.commandHistory, input)
	}

	if err := scanner.Err(); err != nil {
		session.monitor.RecordCounter("session.scanner_errors", 1, map[string]string{
			"session_id": session.sessionID,
		})
		return fmt.Errorf("error reading input: %w", err)
	}

	// Record session end metrics
	session.recordSessionEndMetrics()

	fmt.Println("\n📊 Session Summary")
	fmt.Println("==================")
	fmt.Printf("Duration: %v\n", time.Since(session.startTime).Round(time.Second))
	fmt.Printf("Commands processed: %d\n", session.commandCount)
	fmt.Printf("Successful: %d\n", session.successCount)
	fmt.Printf("Errors: %d\n", session.errorCount)
	if session.commandCount > 0 {
		fmt.Printf("Success rate: %.1f%%\n", float64(session.successCount)/float64(session.commandCount)*100)
	}

	fmt.Println("\n👋 Session ended. Goodbye!")
	return nil
}

// handleSpecialCommand handles special session commands with monitoring
func (s *SessionState) handleSpecialCommand(input string) (handled bool, shouldExit bool) {
	command := strings.ToLower(input)

	// Record metrics if monitor is available
	if s.monitor != nil {
		s.monitor.RecordCounter("session.special_commands", 1, map[string]string{
			"session_id": s.sessionID,
			"command":    command,
		})
	}

	switch command {
	case "help":
		s.showHelp()
		return true, false
	case "history":
		s.showHistory()
		return true, false
	case "clear":
		s.clearHistory()
		return true, false
	case "config":
		s.showConfig()
		return true, false
	case "stats":
		s.showStats()
		return true, false
	case "exit", "quit", "q":
		return true, true
	default:
		return false, false
	}
}

// recordSessionEndMetrics records metrics when the session ends
func (s *SessionState) recordSessionEndMetrics() {
	if s.monitor == nil {
		return
	}

	duration := time.Since(s.startTime)

	s.monitor.RecordDuration("session.final_duration", duration, map[string]string{
		"session_id": s.sessionID,
	})

	s.monitor.RecordGauge("session.final_command_count", float64(s.commandCount), "count", map[string]string{
		"session_id": s.sessionID,
	})

	s.monitor.RecordGauge("session.final_success_count", float64(s.successCount), "count", map[string]string{
		"session_id": s.sessionID,
	})

	s.monitor.RecordGauge("session.final_error_count", float64(s.errorCount), "count", map[string]string{
		"session_id": s.sessionID,
	})

	if s.commandCount > 0 {
		successRate := float64(s.successCount) / float64(s.commandCount) * 100
		s.monitor.RecordGauge("session.final_success_rate", successRate, "percent", map[string]string{
			"session_id": s.sessionID,
		})
	}

	s.monitor.RecordCounter("session.ended", 1, map[string]string{
		"session_id": s.sessionID,
	})
}

// processCommand processes a natural language command
func (s *SessionState) processCommand(input string) error {
	ctx := context.Background()

	// Step 1: Generate command
	commandResult, err := s.manager.GenerateCommand(ctx, input)
	if err != nil {
		return fmt.Errorf("command generation failed: %w", err)
	}

	// Create full result structure
	fullResult := &types.FullResult{
		CommandResult: commandResult,
	}

	// Step 2: Handle dry run mode
	if dryRun {
		// For session mode, we'll use the executor directly for dry run
		// This is a simplified approach for the session
		fmt.Printf("Generated command: %s\n", commandResult.Command.Generated)
		fmt.Println("(Dry run mode - command not executed)")
		return nil
	}

	// Step 3: Check safety requirements
	if commandResult.Safety.RequiresConfirmation && !skipConfirmation {
		fmt.Printf("⚠️  This command requires confirmation: %s\n", commandResult.Command.Generated)
		fmt.Printf("Safety level: %s\n", commandResult.Safety.DangerLevel.String())
		if len(commandResult.Safety.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, warning := range commandResult.Safety.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
		fmt.Print("Do you want to proceed? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Command cancelled.")
			return nil
		}

		// Mark as validated after user confirmation
		commandResult.Command.Validated = true
	}

	// Step 4: Execute command if validated
	if commandResult.Command.Validated || commandResult.Safety.IsSafe {
		executionResult, err := s.manager.ExecuteCommand(ctx, commandResult.Command)
		if err != nil {
			return fmt.Errorf("command execution failed: %w", err)
		}
		fullResult.ExecutionResult = executionResult

		// Step 5: Validate results if requested
		if validateResults {
			validationResult, err := s.manager.ValidateResult(ctx, executionResult, input)
			if err != nil {
				// Don't fail the entire operation if validation fails
				fmt.Printf("Warning: Result validation failed: %v\n", err)
			} else {
				fullResult.ValidationResult = validationResult
			}
		}
	}

	// Display results
	return displayResults(fullResult, input)
}

// showHelp displays help information
func (s *SessionState) showHelp() {
	fmt.Println("\n📖 nl-to-shell Interactive Session Help")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Natural Language Commands:")
	fmt.Println("  Just type what you want to do in natural language!")
	fmt.Println("  Examples:")
	fmt.Println("    list files by size")
	fmt.Println("    find all .txt files")
	fmt.Println("    show disk usage")
	fmt.Println("    create a new directory called 'test'")
	fmt.Println()
	fmt.Println("Special Commands:")
	fmt.Println("  help    - Show this help message")
	fmt.Println("  history - Show your command history")
	fmt.Println("  clear   - Clear command history")
	fmt.Println("  config  - Show current configuration")
	fmt.Println("  exit    - Exit the session")
	fmt.Println()
	fmt.Println("Global Flags (set when starting session):")
	fmt.Printf("  --dry-run: %v\n", dryRun)
	fmt.Printf("  --verbose: %v\n", verbose)
	fmt.Printf("  --provider: %s\n", provider)
	fmt.Printf("  --model: %s\n", model)
	fmt.Printf("  --skip-confirmation: %v\n", skipConfirmation)
	fmt.Printf("  --validate-results: %v\n", validateResults)
	fmt.Println()
}

// showHistory displays the command history
func (s *SessionState) showHistory() {
	fmt.Println("\n📜 Command History")
	fmt.Println("==================")

	if len(s.commandHistory) == 0 {
		fmt.Println("No commands in history yet.")
		return
	}

	for i, cmd := range s.commandHistory {
		fmt.Printf("%3d. %s\n", i+1, cmd)
	}
	fmt.Printf("\nTotal commands: %d\n", len(s.commandHistory))
	fmt.Println()
}

// clearHistory clears the command history
func (s *SessionState) clearHistory() {
	s.commandHistory = make([]string, 0)
	fmt.Println("✅ Command history cleared.")
}

// showConfig displays the current configuration
func (s *SessionState) showConfig() {
	fmt.Println("\n⚙️  Current Configuration")
	fmt.Println("========================")
	fmt.Printf("Session ID: %s\n", s.sessionID)
	fmt.Printf("Session Duration: %v\n", time.Since(s.startTime).Round(time.Second))
	fmt.Printf("Default Provider: %s\n", s.config.DefaultProvider)

	if verbose {
		fmt.Printf("Max File List Size: %d\n", s.config.UserPreferences.MaxFileListSize)
		fmt.Printf("Default Timeout: %v\n", s.config.UserPreferences.DefaultTimeout)
		fmt.Printf("Plugins Enabled: %v\n", s.config.UserPreferences.EnablePlugins)
		fmt.Printf("Auto Update: %v\n", s.config.UserPreferences.AutoUpdate)
	}

	fmt.Println("\nCurrent Flags:")
	fmt.Printf("  Dry Run: %v\n", dryRun)
	fmt.Printf("  Verbose: %v\n", verbose)
	fmt.Printf("  Skip Confirmation: %v\n", skipConfirmation)
	fmt.Printf("  Validate Results: %v\n", validateResults)

	if provider != "" {
		fmt.Printf("  Provider Override: %s\n", provider)
	}
	if model != "" {
		fmt.Printf("  Model Override: %s\n", model)
	}
	fmt.Println()
}

// showStats displays session statistics
func (s *SessionState) showStats() {
	fmt.Println("\n📊 Session Statistics")
	fmt.Println("====================")
	fmt.Printf("Session ID: %s\n", s.sessionID)
	fmt.Printf("Started: %s\n", s.startTime.Format(time.RFC3339))
	fmt.Printf("Duration: %v\n", time.Since(s.startTime).Round(time.Second))
	fmt.Printf("Commands processed: %d\n", s.commandCount)
	fmt.Printf("Successful commands: %d\n", s.successCount)
	fmt.Printf("Failed commands: %d\n", s.errorCount)

	if s.commandCount > 0 {
		successRate := float64(s.successCount) / float64(s.commandCount) * 100
		fmt.Printf("Success rate: %.1f%%\n", successRate)

		avgTime := time.Since(s.startTime) / time.Duration(s.commandCount)
		fmt.Printf("Average time per command: %v\n", avgTime.Round(time.Millisecond))
	}

	fmt.Printf("Commands in history: %d\n", len(s.commandHistory))

	// Show recent metrics if available
	if s.monitor != nil {
		recentMetrics := s.monitor.GetMetricsSince(time.Now().Add(-5 * time.Minute))
		if len(recentMetrics) > 0 {
			fmt.Printf("Recent metrics collected: %d\n", len(recentMetrics))
		}
	}

	fmt.Println()
}

// GetSessionState returns the current session state (for testing)
func (s *SessionState) GetSessionState() (string, time.Time, []string) {
	return s.sessionID, s.startTime, s.commandHistory
}
