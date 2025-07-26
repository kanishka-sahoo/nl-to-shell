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

// OpenRouter Provider Tests

func TestOpenRouterProvider_GenerateCommand(t *testing.T) {
	// Mock server for OpenRouter API
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
		response := OpenRouterResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "openai/gpt-3.5-turbo",
			Choices: []OpenRouterChoice{
				{
					Index: 0,
					Message: OpenRouterMessage{
						Role:    "assistant",
						Content: `{"command": "ls -la", "explanation": "List files in long format", "confidence": 0.9, "alternatives": ["ls -l", "dir"]}`,
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenRouterUsage{
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
		DefaultModel: "openai/gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenRouterProvider(config, retryConfig)

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

func TestOpenRouterProvider_ValidateResult(t *testing.T) {
	// Mock server for OpenRouter API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock successful validation response
		response := OpenRouterResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "openai/gpt-3.5-turbo",
			Choices: []OpenRouterChoice{
				{
					Index: 0,
					Message: OpenRouterMessage{
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
		DefaultModel: "openai/gpt-3.5-turbo",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenRouterProvider(config, retryConfig)

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

func TestOpenRouterProvider_GetModel(t *testing.T) {
	// Test with custom model
	config := &types.ProviderConfig{
		DefaultModel: "anthropic/claude-3-opus",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenRouterProvider(config, retryConfig).(*OpenRouterProvider)

	model := provider.getModel()
	if model != "anthropic/claude-3-opus" {
		t.Errorf("Expected 'anthropic/claude-3-opus', got '%s'", model)
	}

	// Test with default model
	provider = NewOpenRouterProvider(&types.ProviderConfig{}, retryConfig).(*OpenRouterProvider)
	model = provider.getModel()
	if model != "openai/gpt-3.5-turbo" {
		t.Errorf("Expected default 'openai/gpt-3.5-turbo', got '%s'", model)
	}
}

func TestOpenRouterProvider_GetBaseURL(t *testing.T) {
	// Test with custom base URL
	config := &types.ProviderConfig{
		BaseURL: "https://custom.openrouter.ai/api/v1",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOpenRouterProvider(config, retryConfig).(*OpenRouterProvider)

	baseURL := provider.getBaseURL()
	if baseURL != "https://custom.openrouter.ai/api/v1" {
		t.Errorf("Expected custom URL, got '%s'", baseURL)
	}

	// Test with default base URL
	provider = NewOpenRouterProvider(&types.ProviderConfig{}, retryConfig).(*OpenRouterProvider)
	baseURL = provider.getBaseURL()
	if baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected default URL, got '%s'", baseURL)
	}
}

// Gemini Provider Tests

func TestGeminiProvider_GenerateCommand(t *testing.T) {
	// Mock server for Gemini API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Check for API key in URL
		if !strings.Contains(r.URL.RawQuery, "key=") {
			t.Error("Expected API key in URL query parameters")
		}

		// Mock successful response
		response := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiContent{
						Parts: []GeminiPart{
							{
								Text: `{"command": "ls -la", "explanation": "List files in long format", "confidence": 0.9, "alternatives": ["ls -l", "dir"]}`,
							},
						},
					},
					FinishReason: "STOP",
					Index:        0,
				},
			},
			UsageMetadata: &GeminiUsage{
				PromptTokenCount:     50,
				CandidatesTokenCount: 20,
				TotalTokenCount:      70,
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
		DefaultModel: "gemini-1.5-flash",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig)

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

func TestGeminiProvider_ValidateResult(t *testing.T) {
	// Mock server for Gemini API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock successful validation response
		response := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiContent{
						Parts: []GeminiPart{
							{
								Text: `{"is_correct": true, "explanation": "Command executed successfully and listed files as requested", "suggestions": ["Consider using ls -lah for human-readable sizes"], "correction": ""}`,
							},
						},
					},
					FinishReason: "STOP",
					Index:        0,
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
		DefaultModel: "gemini-1.5-flash",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig)

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

func TestGeminiProvider_GetModel(t *testing.T) {
	// Test with custom model
	config := &types.ProviderConfig{
		DefaultModel: "gemini-1.5-pro",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig).(*GeminiProvider)

	model := provider.getModel()
	if model != "gemini-1.5-pro" {
		t.Errorf("Expected 'gemini-1.5-pro', got '%s'", model)
	}

	// Test with default model
	provider = NewGeminiProvider(&types.ProviderConfig{}, retryConfig).(*GeminiProvider)
	model = provider.getModel()
	if model != "gemini-1.5-flash" {
		t.Errorf("Expected default 'gemini-1.5-flash', got '%s'", model)
	}
}

func TestGeminiProvider_GetBaseURL(t *testing.T) {
	// Test with custom base URL
	config := &types.ProviderConfig{
		BaseURL: "https://custom.googleapis.com/v1beta",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig).(*GeminiProvider)

	baseURL := provider.getBaseURL()
	if baseURL != "https://custom.googleapis.com/v1beta" {
		t.Errorf("Expected custom URL, got '%s'", baseURL)
	}

	// Test with default base URL
	provider = NewGeminiProvider(&types.ProviderConfig{}, retryConfig).(*GeminiProvider)
	baseURL = provider.getBaseURL()
	if baseURL != "https://generativelanguage.googleapis.com/v1beta" {
		t.Errorf("Expected default URL, got '%s'", baseURL)
	}
}

func TestGeminiProvider_EmptyCandidates(t *testing.T) {
	// Mock server that returns empty candidates
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GeminiResponse{
			Candidates: []GeminiCandidate{}, // Empty candidates
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		APIKey:       "test-api-key",
		BaseURL:      server.URL,
		DefaultModel: "gemini-1.5-flash",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for empty candidates")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
		// The error might be wrapped by retry logic, so check the cause
		if nlErr.Cause != nil {
			if causeErr, ok := nlErr.Cause.(*types.NLShellError); ok {
				if !strings.Contains(causeErr.Message, "no candidates") {
					t.Errorf("Expected 'no candidates' in cause error message, got '%s'", causeErr.Message)
				}
			}
		} else if !strings.Contains(nlErr.Message, "no candidates") {
			t.Errorf("Expected 'no candidates' in error message, got '%s'", nlErr.Message)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}

func TestGeminiProvider_EmptyParts(t *testing.T) {
	// Mock server that returns candidate with empty parts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiContent{
						Parts: []GeminiPart{}, // Empty parts
					},
					FinishReason: "STOP",
					Index:        0,
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
		DefaultModel: "gemini-1.5-flash",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewGeminiProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for empty parts")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
		// The error might be wrapped by retry logic, so check the cause
		if nlErr.Cause != nil {
			if causeErr, ok := nlErr.Cause.(*types.NLShellError); ok {
				if !strings.Contains(causeErr.Message, "no parts") {
					t.Errorf("Expected 'no parts' in cause error message, got '%s'", causeErr.Message)
				}
			}
		} else if !strings.Contains(nlErr.Message, "no parts") {
			t.Errorf("Expected 'no parts' in error message, got '%s'", nlErr.Message)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}
