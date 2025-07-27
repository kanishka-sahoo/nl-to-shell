package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nl-to-shell/nl-to-shell/internal/config"
	contextpkg "github.com/nl-to-shell/nl-to-shell/internal/context"
	"github.com/nl-to-shell/nl-to-shell/internal/executor"
	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/llm"
	"github.com/nl-to-shell/nl-to-shell/internal/manager"
	"github.com/nl-to-shell/nl-to-shell/internal/safety"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
	"github.com/nl-to-shell/nl-to-shell/internal/validator"
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
	Long: `nl-to-shell is a CLI utility that converts natural language descriptions 
into executable shell commands using Large Language Models (LLMs).

It provides context-aware command generation by analyzing your current working 
directory, git repository state, files, and other environmental factors.

The tool emphasizes safety through dangerous command detection, user confirmation 
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

		return executeCommandGeneration(args[0])
	},
	SilenceUsage: true, // Don't show usage on errors
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
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
	rootCmd.AddCommand(completionCmd)
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
	Long: `Configure providers, credentials, and user preferences.
Use subcommands to set up providers, view current configuration, or reset settings.`,
}

// setupCmd represents the config setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration setup",
	Long: `Run interactive setup to configure providers and credentials.
This will guide you through setting up API keys and preferences.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("configuration setup not yet implemented")
	},
}

// showCmd represents the config show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration settings (credentials will be masked).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("configuration display not yet implemented")
	},
}

// resetCmd represents the config reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  `Reset all configuration settings to their default values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("configuration reset not yet implemented")
	},
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
	Long: `Check for and install updates to nl-to-shell.
Use subcommands to check for updates or install them.`,
}

// checkCmd represents the update check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for available updates",
	Long:  `Check if there are any available updates without installing them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("update checking not yet implemented")
	},
}

// installCmd represents the update install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install available updates",
	Long:  `Install the latest available update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("update installation not yet implemented")
	},
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

func init() {
	// Add subcommands to config
	configCmd.AddCommand(setupCmd)
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(resetCmd)

	// Add subcommands to update
	updateCmd.AddCommand(checkCmd)
	updateCmd.AddCommand(installCmd)
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

// executeCommandGeneration handles the main command generation flow
func executeCommandGeneration(input string) error {
	ctx := context.Background()

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

	// Create a temporary LLM provider for result validator
	tempProvider, err := createLLMProvider(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider for validator: %w", err)
	}
	resultValidator := validator.NewResultValidator(tempProvider)

	// Create LLM provider
	llmProvider, err := createLLMProvider(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Create command manager
	commandManager := manager.NewManager(
		contextGatherer,
		llmProvider,
		safetyValidator,
		commandExecutor,
		resultValidator,
		cfg,
	)

	// Create execution options from CLI flags
	options := &types.ExecutionOptions{
		DryRun:           dryRun,
		SkipConfirmation: skipConfirmation,
		ValidateResults:  validateResults,
		Timeout:          cfg.UserPreferences.DefaultTimeout,
	}

	// Execute the full pipeline
	result, err := commandManager.GenerateAndExecute(ctx, input, options)
	if err != nil {
		return fmt.Errorf("command generation failed: %w", err)
	}

	// Display results
	return displayResults(result, input)
}

// createLLMProvider creates an LLM provider based on configuration
func createLLMProvider(cfg *types.Config) (interfaces.LLMProvider, error) {
	providerName := cfg.DefaultProvider
	if provider != "" {
		providerName = provider
	}

	// Get provider configuration
	providerConfig, exists := cfg.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not configured", providerName)
	}

	// Override model if specified via CLI
	if model != "" {
		providerConfig.DefaultModel = model
	}

	// Create provider using factory
	factory := llm.NewProviderFactory()
	return factory.CreateProvider(providerName, &providerConfig)
}

// displayResults displays the command generation and execution results
func displayResults(result *types.FullResult, originalInput string) error {
	// Display generated command
	fmt.Printf("Generated command: %s\n", result.CommandResult.Command.Generated)

	if verbose {
		fmt.Printf("Confidence: %.2f\n", result.CommandResult.Confidence)
		if len(result.CommandResult.Alternatives) > 0 {
			fmt.Println("Alternatives:")
			for i, alt := range result.CommandResult.Alternatives {
				fmt.Printf("  %d. %s\n", i+1, alt)
			}
		}
	}

	// Display safety information
	if result.CommandResult.Safety.DangerLevel > types.Safe {
		fmt.Printf("⚠️  Safety level: %s\n", result.CommandResult.Safety.DangerLevel.String())
		if len(result.CommandResult.Safety.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, warning := range result.CommandResult.Safety.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	}

	// Handle dry run results
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

	// Handle confirmation requirement
	if result.RequiresConfirmation {
		fmt.Printf("\n⚠️  This command requires confirmation. Use --skip-confirmation to bypass.\n")
		return nil
	}

	// Display execution results
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

		// Display validation results
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
