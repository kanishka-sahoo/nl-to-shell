# nl-to-shell

[![Go Version](https://img.shields.io/github/go-mod/go-version/nl-to-shell/nl-to-shell)](https://golang.org/)
[![License](https://img.shields.io/github/license/nl-to-shell/nl-to-shell)](LICENSE)
[![Release](https://img.shields.io/github/v/release/nl-to-shell/nl-to-shell)](https://github.com/nl-to-shell/nl-to-shell/releases)
[![Tests](https://img.shields.io/github/workflow/status/nl-to-shell/nl-to-shell/Tests)](https://github.com/nl-to-shell/nl-to-shell/actions)

A powerful command-line utility that converts natural language descriptions into executable shell commands using Large Language Models (LLMs). Built with safety, context-awareness, and extensibility in mind.

## 🚀 Features

### Core Functionality
- **🗣️ Natural Language Processing**: Convert plain English descriptions into precise shell commands
- **🧠 Context Awareness**: Analyzes current directory, git status, file types, and environment for enhanced accuracy
- **🔒 Safety First**: Comprehensive dangerous command detection with user confirmation prompts
- **🎯 Result Validation**: AI-powered validation of command execution results with automatic correction suggestions

### AI Provider Support
- **🤖 Multiple Providers**: OpenAI, Anthropic Claude, Google Gemini, OpenRouter, and Ollama
- **🏠 Local & Cloud**: Support for both cloud-based and local AI models
- **⚡ Provider Fallback**: Automatic fallback and retry mechanisms for reliability
- **🎛️ Model Selection**: Runtime provider and model override capabilities

### User Experience
- **💻 Cross-Platform**: Native support for Linux, macOS, and Windows
- **🔄 Interactive Sessions**: Continuous operation mode with persistent context
- **👁️ Dry Run Mode**: Preview commands before execution with detailed analysis
- **📊 Performance Monitoring**: Built-in metrics and performance tracking
- **🔧 Plugin System**: Extensible context providers for specialized environments

### Enterprise Features
- **🔄 Auto-Updates**: Intelligent update management with backup and recovery
- **📝 Comprehensive Logging**: Structured logging with configurable levels
- **⚙️ Configuration Management**: Secure credential storage with system keychain integration
- **🛡️ Security Audit**: Built-in security validation and audit trails

## 📦 Installation

### Package Managers (Recommended)

#### macOS (Homebrew)
```bash
brew install nl-to-shell/tap/nl-to-shell
```

#### Linux (APT)
```bash
curl -fsSL https://packages.nl-to-shell.com/gpg | sudo apt-key add -
echo "deb https://packages.nl-to-shell.com/apt stable main" | sudo tee /etc/apt/sources.list.d/nl-to-shell.list
sudo apt update && sudo apt install nl-to-shell
```

#### Windows (Chocolatey)
```powershell
choco install nl-to-shell
```

### Direct Download

Download the latest release for your platform from [GitHub Releases](https://github.com/nl-to-shell/nl-to-shell/releases).

### From Source

```bash
git clone https://github.com/nl-to-shell/nl-to-shell.git
cd nl-to-shell
make build
sudo make install
```

### Using Go Install

```bash
go install github.com/nl-to-shell/nl-to-shell/cmd/nl-to-shell@latest
```

## 🚀 Quick Start

### 1. Initial Configuration
Set up your AI provider credentials:
```bash
nl-to-shell config setup
```

This interactive setup will guide you through:
- Choosing your preferred AI provider
- Configuring API credentials
- Setting user preferences
- Testing the connection

### 2. Basic Usage
Generate your first command:
```bash
nl-to-shell "list files by size in descending order"
# Output: Generated command: ls -lhS
```

### 3. Safety Features
Preview potentially dangerous commands:
```bash
nl-to-shell --dry-run "delete all temporary files"
# Shows analysis without executing
```

### 4. Interactive Mode
Start a session for multiple commands:
```bash
nl-to-shell session
# Enters interactive mode with persistent context
```

## 💡 Usage Examples

### File Operations
```bash
# Find large files
nl-to-shell "find files larger than 100MB"

# Copy with progress
nl-to-shell "copy all images to backup folder with progress"

# Archive old logs
nl-to-shell "compress log files older than 30 days"
```

### System Administration
```bash
# Process management
nl-to-shell "show processes using most CPU"

# Disk usage analysis
nl-to-shell "show disk usage by directory"

# Network diagnostics
nl-to-shell "check if port 8080 is open"
```

### Development Workflows
```bash
# Git operations
nl-to-shell "show git status with file changes"

# Build and test
nl-to-shell "run tests and show coverage"

# Docker management
nl-to-shell "list running containers with resource usage"
```

### Advanced Features
```bash
# Provider selection
nl-to-shell --provider anthropic --model claude-3 "complex command"

# Skip confirmations (advanced users)
nl-to-shell --skip-confirmation "remove build artifacts"

# Disable result validation for speed
nl-to-shell --validate-results=false "simple listing command"
```

## Configuration

The tool stores configuration in platform-specific locations:
- Linux: `~/.config/nl-to-shell/`
- macOS: `~/Library/Application Support/nl-to-shell/`
- Windows: `%APPDATA%\nl-to-shell\`

## Supported Providers

- **OpenAI**: GPT-3.5, GPT-4, and newer models
- **Anthropic**: Claude models
- **Google Gemini**: Gemini Pro and other models
- **OpenRouter**: Access to multiple models through one API
- **Ollama**: Local and remote Ollama instances

## Safety Features

- Pattern-based dangerous command detection
- User confirmation for potentially harmful operations
- Dry run mode for command preview
- Result validation and automatic correction
- Configurable safety levels

## Development

### Project Structure

```
├── cmd/                    # CLI entry points
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── context/          # Context gathering
│   ├── executor/         # Command execution
│   ├── llm/             # LLM provider interfaces
│   ├── manager/         # Command orchestration
│   ├── plugins/         # Plugin system
│   ├── safety/          # Safety validation
│   └── updater/         # Update management
└── pkg/                  # Public library code
```

### Building

```bash
# Build for current platform
go build -o bin/nl-to-shell ./cmd/nl-to-shell

# Cross-compile for all platforms
make build-all
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Inspired by various natural language to code tools
- Thanks to the open source community for LLM integration libraries