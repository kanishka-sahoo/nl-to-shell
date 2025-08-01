package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kanishka-sahoo/nl-to-shell/internal/config"
	contextpkg "github.com/kanishka-sahoo/nl-to-shell/internal/context"
	"github.com/kanishka-sahoo/nl-to-shell/internal/errors"
	"github.com/kanishka-sahoo/nl-to-shell/internal/executor"
	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/llm"
	"github.com/kanishka-sahoo/nl-to-shell/internal/manager"
	"github.com/kanishka-sahoo/nl-to-shell/internal/performance"
	"github.com/kanishka-sahoo/nl-to-shell/internal/safety"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
	"github.com/kanishka-sahoo/nl-to-shell/internal/updater"
	"github.com/kanishka-sahoo/nl-to-shell/internal/validator"
)

var (
	// Global flags
	dryRun           bool
	verbose          bool
	provider         string
	model            string
	skipConfirmation bool
	validateResults  bool
	sessionMode      bool

	// Global infrastructure
	globalMonitor *performance.Monitor
	globalLogger  errors.Logger
)

// Version information - will be set during build
var (
	Version   = "0.1.0-dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nl-to-shell",
	Short: "Convert natural language to shell commands",
	Long: `Convert natural language to shell commands using Large Language Models (LLMs).

Provides context-aware command generation by analyzing your current working 
directory, git repository state, files, and other environmental factors.

Emphasizes safety through dangerous command detection, user confirmation 
prompts, and validation systems while supporting multiple AI providers and 
cross-platform compatibility.`,
	Example: `  # Generate a command from natural language
  nl-to-shell "list files by size in descending order"
  
  # Use dry run mode to preview the command
  nl-to-shell --dry-run "delete all .tmp files"
  
  # Use a specific provider and model
  nl-to-shell --provider openai --model gpt-4 "find large files"
  
  # Skip confirmation for dangerous commands (advanced users)
  nl-to-shell --skip-confirmation "remove temporary files"
  
  # Disable result validation for faster execution
  nl-to-shell --validate-results=false "list current directory"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Check if the argument looks like a subcommand that doesn't exist
		arg := args[0]
		if !strings.Contains(arg, " ") && len(arg) > 0 && arg[0] != '-' {
			// Check if it's a known subcommand
			isKnownSubcommand := false
			for _, subCmd := range cmd.Commands() {
				if subCmd.Name() == arg || subCmd.HasAlias(arg) {
					isKnownSubcommand = true
					break
				}
			}

			// If it looks like a command but isn't known, return unknown command error
			if !isKnownSubcommand && !strings.Contains(arg, " ") {
				return fmt.Errorf("unknown command \"%s\" for \"%s\"", arg, cmd.CommandPath())
			}
		}

		return executeCommandGeneration(args[0])
	},
	SilenceUsage: true, // Don't show usage on errors
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// ExecuteWithContext executes the root command with the provided context
func ExecuteWithContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(nl-to-shell completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ nl-to-shell completion bash > /etc/bash_completion.d/nl-to-shell
  # macOS:
  $ nl-to-shell completion bash > /usr/local/etc/bash_completion.d/nl-to-shell

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ nl-to-shell completion zsh > "${fpath[1]}/_nl-to-shell"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ nl-to-shell completion fish | source

  # To load completions for each session, execute once:
  $ nl-to-shell completion fish > ~/.config/fish/completions/nl-to-shell.fish

PowerShell:

  PS> nl-to-shell completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> nl-to-shell completion powershell > nl-to-shell.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	// Initialize global infrastructure
	initializeGlobalInfrastructure()

	// Global flags that apply to all commands
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview the command without executing it")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output for detailed information")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "", "LLM provider to use (openai, anthropic, gemini, openrouter, ollama)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to use for the specified provider")
	rootCmd.PersistentFlags().BoolVar(&skipConfirmation, "skip-confirmation", false, "Skip confirmation prompts for dangerous commands (advanced users)")
	rootCmd.PersistentFlags().BoolVar(&validateResults, "validate-results", true, "Validate command results using AI")

	// Add subcommands
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(helpCmd)
	rootCmd.AddCommand(completionCmd)
}

// initializeGlobalInfrastructure sets up monitoring and logging
func initializeGlobalInfrastructure() {
	// Initialize performance monitoring
	monitorConfig := &performance.MonitorConfig{
		Enabled:              true,
		MaxMetrics:           5000,
		CollectionInterval:   60 * time.Second,
		EnableMemoryStats:    true,
		EnableGoroutineStats: true,
	}
	globalMonitor = performance.NewMonitor(monitorConfig)

	// Initialize structured logging
	globalLogger = errors.NewStructuredLogger(false)
	errors.SetGlobalLogger(globalLogger)
}

// generateCmd represents the generate command (explicit command for generation)
var generateCmd = &cobra.Command{
	Use:   "generate [natural language description]",
	Short: "Generate shell command from natural language",
	Long: `Generate a shell command from a natural language description.
This is the main functionality of nl-to-shell.`,
	Example: `  # Generate a command from natural language
  nl-to-shell generate "list files by size in descending order"
  
  # Use with flags
  nl-to-shell generate --dry-run "delete all .tmp files"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeCommandGeneration(args[0])
	},
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Long: `Manage configuration settings for nl-to-shell.
Use subcommands to set up providers, view current configuration, or reset settings.`,
}

