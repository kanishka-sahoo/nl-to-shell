package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	dryRun           bool
	verbose          bool
	provider         string
	model            string
	skipConfirmation bool
	validateResults  bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nl-to-shell",
	Short: "Convert natural language to shell commands",
	Long: `nl-to-shell is a CLI utility that converts natural language descriptions 
into executable shell commands using Large Language Models (LLMs).

It provides context-aware command generation by analyzing your current working 
directory, git repository state, files, and other environmental factors.`,
	Example: `  # Generate a command from natural language
  nl-to-shell "list files by size in descending order"
  
  # Use dry run mode to preview the command
  nl-to-shell --dry-run "delete all .tmp files"
  
  # Use a specific provider and model
  nl-to-shell --provider openai --model gpt-4 "find large files"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// This will be implemented in later tasks
		return fmt.Errorf("command generation not yet implemented")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview the command without executing it")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "", "LLM provider to use (openai, anthropic, gemini, openrouter, ollama)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to use for the specified provider")
	rootCmd.PersistentFlags().BoolVar(&skipConfirmation, "skip-confirmation", false, "Skip confirmation prompts for dangerous commands")
	rootCmd.PersistentFlags().BoolVar(&validateResults, "validate-results", true, "Validate command results using AI")

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Long:  `Configure providers, credentials, and user preferences.`,
}

// setupCmd represents the config setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration setup",
	Long:  `Run interactive setup to configure providers and credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("configuration setup not yet implemented")
	},
}

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Start an interactive session",
	Long: `Start an interactive session for continuous command generation 
without restarting the tool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("session mode not yet implemented")
	},
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Manage updates",
	Long:  `Check for and install updates.`,
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
	Long:  `Display the current version of nl-to-shell.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("nl-to-shell version 0.1.0-dev")
	},
}

func init() {
	// Add subcommands to config
	configCmd.AddCommand(setupCmd)

	// Add subcommands to update
	updateCmd.AddCommand(checkCmd)
	updateCmd.AddCommand(installCmd)
}
