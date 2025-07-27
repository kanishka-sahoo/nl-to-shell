package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/config"
	contextpkg "github.com/nl-to-shell/nl-to-shell/internal/context"
	"github.com/nl-to-shell/nl-to-shell/internal/executor"
	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/manager"
	"github.com/nl-to-shell/nl-to-shell/internal/safety"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
	"github.com/nl-to-shell/nl-to-shell/internal/validator"
)

// SessionState holds the state for an interactive session
type SessionState struct {
	manager         interfaces.CommandManager
	config          *types.Config
	contextGatherer interfaces.ContextGatherer
	sessionID       string
	startTime       time.Time
	commandHistory  []string
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

	return &SessionState{
		manager:         commandManager,
		config:          cfg,
		contextGatherer: contextGatherer,
		sessionID:       sessionID,
		startTime:       time.Now(),
		commandHistory:  make([]string, 0),
	}, nil
}

// RunInteractiveSession starts and runs an interactive session
func RunInteractiveSession() error {
	session, err := NewSessionState()
	if err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	fmt.Println("🚀 Welcome to nl-to-shell interactive session!")
	fmt.Printf("Session ID: %s\n", session.sessionID)
	fmt.Println("Type your natural language commands, or use these special commands:")
	fmt.Println("  help    - Show this help message")
	fmt.Println("  history - Show command history")
	fmt.Println("  clear   - Clear command history")
	fmt.Println("  config  - Show current configuration")
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

		// Handle special commands
		if handled, shouldExit := session.handleSpecialCommand(input); handled {
			if shouldExit {
				break
			}
			continue
		}

		// Process natural language command
		err := session.processCommand(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		// Add to history
		session.commandHistory = append(session.commandHistory, input)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	fmt.Println("\n👋 Session ended. Goodbye!")
	return nil
}

// handleSpecialCommand handles special session commands
func (s *SessionState) handleSpecialCommand(input string) (handled bool, shouldExit bool) {
	switch strings.ToLower(input) {
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
	case "exit", "quit", "q":
		return true, true
	default:
		return false, false
	}
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

// GetSessionState returns the current session state (for testing)
func (s *SessionState) GetSessionState() (string, time.Time, []string) {
	return s.sessionID, s.startTime, s.commandHistory
}
