package llm

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// RetryConfig defines retry behavior for provider operations
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []ErrorMatcher
}

// ErrorMatcher defines conditions for retryable errors
type ErrorMatcher func(error) bool

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []ErrorMatcher{
			IsNetworkError,
			IsRateLimitError,
			IsTemporaryError,
		},
	}
}

// IsNetworkError checks if an error is network-related
func IsNetworkError(err error) bool {
	if nlErr, ok := err.(*types.NLShellError); ok {
		return nlErr.Type == types.ErrTypeNetwork
	}
	return false
}

// IsRateLimitError checks if an error is rate limit-related
func IsRateLimitError(err error) bool {
	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type == types.ErrTypeProvider {
			if httpErr, ok := nlErr.Context["http_status"].(int); ok {
				return httpErr == http.StatusTooManyRequests
			}
		}
	}
	return false
}

// IsTemporaryError checks if an error is temporary
func IsTemporaryError(err error) bool {
	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type == types.ErrTypeProvider {
			if httpErr, ok := nlErr.Context["http_status"].(int); ok {
				return httpErr >= 500 && httpErr < 600
			}
		}
	}
	return false
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// ExecuteWithRetry executes an operation with retry logic
func ExecuteWithRetry(ctx context.Context, config *RetryConfig, operation RetryableOperation) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		shouldRetry := false
		for _, matcher := range config.RetryableErrors {
			if matcher(err) {
				shouldRetry = true
				break
			}
		}

		if !shouldRetry {
			break
		}
	}

	return &types.NLShellError{
		Type:    types.ErrTypeProvider,
		Message: fmt.Sprintf("operation failed after %d attempts", config.MaxRetries+1),
		Cause:   lastErr,
		Context: map[string]interface{}{
			"max_retries": config.MaxRetries,
		},
	}
}

// ProviderFactory creates LLM provider instances
type ProviderFactory struct {
	retryConfig *RetryConfig
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		retryConfig: DefaultRetryConfig(),
	}
}

// NewProviderFactoryWithRetry creates a new provider factory with custom retry config
func NewProviderFactoryWithRetry(retryConfig *RetryConfig) *ProviderFactory {
	return &ProviderFactory{
		retryConfig: retryConfig,
	}
}

