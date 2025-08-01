package cli

import (
	"fmt"
	"strings"
)

// HelpSystem provides comprehensive help and documentation
type HelpSystem struct {
	topics map[string]HelpTopic
}

// HelpTopic represents a help topic with detailed information
type HelpTopic struct {
	Title       string
	Description string
	Usage       string
	Examples    []HelpExample
	SeeAlso     []string
}

// HelpExample represents a usage example
type HelpExample struct {
	Description string
	Command     string
	Output      string
}

// NewHelpSystem creates a new help system with all topics
func NewHelpSystem() *HelpSystem {
	hs := &HelpSystem{
		topics: make(map[string]HelpTopic),
	}

	hs.initializeTopics()
	return hs
}

// GetTopic returns help for a specific topic
func (hs *HelpSystem) GetTopic(topic string) (HelpTopic, bool) {
	t, exists := hs.topics[strings.ToLower(topic)]
	return t, exists
}

// ListTopics returns all available help topics
func (hs *HelpSystem) ListTopics() []string {
	topics := make([]string, 0, len(hs.topics))
	for topic := range hs.topics {
		topics = append(topics, topic)
	}
	return topics
}

// DisplayTopic displays help for a specific topic
func (hs *HelpSystem) DisplayTopic(topic string) error {
	t, exists := hs.topics[strings.ToLower(topic)]
	if !exists {
		return fmt.Errorf("help topic '%s' not found", topic)
	}

	fmt.Printf("📖 %s\n", t.Title)
	fmt.Println(strings.Repeat("=", len(t.Title)+4))
	fmt.Println()

	if t.Description != "" {
		fmt.Println("Description:")
		fmt.Println(t.Description)
		fmt.Println()
	}

	if t.Usage != "" {
		fmt.Println("Usage:")
		fmt.Println(t.Usage)
		fmt.Println()
	}

	if len(t.Examples) > 0 {
		fmt.Println("Examples:")
		for i, example := range t.Examples {
			fmt.Printf("%d. %s\n", i+1, example.Description)
			fmt.Printf("   $ %s\n", example.Command)
			if example.Output != "" {
				fmt.Printf("   %s\n", example.Output)
			}
			fmt.Println()
		}
	}

	if len(t.SeeAlso) > 0 {
		fmt.Println("See Also:")
		for _, related := range t.SeeAlso {
			fmt.Printf("  - %s\n", related)
		}
		fmt.Println()
	}

	return nil
}

// DisplayOverview displays a general help overview
func (hs *HelpSystem) DisplayOverview() {
	fmt.Println("🚀 nl-to-shell - Natural Language to Shell Commands")
	fmt.Println("===================================================")
	fmt.Println()
	fmt.Println("nl-to-shell converts natural language descriptions into executable")
	fmt.Println("shell commands using Large Language Models (LLMs). It provides")
	fmt.Println("context-aware command generation with safety features.")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  1. Configure a provider: nl-to-shell config setup")
	fmt.Println("  2. Generate a command: nl-to-shell \"list files by size\"")
	fmt.Println("  3. Start a session: nl-to-shell session")
	fmt.Println()
	fmt.Println("Available Help Topics:")
	topics := hs.ListTopics()
	for _, topic := range topics {
		if t, exists := hs.topics[topic]; exists {
			fmt.Printf("  %-15s - %s\n", topic, strings.Split(t.Description, "\n")[0])
		}
	}
	fmt.Println()
	fmt.Println("Use 'nl-to-shell help <topic>' for detailed information on a topic.")
}