// setupCmd represents the config setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration setup",
	Long: `Interactive configuration setup for nl-to-shell.
This will guide you through setting up API keys and preferences.`,
	RunE: executeConfigSetup,
}

// showCmd represents the config show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Show current configuration settings (credentials will be masked).`,
	RunE:  executeConfigShow,
}

// resetCmd represents the config reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  `Reset configuration to defaults.`,
	RunE:  executeConfigReset,
}

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Start an interactive session",
	Long: `Start an interactive session for continuous command generation 
without restarting the tool. This maintains context and configuration 
between multiple command generations.

In session mode, you can:
- Execute multiple commands without restarting
- Use special commands like 'help', 'history', 'config'
- Maintain session state and context
- Exit cleanly with 'exit' or Ctrl+C`,
	Example: `  # Start an interactive session
  nl-to-shell session
  
  # Start session with specific provider
  nl-to-shell session --provider openai
  
  # Start session in dry-run mode
  nl-to-shell session --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInteractiveSession()
	},
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Manage updates",
	Long: `Manage updates for nl-to-shell.
Use subcommands to check for updates or install them.`,
}

// checkCmd represents the update check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for available updates",
	Long:  `Check for available updates without installing them.`,
	RunE:  executeUpdateCheck,
}

// installCmd represents the update install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install available updates",
	Long:  `Install available updates.`,
	RunE:  executeUpdateInstall,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the current version of nl-to-shell along with build information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("nl-to-shell version %s\n", Version)
		if verbose {
			fmt.Printf("Git commit: %s\n", GitCommit)
			fmt.Printf("Build date: %s\n", BuildDate)
		}
	},
}

// helpCmd represents the help command
var helpCmd = &cobra.Command{
	Use:   "help [topic]",
	Short: "Show comprehensive help information",
	Long: `Display detailed help information about nl-to-shell usage, features, and configuration.