// CreateProvider creates a provider instance for the given provider name
func (f *ProviderFactory) CreateProvider(providerName string, config *types.ProviderConfig) (interfaces.LLMProvider, error) {
	switch providerName {
	case "openai":
		return NewOpenAIProvider(config, f.retryConfig), nil
	case "anthropic":
		return NewAnthropicProvider(config, f.retryConfig), nil
	case "gemini":
		return NewGeminiProvider(config, f.retryConfig), nil
	case "openrouter":
		return NewOpenRouterProvider(config, f.retryConfig), nil
	case "ollama":
		return NewOllamaProvider(config, f.retryConfig), nil
	default:
		return nil, &types.NLShellError{
			Type:    types.ErrTypeConfiguration,
			Message: "unsupported provider: " + providerName,
			Context: map[string]interface{}{
				"provider":            providerName,
				"supported_providers": []string{"openai", "anthropic", "gemini", "openrouter", "ollama"},
			},
		}
	}
}

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	config        *types.ProviderConfig
	retryConfig   *RetryConfig
	httpClient    *http.Client
	promptBuilder *PromptBuilder
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(config *types.ProviderConfig, retryConfig *RetryConfig) *BaseProvider {
	timeout := 30 * time.Second
	if config != nil && config.Timeout > 0 {
		timeout = config.Timeout
	}

	return &BaseProvider{
		config:        config,
		retryConfig:   retryConfig,
		promptBuilder: NewPromptBuilder(),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// executeWithRetry wraps an operation with retry logic
func (bp *BaseProvider) executeWithRetry(ctx context.Context, operation RetryableOperation) error {
	return ExecuteWithRetry(ctx, bp.retryConfig, operation)
}

// validateConfig validates the provider configuration
func (bp *BaseProvider) validateConfig() error {
	if bp.config == nil {
		return &types.NLShellError{
			Type:    types.ErrTypeConfiguration,
			Message: "provider configuration is nil",
		}
	}
	return nil
}

// OpenAIProvider implements LLMProvider for OpenAI
type OpenAIProvider struct {
	*BaseProvider
}

func NewOpenAIProvider(config *types.ProviderConfig, retryConfig *RetryConfig) interfaces.LLMProvider {
	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(config, retryConfig),
	}
}

func (p *OpenAIProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.CommandResponse
	var err error

	operation := func() error {
		response, err = p.generateCommandInternal(ctx, prompt, context)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OpenAIProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.ValidationResponse
	var err error

	operation := func() error {
		response, err = p.validateResultInternal(ctx, command, output, intent)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OpenAIProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:            "openai",
		RequiresAuth:    true,
		SupportedModels: []string{"gpt-3.5-turbo", "gpt-4", "gpt-4-turbo"},
	}
}

// AnthropicProvider implements LLMProvider for Anthropic
type AnthropicProvider struct {
	*BaseProvider
}

func NewAnthropicProvider(config *types.ProviderConfig, retryConfig *RetryConfig) interfaces.LLMProvider {
	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(config, retryConfig),
	}
}

func (p *AnthropicProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.CommandResponse
	var err error

	operation := func() error {
		response, err = p.generateCommandInternal(ctx, prompt, context)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *AnthropicProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.ValidationResponse
	var err error

	operation := func() error {
		response, err = p.validateResultInternal(ctx, command, output, intent)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *AnthropicProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:            "anthropic",
		RequiresAuth:    true,
		SupportedModels: []string{"claude-3-haiku", "claude-3-sonnet", "claude-3-opus"},
	}
}

// GeminiProvider implements LLMProvider for Google Gemini
type GeminiProvider struct {
	*BaseProvider
}

func NewGeminiProvider(config *types.ProviderConfig, retryConfig *RetryConfig) interfaces.LLMProvider {
	return &GeminiProvider{
		BaseProvider: NewBaseProvider(config, retryConfig),
	}
}

func (p *GeminiProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.CommandResponse
	var err error

	operation := func() error {
		response, err = p.generateCommandInternal(ctx, prompt, context)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *GeminiProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.ValidationResponse
	var err error

	operation := func() error {
		response, err = p.validateResultInternal(ctx, command, output, intent)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *GeminiProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:            "gemini",
		RequiresAuth:    true,
		SupportedModels: []string{"gemini-pro", "gemini-pro-vision"},
	}
}

// OpenRouterProvider implements LLMProvider for OpenRouter
type OpenRouterProvider struct {
	*BaseProvider
}

func NewOpenRouterProvider(config *types.ProviderConfig, retryConfig *RetryConfig) interfaces.LLMProvider {
	return &OpenRouterProvider{
		BaseProvider: NewBaseProvider(config, retryConfig),
	}
}

func (p *OpenRouterProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.CommandResponse
	var err error

	operation := func() error {
		response, err = p.generateCommandInternal(ctx, prompt, context)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OpenRouterProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.ValidationResponse
	var err error

	operation := func() error {
		response, err = p.validateResultInternal(ctx, command, output, intent)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OpenRouterProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:            "openrouter",
		RequiresAuth:    true,
		SupportedModels: []string{"openai/gpt-4", "anthropic/claude-3-opus", "meta-llama/llama-2-70b-chat"},
	}
}

// OllamaProvider implements LLMProvider for Ollama
type OllamaProvider struct {
	*BaseProvider
}

func NewOllamaProvider(config *types.ProviderConfig, retryConfig *RetryConfig) interfaces.LLMProvider {
	return &OllamaProvider{
		BaseProvider: NewBaseProvider(config, retryConfig),
	}
}

func (p *OllamaProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.CommandResponse
	var err error

	operation := func() error {
		response, err = p.generateCommandInternal(ctx, prompt, context)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OllamaProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	var response *types.ValidationResponse
	var err error

	operation := func() error {
		response, err = p.validateResultInternal(ctx, command, output, intent)
		return err
	}

	if retryErr := p.executeWithRetry(ctx, operation); retryErr != nil {
		return nil, retryErr
	}

	return response, nil
}

func (p *OllamaProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:            "ollama",
		RequiresAuth:    false,
		SupportedModels: []string{"llama2", "codellama", "mistral"},
	}
}