// initializeTopics sets up all help topics
func (hs *HelpSystem) initializeTopics() {
	hs.topics["getting-started"] = HelpTopic{
		Title: "Getting Started with nl-to-shell",
		Description: `nl-to-shell helps you convert natural language descriptions into shell commands.
Before using it, you need to configure at least one AI provider.`,
		Usage: `1. Run initial setup: nl-to-shell config setup
2. Generate your first command: nl-to-shell "list files"
3. Use session mode for multiple commands: nl-to-shell session`,
		Examples: []HelpExample{
			{
				Description: "Set up configuration interactively",
				Command:     "nl-to-shell config setup",
				Output:      "Guides you through provider setup",
			},
			{
				Description: "Generate a simple command",
				Command:     `nl-to-shell "show current directory contents"`,
				Output:      "Generated command: ls -la",
			},
		},
		SeeAlso: []string{"providers", "configuration", "safety"},
	}

	hs.topics["providers"] = HelpTopic{
		Title: "AI Providers",
		Description: `nl-to-shell supports multiple AI providers for command generation.
Each provider has different models, pricing, and capabilities.`,
		Usage: `Configure providers using: nl-to-shell config setup
Override provider at runtime: nl-to-shell --provider openai "command"`,
		Examples: []HelpExample{
			{
				Description: "Use OpenAI with specific model",
				Command:     `nl-to-shell --provider openai --model gpt-4 "find large files"`,
			},
			{
				Description: "Use local Ollama instance",
				Command:     `nl-to-shell --provider ollama "list processes"`,
			},
		},
		SeeAlso: []string{"configuration", "models"},
	}

	hs.topics["safety"] = HelpTopic{
		Title: "Safety Features",
		Description: `nl-to-shell includes comprehensive safety features to prevent
dangerous command execution. Commands are analyzed for potential risks.`,
		Usage: `Safety checks are automatic. Use --skip-confirmation to bypass (advanced users).
Use --dry-run to preview commands without execution.`,
		Examples: []HelpExample{
			{
				Description: "Preview a potentially dangerous command",
				Command:     `nl-to-shell --dry-run "delete all temporary files"`,
				Output:      "Shows analysis without executing",
			},
			{
				Description: "Skip confirmation for trusted commands",
				Command:     `nl-to-shell --skip-confirmation "remove old logs"`,
			},
		},
		SeeAlso: []string{"dry-run", "confirmation"},
	}

	hs.topics["session"] = HelpTopic{
		Title: "Interactive Sessions",
		Description: `Session mode allows you to run multiple commands without restarting
the tool. Context and configuration are maintained between commands.`,
		Usage: `Start a session: nl-to-shell session
Use special commands: help, history, config, stats, exit`,
		Examples: []HelpExample{
			{
				Description: "Start an interactive session",
				Command:     "nl-to-shell session",
				Output:      "Enters interactive mode",
			},
			{
				Description: "Start session with specific provider",
				Command:     "nl-to-shell --provider anthropic session",
			},
		},
		SeeAlso: []string{"commands", "history"},
	}

	hs.topics["configuration"] = HelpTopic{
		Title: "Configuration Management",
		Description: `nl-to-shell stores configuration in your system's standard config directory.
Configuration includes provider settings, credentials, and user preferences.`,
		Usage: `Setup: nl-to-shell config setup
Show: nl-to-shell config show
Reset: nl-to-shell config reset`,
		Examples: []HelpExample{
			{
				Description: "View current configuration",
				Command:     "nl-to-shell config show",
				Output:      "Displays all settings (credentials masked)",
			},
			{
				Description: "Reset to defaults",
				Command:     "nl-to-shell config reset",
				Output:      "Resets all settings to defaults",
			},
		},
		SeeAlso: []string{"providers", "credentials"},
	}

	hs.topics["dry-run"] = HelpTopic{
		Title: "Dry Run Mode",
		Description: `Dry run mode analyzes and previews commands without executing them.
This is useful for understanding what a command will do before running it.`,
		Usage: `Add --dry-run flag to any command generation.`,
		Examples: []HelpExample{
			{
				Description: "Preview a file operation",
				Command:     `nl-to-shell --dry-run "copy all images to backup folder"`,
				Output:      "Shows predicted outcomes without execution",
			},
		},
		SeeAlso: []string{"safety", "preview"},
	}

	hs.topics["updates"] = HelpTopic{
		Title: "Update Management",
		Description: `nl-to-shell can automatically check for and install updates.
Updates include new features, bug fixes, and security improvements.`,
		Usage: `Check for updates: nl-to-shell update check
Install updates: nl-to-shell update install`,
		Examples: []HelpExample{
			{
				Description: "Check for available updates",
				Command:     "nl-to-shell update check",
				Output:      "Shows current and latest versions",
			},
			{
				Description: "Install latest update",
				Command:     "nl-to-shell update install",
				Output:      "Downloads and installs update",
			},
		},
		SeeAlso: []string{"version", "installation"},
	}

	hs.topics["troubleshooting"] = HelpTopic{
		Title:       "Troubleshooting",
		Description: `Common issues and solutions for nl-to-shell usage.`,
		Usage: `Use --verbose flag for detailed output.
Check logs and configuration if commands fail.`,
		Examples: []HelpExample{
			{
				Description: "Debug command generation issues",
				Command:     `nl-to-shell --verbose "your command here"`,
				Output:      "Shows detailed execution information",
			},
			{
				Description: "Verify configuration",
				Command:     "nl-to-shell config show",
				Output:      "Displays current settings",
			},
		},
		SeeAlso: []string{"configuration", "logs", "support"},
	}
}