Use without arguments to see an overview, or specify a topic for detailed information.`,
	Example: `  # Show general help overview
  nl-to-shell help
  
  # Get help on specific topics
  nl-to-shell help getting-started
  nl-to-shell help providers
  nl-to-shell help safety`,
	Args: cobra.MaximumNArgs(1),
	RunE: executeHelp,
}

func init() {
	// Add subcommands to config
	configCmd.AddCommand(setupCmd)
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(resetCmd)

	// Add subcommands to update
	updateCmd.AddCommand(checkCmd)
	updateCmd.AddCommand(installCmd)

	// Add flags to update commands
	checkCmd.Flags().Bool("prerelease", false, "Include prerelease versions in update check")
	installCmd.Flags().Bool("prerelease", false, "Allow installation of prerelease versions")
	installCmd.Flags().Bool("no-backup", false, "Skip creating backup before update")
}

// GetGlobalFlags returns the current global flag values
func GetGlobalFlags() GlobalFlags {
	return GlobalFlags{
		DryRun:           dryRun,
		Verbose:          verbose,
		Provider:         provider,
		Model:            model,
		SkipConfirmation: skipConfirmation,
		ValidateResults:  validateResults,
		SessionMode:      sessionMode,
	}
}

// GlobalFlags represents the global CLI flags
type GlobalFlags struct {
	DryRun           bool
	Verbose          bool
	Provider         string
	Model            string
	SkipConfirmation bool
	ValidateResults  bool
	SessionMode      bool
}

// executeCommandGeneration handles the main command generation flow with comprehensive monitoring
func executeCommandGeneration(input string) error {
	// Start overall timing
	timer := globalMonitor.StartTimer("command_generation.total_time", map[string]string{
		"provider": provider,
		"dry_run":  fmt.Sprintf("%v", dryRun),
	})
	defer timer.Stop()

	// Record command generation attempt
	globalMonitor.RecordCounter("command_generation.attempts", 1, map[string]string{
		"provider": provider,
		"dry_run":  fmt.Sprintf("%v", dryRun),
	})

	ctx := context.Background()

	// Load configuration with monitoring
	configTimer := globalMonitor.StartTimer("command_generation.config_load", nil)
	configManager := config.NewManager()
	cfg, err := configManager.Load()
	configTimer.Stop()

	if err != nil {
		// Log configuration load error
		nlErr := &types.NLShellError{
			Type:      types.ErrTypeConfiguration,
			Message:   "failed to load configuration",
			Cause:     err,
			Severity:  types.SeverityWarning,
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"input": input,
			},
		}
		globalLogger.LogError(nlErr)

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

		globalMonitor.RecordCounter("command_generation.config_load_failures", 1, nil)
	} else {
		globalMonitor.RecordCounter("command_generation.config_load_success", 1, nil)
	}

	// Override configuration with CLI flags
	if provider != "" {
		cfg.DefaultProvider = provider
	}

	// Create components with monitoring
	componentTimer := globalMonitor.StartTimer("command_generation.component_creation", nil)

	contextGatherer := contextpkg.NewGatherer()
	safetyValidator := safety.NewValidator()
	commandExecutor := executor.NewExecutor()

	// Create LLM provider with error handling
	llmProvider, err := createLLMProvider(cfg)
	if err != nil {
		componentTimer.Stop()

		nlErr := &types.NLShellError{
			Type:      types.ErrTypeProvider,
			Message:   "failed to create LLM provider",
			Cause:     err,
			Severity:  types.SeverityError,
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"input":    input,
				"provider": cfg.DefaultProvider,
			},
		}
		globalLogger.LogError(nlErr)
		globalMonitor.RecordCounter("command_generation.provider_creation_failures", 1, map[string]string{
			"provider": cfg.DefaultProvider,
		})

		return fmt.Errorf("failed to create LLM provider: %w", err)
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

	componentTimer.Stop()
	globalMonitor.RecordCounter("command_generation.component_creation_success", 1, nil)

	// Create execution options from CLI flags
	options := &types.ExecutionOptions{
		DryRun:           dryRun,
		SkipConfirmation: skipConfirmation,
		ValidateResults:  validateResults,
		Timeout:          cfg.UserPreferences.DefaultTimeout,
	}

	// Execute the full pipeline with monitoring
	pipelineTimer := globalMonitor.StartTimer("command_generation.pipeline_execution", map[string]string{
		"provider":          cfg.DefaultProvider,
		"dry_run":           fmt.Sprintf("%v", dryRun),
		"skip_confirmation": fmt.Sprintf("%v", skipConfirmation),
		"validate_results":  fmt.Sprintf("%v", validateResults),
	})

	result, err := commandManager.GenerateAndExecute(ctx, input, options)
	pipelineTimer.Stop()

	if err != nil {
		// Log pipeline execution error
		nlErr, ok := err.(*types.NLShellError)
		if !ok {
			nlErr = &types.NLShellError{
				Type:      types.ErrTypeValidation,
				Message:   "command generation pipeline failed",
				Cause:     err,
				Severity:  types.SeverityError,
				Timestamp: time.Now(),
				Context: map[string]interface{}{
					"input":    input,
					"provider": cfg.DefaultProvider,
					"options":  options,
				},
			}
		}
		globalLogger.LogError(nlErr)
		globalMonitor.RecordCounter("command_generation.pipeline_failures", 1, map[string]string{
			"provider":   cfg.DefaultProvider,
			"error_type": nlErr.Type.String(),
		})

		return fmt.Errorf("command generation failed: %w", err)
	}

	globalMonitor.RecordCounter("command_generation.pipeline_success", 1, map[string]string{
		"provider": cfg.DefaultProvider,
	})

	// Record result metrics
	recordResultMetrics(result, cfg.DefaultProvider)

	// Display results with monitoring
	displayTimer := globalMonitor.StartTimer("command_generation.result_display", nil)
	displayErr := displayResults(result, input)
	displayTimer.Stop()

	if displayErr != nil {
		nlErr := &types.NLShellError{
			Type:      types.ErrTypeValidation,
			Message:   "failed to display results",
			Cause:     displayErr,
			Severity:  types.SeverityWarning,
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"input": input,
			},
		}
		globalLogger.LogError(nlErr)
		globalMonitor.RecordCounter("command_generation.display_failures", 1, nil)
	}

	return displayErr
}

// createLLMProvider creates an LLM provider based on configuration
func createLLMProvider(cfg *types.Config) (interfaces.LLMProvider, error) {
	providerName := cfg.DefaultProvider
	if provider != "" {
		providerName = provider
	}

	// Get provider configuration using config manager (handles env vars and credential storage)
	configManager := config.NewManager()
	providerConfig, err := configManager.GetProviderConfig(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config for %s: %w", providerName, err)
	}

	// Override model if specified via CLI
	if model != "" {
		providerConfig.DefaultModel = model
	}

	// Create provider using factory
	factory := llm.NewProviderFactory()
	return factory.CreateProvider(providerName, providerConfig)
}

// recordResultMetrics records metrics about the command generation results
func recordResultMetrics(result *types.FullResult, provider string) {
	if result == nil {
		return
	}

	tags := map[string]string{
		"provider": provider,
	}

	// Record command result metrics
	if result.CommandResult != nil {
		globalMonitor.RecordGauge("command_generation.confidence", result.CommandResult.Confidence, "score", tags)
		globalMonitor.RecordGauge("command_generation.alternatives_count", float64(len(result.CommandResult.Alternatives)), "count", tags)

		if result.CommandResult.Safety != nil {
			globalMonitor.RecordCounter("command_generation.safety_checks", 1, map[string]string{
				"provider":     provider,
				"danger_level": result.CommandResult.Safety.DangerLevel.String(),
				"is_safe":      fmt.Sprintf("%v", result.CommandResult.Safety.IsSafe),
			})
		}
	}

	// Record execution metrics
	if result.ExecutionResult != nil {
		globalMonitor.RecordCounter("command_generation.executions", 1, map[string]string{
			"provider":  provider,
			"exit_code": fmt.Sprintf("%d", result.ExecutionResult.ExitCode),
			"success":   fmt.Sprintf("%v", result.ExecutionResult.Success),
		})

		globalMonitor.RecordDuration("command_generation.execution_time", result.ExecutionResult.Duration, tags)
	}

	// Record validation metrics
	if result.ValidationResult != nil {
		globalMonitor.RecordCounter("command_generation.validations", 1, map[string]string{
			"provider":   provider,
			"is_correct": fmt.Sprintf("%v", result.ValidationResult.IsCorrect),
		})
	}

	// Record dry run metrics
	if result.DryRunResult != nil {
		globalMonitor.RecordCounter("command_generation.dry_runs", 1, tags)
	}

	// Record confirmation requirements
	if result.RequiresConfirmation {
		globalMonitor.RecordCounter("command_generation.confirmations_required", 1, tags)
	}
}

// displayResults displays the command generation and execution results with enhanced formatting
func displayResults(result *types.FullResult, originalInput string) error {
	if result == nil || result.CommandResult == nil {
		return fmt.Errorf("no results to display")
	}

	// Display generated command (maintain backward compatibility)
	fmt.Printf("Generated command: %s\n", result.CommandResult.Command.Generated)

	// Display confidence and metadata
	if verbose {
		fmt.Printf("Confidence: %.2f\n", result.CommandResult.Confidence)

		// Safely access provider information
		provider := "unknown"
		if result.CommandResult.Command.Context != nil && result.CommandResult.Command.Context.Environment != nil {
			if p, exists := result.CommandResult.Command.Context.Environment["PROVIDER"]; exists {
				provider = p
			}
		}
		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("Generated at: %s\n", result.CommandResult.Command.Timestamp.Format(time.RFC3339))

		if len(result.CommandResult.Alternatives) > 0 {
			fmt.Println("\nAlternatives:")
			for i, alt := range result.CommandResult.Alternatives {
				fmt.Printf("  %d. %s\n", i+1, alt)
			}
		}
	}

	// Display safety information (maintain backward compatibility)
	if result.CommandResult.Safety != nil && result.CommandResult.Safety.DangerLevel > types.Safe {
		fmt.Printf("⚠️  Safety level: %s\n", result.CommandResult.Safety.DangerLevel.String())
		if len(result.CommandResult.Safety.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, warning := range result.CommandResult.Safety.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	}

	// Handle dry run results (maintain backward compatibility)
	if result.DryRunResult != nil {
		fmt.Println("\n--- Dry Run Analysis ---")
		fmt.Printf("Analysis: %s\n", result.DryRunResult.Analysis)
		if len(result.DryRunResult.Predictions) > 0 {
			fmt.Println("Predicted outcomes:")
			for _, prediction := range result.DryRunResult.Predictions {
				fmt.Printf("  - %s\n", prediction)
			}
		}
		return nil
	}

	// Handle confirmation requirement (maintain backward compatibility)
	if result.RequiresConfirmation {
		fmt.Printf("\n⚠️  This command requires confirmation. Use --skip-confirmation to bypass.\n")
		return nil
	}

	// Display execution results (maintain backward compatibility)
	if result.ExecutionResult != nil {
		fmt.Println("\n--- Execution Results ---")
		fmt.Printf("Exit code: %d\n", result.ExecutionResult.ExitCode)
		fmt.Printf("Duration: %v\n", result.ExecutionResult.Duration)

		if result.ExecutionResult.Stdout != "" {
			fmt.Println("Output:")
			fmt.Println(result.ExecutionResult.Stdout)
		}

		if result.ExecutionResult.Stderr != "" {
			fmt.Println("Error output:")
			fmt.Println(result.ExecutionResult.Stderr)
		}

		// Display validation results (maintain backward compatibility)
		if result.ValidationResult != nil {
			fmt.Println("\n--- Validation Results ---")
			if result.ValidationResult.IsCorrect {
				fmt.Println("✅ Command executed successfully and achieved the intended result")
			} else {
				fmt.Println("❌ Command may not have achieved the intended result")
				fmt.Printf("Explanation: %s\n", result.ValidationResult.Explanation)

				if len(result.ValidationResult.Suggestions) > 0 {
					fmt.Println("Suggestions:")
					for _, suggestion := range result.ValidationResult.Suggestions {
						fmt.Printf("  - %s\n", suggestion)
					}
				}

				if result.ValidationResult.CorrectedCommand != "" {
					fmt.Printf("Suggested correction: %s\n", result.ValidationResult.CorrectedCommand)
				}
			}
		}
	}

	return nil
}

// executeUpdateCheck handles the update check command
func executeUpdateCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration to check update settings
	configManager := config.NewManager()
	cfg, err := configManager.Load()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
		}
		// Use default configuration
		cfg = &types.Config{
			UpdateSettings: types.UpdateSettings{
				AllowPrerelease: false,
			},
		}
	}

	// Check for prerelease flag
	prerelease, _ := cmd.Flags().GetBool("prerelease")
	if prerelease {
		cfg.UpdateSettings.AllowPrerelease = true
	}

	// Create update manager
	updateManager := updater.NewManager(Version, "nl-to-shell", "nl-to-shell")

	fmt.Println("Checking for updates...")
	if cfg.UpdateSettings.AllowPrerelease {
		fmt.Println("(including prerelease versions)")
	}

	updateInfo, err := updateManager.CheckForUpdates(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("Current version: %s\n", updateInfo.CurrentVersion)
	fmt.Printf("Latest version: %s\n", updateInfo.LatestVersion)

	if updateInfo.Available {
		fmt.Println("✅ Update available!")
		if updateInfo.ReleaseNotes != "" {
			fmt.Println("\nRelease notes:")
			fmt.Println(updateInfo.ReleaseNotes)
		}
		fmt.Println("\nTo install the update, run: nl-to-shell update install")
	} else {
		fmt.Println("✅ You are running the latest version")
	}

	return nil
}

// executeUpdateInstall handles the update install command
func executeUpdateInstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration to check update settings
	configManager := config.NewManager()
	cfg, err := configManager.Load()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
		}
		// Use default configuration
		cfg = &types.Config{
			UpdateSettings: types.UpdateSettings{
				AllowPrerelease:    false,
				BackupBeforeUpdate: true,
			},
		}
	}

	// Check for flags
	prerelease, _ := cmd.Flags().GetBool("prerelease")
	if prerelease {
		cfg.UpdateSettings.AllowPrerelease = true
	}

	noBackup, _ := cmd.Flags().GetBool("no-backup")
	if noBackup {
		cfg.UpdateSettings.BackupBeforeUpdate = false
	}

	// Create update manager
	updateManager := updater.NewManager(Version, "nl-to-shell", "nl-to-shell")

	fmt.Println("Checking for updates...")
	if cfg.UpdateSettings.AllowPrerelease {
		fmt.Println("(including prerelease versions)")
	}

	updateInfo, err := updateManager.CheckForUpdates(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !updateInfo.Available {
		fmt.Println("✅ You are already running the latest version")
		return nil
	}

	fmt.Printf("Installing update from %s to %s...\n", updateInfo.CurrentVersion, updateInfo.LatestVersion)

	if cfg.UpdateSettings.BackupBeforeUpdate {
		fmt.Println("Creating backup before update...")
	}

	if err := updateManager.PerformUpdate(ctx, updateInfo); err != nil {
		return fmt.Errorf("failed to install update: %w", err)
	}

	fmt.Println("✅ Update installed successfully!")
	fmt.Println("Please restart nl-to-shell to use the new version.")

	return nil
}

// executeConfigSetup handles the config setup command
func executeConfigSetup(cmd *cobra.Command, args []string) error {
	timer := globalMonitor.StartTimer("config.setup", nil)
	defer timer.Stop()

	fmt.Println("🔧 nl-to-shell Configuration Setup")
	fmt.Println("===================================")

	configManager := config.NewManager()

	// Try to load existing configuration
	_, err := configManager.Load()
	if err != nil && verbose {
		fmt.Printf("Creating new configuration (existing config not found: %v)\n", err)
	}

	// Run interactive setup
	if err := configManager.SetupInteractive(); err != nil {
		nlErr := &types.NLShellError{
			Type:      types.ErrTypeConfiguration,
			Message:   "interactive configuration setup failed",
			Cause:     err,
			Severity:  types.SeverityError,
			Timestamp: time.Now(),
		}
		globalLogger.LogError(nlErr)
		globalMonitor.RecordCounter("config.setup_failures", 1, nil)
		return fmt.Errorf("configuration setup failed: %w", err)
	}

	globalMonitor.RecordCounter("config.setup_success", 1, nil)
	fmt.Println("✅ Configuration setup completed successfully!")
	return nil
}

// executeConfigShow handles the config show command
func executeConfigShow(cmd *cobra.Command, args []string) error {
	timer := globalMonitor.StartTimer("config.show", nil)
	defer timer.Stop()

	configManager := config.NewManager()
	cfg, err := configManager.Load()
	if err != nil {
		nlErr := &types.NLShellError{
			Type:      types.ErrTypeConfiguration,
			Message:   "failed to load configuration",
			Cause:     err,
			Severity:  types.SeverityError,
			Timestamp: time.Now(),
		}
		globalLogger.LogError(nlErr)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("⚙️  Current Configuration")
	fmt.Println("========================")
	fmt.Printf("Default Provider: %s\n", cfg.DefaultProvider)

	fmt.Println("\nConfigured Providers:")
	for name, providerCfg := range cfg.Providers {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Default Model: %s\n", providerCfg.DefaultModel)
		fmt.Printf("    Timeout: %v\n", providerCfg.Timeout)
		if providerCfg.BaseURL != "" {
			fmt.Printf("    Base URL: %s\n", providerCfg.BaseURL)
		}
		// Mask API key
		if providerCfg.APIKey != "" {
			fmt.Printf("    API Key: %s***\n", providerCfg.APIKey[:min(8, len(providerCfg.APIKey))])
		} else {
			fmt.Printf("    API Key: (not configured)\n")
		}
	}

	fmt.Println("\nUser Preferences:")
	fmt.Printf("  Default Timeout: %v\n", cfg.UserPreferences.DefaultTimeout)
	fmt.Printf("  Max File List Size: %d\n", cfg.UserPreferences.MaxFileListSize)
	fmt.Printf("  Enable Plugins: %v\n", cfg.UserPreferences.EnablePlugins)
	fmt.Printf("  Auto Update: %v\n", cfg.UserPreferences.AutoUpdate)

	fmt.Println("\nUpdate Settings:")
	fmt.Printf("  Auto Check: %v\n", cfg.UpdateSettings.AutoCheck)
	fmt.Printf("  Check Interval: %v\n", cfg.UpdateSettings.CheckInterval)
	fmt.Printf("  Allow Prerelease: %v\n", cfg.UpdateSettings.AllowPrerelease)
	fmt.Printf("  Backup Before Update: %v\n", cfg.UpdateSettings.BackupBeforeUpdate)

	globalMonitor.RecordCounter("config.show_success", 1, nil)
	return nil
}

// executeConfigReset handles the config reset command
func executeConfigReset(cmd *cobra.Command, args []string) error {
	timer := globalMonitor.StartTimer("config.reset", nil)
	defer timer.Stop()

	fmt.Println("⚠️  Configuration Reset")
	fmt.Println("======================")
	fmt.Println("This will reset all configuration settings to their default values.")
	fmt.Print("Are you sure you want to continue? (y/N): ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("Configuration reset cancelled.")
		return nil
	}

	// Create default configuration
	defaultCfg := &types.Config{
		DefaultProvider: "openai",
		Providers:       make(map[string]types.ProviderConfig),
		UserPreferences: types.UserPreferences{
			DefaultTimeout:  30 * time.Second,
			MaxFileListSize: 100,
			EnablePlugins:   true,
			AutoUpdate:      true,
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck:          true,
			CheckInterval:      24 * time.Hour,
			AllowPrerelease:    false,
			BackupBeforeUpdate: true,
		},
	}

	configManager := config.NewManager()
	if err := configManager.Save(defaultCfg); err != nil {
		nlErr := &types.NLShellError{
			Type:      types.ErrTypeConfiguration,
			Message:   "failed to save default configuration",
			Cause:     err,
			Severity:  types.SeverityError,
			Timestamp: time.Now(),
		}
		globalLogger.LogError(nlErr)
		globalMonitor.RecordCounter("config.reset_failures", 1, nil)
		return fmt.Errorf("failed to reset configuration: %w", err)
	}

	globalMonitor.RecordCounter("config.reset_success", 1, nil)
	fmt.Println("✅ Configuration has been reset to default values.")
	fmt.Println("Run 'nl-to-shell config setup' to configure providers.")
	return nil
}

// executeHelp handles the help command
func executeHelp(cmd *cobra.Command, args []string) error {
	helpSystem := NewHelpSystem()

	if len(args) == 0 {
		// Show overview
		helpSystem.DisplayOverview()
		return nil
	}

	// Show specific topic
	topic := args[0]
	if err := helpSystem.DisplayTopic(topic); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Println("\nAvailable topics:")
		topics := helpSystem.ListTopics()
		for _, t := range topics {
			fmt.Printf("  - %s\n", t)
		}
		return err
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
