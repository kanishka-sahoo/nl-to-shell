package llm

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// MockProvider implements LLMProvider for testing
type MockProvider struct {
	generateCommandFunc func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error)
	validateResultFunc  func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error)
	getProviderInfoFunc func() types.ProviderInfo
}

func (m *MockProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if m.generateCommandFunc != nil {
		return m.generateCommandFunc(ctx, prompt, context)
	}
	return &types.CommandResponse{
		Command:     "echo 'mock command'",
		Explanation: "Mock response",
		Confidence:  1.0,
	}, nil
}

func (m *MockProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if m.validateResultFunc != nil {
		return m.validateResultFunc(ctx, command, output, intent)
	}
	return &types.ValidationResponse{
		IsCorrect:   true,
		Explanation: "Mock validation",
	}, nil
}

func (m *MockProvider) GetProviderInfo() types.ProviderInfo {
	if m.getProviderInfoFunc != nil {
		return m.getProviderInfoFunc()
	}
	return types.ProviderInfo{
		Name:            "mock",
		RequiresAuth:    false,
		SupportedModels: []string{"mock-model"},
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.InitialDelay != time.Second {
		t.Errorf("Expected InitialDelay to be 1s, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor to be 2.0, got %f", config.BackoffFactor)
	}

	if len(config.RetryableErrors) != 3 {
		t.Errorf("Expected 3 retryable error matchers, got %d", len(config.RetryableErrors))
	}
}

func TestErrorMatchers(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		matcher  ErrorMatcher
		expected bool
	}{
		{
			name: "network error",
			err: &types.NLShellError{
				Type:    types.ErrTypeNetwork,
				Message: "network error",
			},
			matcher:  IsNetworkError,
			expected: true,
		},
		{
			name: "non-network error",
			err: &types.NLShellError{
				Type:    types.ErrTypeConfiguration,
				Message: "config error",
			},
			matcher:  IsNetworkError,
			expected: false,
		},
		{
			name: "rate limit error",
			err: &types.NLShellError{
				Type:    types.ErrTypeProvider,
				Message: "rate limited",
				Context: map[string]interface{}{
					"http_status": http.StatusTooManyRequests,
				},
			},
			matcher:  IsRateLimitError,
			expected: true,
		},
		{
			name: "temporary error",
			err: &types.NLShellError{
				Type:    types.ErrTypeProvider,
				Message: "server error",
				Context: map[string]interface{}{
					"http_status": http.StatusInternalServerError,
				},
			},
			matcher:  IsTemporaryError,
			expected: true,
		},
		{
			name: "non-temporary error",
			err: &types.NLShellError{
				Type:    types.ErrTypeProvider,
				Message: "bad request",
				Context: map[string]interface{}{
					"http_status": http.StatusBadRequest,
				},
			},
			matcher:  IsTemporaryError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.matcher(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExecuteWithRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("success on first attempt", func(t *testing.T) {
		config := &RetryConfig{
			MaxRetries:    3,
			InitialDelay:  time.Millisecond,
			MaxDelay:      time.Second,
			BackoffFactor: 2.0,
		}

		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		err := ExecuteWithRetry(ctx, config, operation)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		config := &RetryConfig{
			MaxRetries:      3,
			InitialDelay:    time.Millisecond,
			MaxDelay:        time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []ErrorMatcher{IsNetworkError},
		}

		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return &types.NLShellError{
					Type:    types.ErrTypeNetwork,
					Message: "network error",
				}
			}
			return nil
		}

		err := ExecuteWithRetry(ctx, config, operation)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		config := &RetryConfig{
			MaxRetries:      3,
			InitialDelay:    time.Millisecond,
			MaxDelay:        time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []ErrorMatcher{IsNetworkError},
		}

		attempts := 0
		operation := func() error {
			attempts++
			return &types.NLShellError{
				Type:    types.ErrTypeConfiguration,
				Message: "config error",
			}
		}

		err := ExecuteWithRetry(ctx, config, operation)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		config := &RetryConfig{
			MaxRetries:      2,
			InitialDelay:    time.Millisecond,
			MaxDelay:        time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []ErrorMatcher{IsNetworkError},
		}

		attempts := 0
		operation := func() error {
			attempts++
			return &types.NLShellError{
				Type:    types.ErrTypeNetwork,
				Message: "network error",
			}
		}

		err := ExecuteWithRetry(ctx, config, operation)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if attempts != 3 { // MaxRetries + 1
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}

		// Check that the error is wrapped correctly
		if nlErr, ok := err.(*types.NLShellError); ok {
			if nlErr.Type != types.ErrTypeProvider {
				t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
			}
		} else {
			t.Error("Expected NLShellError")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		config := &RetryConfig{
			MaxRetries:      3,
			InitialDelay:    time.Second,
			MaxDelay:        time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []ErrorMatcher{IsNetworkError},
		}

		attempts := 0
		operation := func() error {
			attempts++
			return &types.NLShellError{
				Type:    types.ErrTypeNetwork,
				Message: "network error",
			}
		}

		err := ExecuteWithRetry(ctx, config, operation)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

func TestProviderFactory(t *testing.T) {
	factory := NewProviderFactory()

	config := &types.ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "https://api.test.com",
		DefaultModel: "test-model",
		Timeout:      30 * time.Second,
	}

	t.Run("create supported providers", func(t *testing.T) {
		providers := []string{"openai", "anthropic", "gemini", "openrouter", "ollama"}

		for _, providerName := range providers {
			provider, err := factory.CreateProvider(providerName, config)
			if err != nil {
				t.Errorf("Failed to create %s provider: %v", providerName, err)
			}

			if provider == nil {
				t.Errorf("Provider %s is nil", providerName)
			}

			// Test that the provider is not nil (interface implementation is guaranteed by type)
			if provider == nil {
				t.Errorf("Provider %s is nil", providerName)
			}
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		provider, err := factory.CreateProvider("unsupported", config)
		if err == nil {
			t.Error("Expected error for unsupported provider")
		}

		if provider != nil {
			t.Error("Expected nil provider for unsupported provider")
		}

		// Check error details
		if nlErr, ok := err.(*types.NLShellError); ok {
			if nlErr.Type != types.ErrTypeConfiguration {
				t.Errorf("Expected error type %v, got %v", types.ErrTypeConfiguration, nlErr.Type)
			}
		} else {
			t.Error("Expected NLShellError")
		}
	})
}

func TestProviderFactoryWithCustomRetry(t *testing.T) {
	customRetryConfig := &RetryConfig{
		MaxRetries:    5,
		InitialDelay:  2 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 3.0,
	}

	factory := NewProviderFactoryWithRetry(customRetryConfig)

	config := &types.ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "https://api.test.com",
		DefaultModel: "test-model",
		Timeout:      30 * time.Second,
	}

	provider, err := factory.CreateProvider("openai", config)
	if err != nil {
		t.Errorf("Failed to create provider: %v", err)
	}

	if provider == nil {
		t.Error("Provider is nil")
	}
}

func TestBaseProvider(t *testing.T) {
	config := &types.ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "https://api.test.com",
		DefaultModel: "test-model",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	baseProvider := NewBaseProvider(config, retryConfig)

	t.Run("validate config success", func(t *testing.T) {
		err := baseProvider.validateConfig()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("validate config nil", func(t *testing.T) {
		baseProvider := NewBaseProvider(nil, retryConfig)
		err := baseProvider.validateConfig()
		if err == nil {
			t.Error("Expected error for nil config")
		}

		if nlErr, ok := err.(*types.NLShellError); ok {
			if nlErr.Type != types.ErrTypeConfiguration {
				t.Errorf("Expected error type %v, got %v", types.ErrTypeConfiguration, nlErr.Type)
			}
		} else {
			t.Error("Expected NLShellError")
		}
	})

	t.Run("http client configuration", func(t *testing.T) {
		if baseProvider.httpClient == nil {
			t.Error("HTTP client is nil")
		}

		if baseProvider.httpClient.Timeout != config.Timeout {
			t.Errorf("Expected timeout %v, got %v", config.Timeout, baseProvider.httpClient.Timeout)
		}
	})

	t.Run("default timeout", func(t *testing.T) {
		configNoTimeout := &types.ProviderConfig{
			APIKey:       "test-key",
			BaseURL:      "https://api.test.com",
			DefaultModel: "test-model",
		}

		baseProvider := NewBaseProvider(configNoTimeout, retryConfig)
		expectedTimeout := 30 * time.Second

		if baseProvider.httpClient.Timeout != expectedTimeout {
			t.Errorf("Expected default timeout %v, got %v", expectedTimeout, baseProvider.httpClient.Timeout)
		}
	})
}

func TestProviderImplementations(t *testing.T) {
	config := &types.ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "https://api.test.com",
		DefaultModel: "test-model",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()

	providers := map[string]interfaces.LLMProvider{
		"openai":     NewOpenAIProvider(config, retryConfig),
		"anthropic":  NewAnthropicProvider(config, retryConfig),
		"gemini":     NewGeminiProvider(config, retryConfig),
		"openrouter": NewOpenRouterProvider(config, retryConfig),
		"ollama":     NewOllamaProvider(config, retryConfig),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			// Test GetProviderInfo
			info := provider.GetProviderInfo()
			if info.Name == "" {
				t.Error("Provider name is empty")
			}

			if len(info.SupportedModels) == 0 {
				t.Error("No supported models")
			}

			// Note: We don't test actual API calls here since they would fail
			// with invalid URLs. The individual provider tests handle that.
		})
	}
}

func TestProviderWithNilConfig(t *testing.T) {
	retryConfig := DefaultRetryConfig()
	ctx := context.Background()

	provider := NewOpenAIProvider(nil, retryConfig)

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for nil config")
	}

	_, err = provider.ValidateResult(ctx, "test", "test", "test")
	if err == nil {
		t.Error("Expected error for nil config")
	}
}
