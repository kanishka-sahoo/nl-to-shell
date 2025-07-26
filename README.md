# nl-to-shell

A command-line utility that converts natural language descriptions into executable shell commands using Large Language Models (LLMs).

## Features

- **Natural Language to Shell**: Convert plain English descriptions into shell commands
- **Context Awareness**: Analyzes current directory, git status, and environment for better command generation
- **Multiple AI Providers**: Support for OpenAI, Anthropic, Google Gemini, OpenRouter, and Ollama
- **Safety First**: Dangerous command detection with user confirmation prompts
- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Plugin System**: Extensible context providers for enhanced command generation
- **Auto-Updates**: Automatic update management and version control

## Installation

### From Source

```bash
git clone https://github.com/nl-to-shell/nl-to-shell.git
cd nl-to-shell
go build -o bin/nl-to-shell ./cmd/nl-to-shell
```

### Using Go Install

```bash
go install github.com/nl-to-shell/nl-to-shell/cmd/nl-to-shell@latest
```

## Quick Start

1. **Initial Setup**
   ```bash
   nl-to-shell config setup
   ```

2. **Generate a Command**
   ```bash
   nl-to-shell "list files by size"
   ```

3. **Dry Run Mode**
   ```bash
   nl-to-shell --dry-run "delete all .tmp files"
   ```

4. **Interactive Session**
   ```bash
   nl-to-shell session
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