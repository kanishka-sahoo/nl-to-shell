package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestOpenAIProvider_GenerateCommand(t *testing.T) {
	// Mock server for OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		// Mock successful response
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-3.5-turbo",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: `{"command": "ls -la", "explanation": "List files in long format", "confidence": 0.9, "alternatives": ["ls -l", "dir"]}`,
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     50,
				CompletionTokens: 20,
				TotalTokens:      70,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test configuration
	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()
	context := &types.Context{
		WorkingDirectory: "/home/user",
		Files: []types.FileInfo{
			{Name: "test.txt", IsDir: false, Size: 100},
			{Name: "docs", IsDir: true},
		},
	}

	// Test command generation
	response, err := provider.GenerateCommand(ctx, "list all files", context)
	if err != nil {
		t.Fatalf("GenerateCommand failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", response.Command)
	}

	if response.Explanation != "List files in long format" {
		t.Errorf("Expected explanation 'List files in long format', got '%s'", response.Explanation)
	}

	if response.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %f", response.Confidence)
	}

	if len(response.Alternatives) != 2 {
		t.Errorf("Expected 2 alternatives, got %d", len(response.Alternatives))
	}
}

func TestOpenAIProvider_ValidateResult(t *testing.T) {
	// Mock server for OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock successful validation response
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-3.5-turbo",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: `{"is_correct": true, "explanation": "Command executed successfully and listed files as requested", "suggestions": ["Consider using ls -lah for human-readable sizes"], "correction": ""}`,
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test configuration
	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()

	// Test result validation
	response, err := provider.ValidateResult(ctx, "ls -la", "total 8\n-rw-r--r-- 1 user user 100 Jan 1 12:00 test.txt\ndrwxr-xr-x 2 user user 4096 Jan 1 12:00 docs", "list all files")
	if err != nil {
		t.Fatalf("ValidateResult failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if !response.IsCorrect {
		t.Error("Expected IsCorrect to be true")
	}

	if response.Explanation != "Command executed successfully and listed files as requested" {
		t.Errorf("Unexpected explanation: %s", response.Explanation)
	}

	if len(response.Suggestions) != 1 {
		t.Errorf("Expected 1 suggestion, got %d", len(response.Suggestions))
	}
}

func TestOpenAIProvider_APIError(t *testing.T) {
	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		response := OpenAIResponse{
			Error: &OpenAIError{
				Message: "Invalid request",
				Type:    "invalid_request_error",
				Code:    "invalid_request",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for API error response")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}

func TestOpenAIProvider_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      "http://invalid-url-that-does-not-exist.com",
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      1 * time.Second, // Short timeout
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for network failure")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}

func TestOpenAIProvider_RateLimitRetry(t *testing.T) {
	attempts := 0

	// Mock server that returns rate limit error first, then success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		if attempts == 1 {
			// First attempt: rate limit error
			w.WriteHeader(http.StatusTooManyRequests)
			response := OpenAIResponse{
				Error: &OpenAIError{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_error",
					Code:    "rate_limit_exceeded",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Second attempt: success
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-3.5-turbo",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: `{"command": "echo success", "explanation": "Success after retry", "confidence": 1.0}`,
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	// Use custom retry config with shorter delays for testing
	retryConfig := &RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		RetryableErrors: []ErrorMatcher{
			IsRateLimitError,
		},
	}

	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()

	response, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	if response.Command != "echo success" {
		t.Errorf("Expected 'echo success', got '%s'", response.Command)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestOpenAIProvider_FallbackParsing(t *testing.T) {
	// Mock server that returns non-JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-3.5-turbo",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "ls -la", // Plain text, not JSON
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig)

	ctx := context.Background()

	response, err := provider.GenerateCommand(ctx, "list files", &types.Context{})
	if err != nil {
		t.Fatalf("GenerateCommand failed: %v", err)
	}

	if response.Command != "ls -la" {
		t.Errorf("Expected 'ls -la', got '%s'", response.Command)
	}

	if response.Confidence != 0.8 {
		t.Errorf("Expected default confidence 0.8, got %f", response.Confidence)
	}

	if response.Explanation != "Generated by OpenAI" {
		t.Errorf("Expected default explanation, got '%s'", response.Explanation)
	}
}

func TestOpenAIProvider_BuildSystemPrompt(t *testing.T) {
	config := &types.ProviderConfig{
		APIKey:       "test-key",
		DefaultModel: "gpt-3.5-turbo",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig).(*OpenAIProvider)

	context := &types.Context{
		WorkingDirectory: "/home/user/project",
		Files: []types.FileInfo{
			{Name: "main.go", IsDir: false, Size: 1024},
			{Name: "src", IsDir: true},
		},
		GitInfo: &types.GitContext{
			IsRepository:          true,
			CurrentBranch:         "main",
			HasUncommittedChanges: true,
		},
	}

	prompt := provider.promptBuilder.BuildSystemPrompt(context)

	// Check that the prompt contains expected elements
	if !strings.Contains(prompt, "expert shell command generator") {
		t.Error("Prompt should contain role description")
	}

	if !strings.Contains(prompt, "/home/user/project") {
		t.Error("Prompt should contain working directory")
	}

	if !strings.Contains(prompt, "main.go") {
		t.Error("Prompt should contain file information")
	}

	if !strings.Contains(prompt, "branch 'main'") {
		t.Error("Prompt should contain git information")
	}

	if !strings.Contains(prompt, "has uncommitted changes") {
		t.Error("Prompt should contain git status")
	}

	if !strings.Contains(prompt, "JSON object") {
		t.Error("Prompt should specify JSON response format")
	}
}

func TestOpenAIProvider_GetModel(t *testing.T) {
	// Test with custom model
	config := &types.ProviderConfig{
		DefaultModel: "gpt-4",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig).(*OpenAIProvider)

	model := provider.getModel()
	if model != "gpt-4" {
		t.Errorf("Expected 'gpt-4', got '%s'", model)
	}

	// Test with default model
	provider = NewOpenAIProvider(&types.ProviderConfig{}, retryConfig).(*OpenAIProvider)
	model = provider.getModel()
	if model != "gpt-3.5-turbo" {
		t.Errorf("Expected default 'gpt-3.5-turbo', got '%s'", model)
	}
}

func TestOpenAIProvider_GetBaseURL(t *testing.T) {
	// Test with custom base URL
	config := &types.ProviderConfig{
		BaseURL: "https://custom.openai.com/v1",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenAIProvider(config, retryConfig).(*OpenAIProvider)

	baseURL := provider.getBaseURL()
	if baseURL != "https://custom.openai.com/v1" {
		t.Errorf("Expected custom URL, got '%s'", baseURL)
	}

	// Test with default base URL
	provider = NewOpenAIProvider(&types.ProviderConfig{}, retryConfig).(*OpenAIProvider)
	baseURL = provider.getBaseURL()
	if baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default URL, got '%s'", baseURL)
	}
}
