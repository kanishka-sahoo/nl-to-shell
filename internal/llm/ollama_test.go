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

func TestOllamaProvider_GenerateCommand(t *testing.T) {
	// Mock server for Ollama API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Mock successful response
		response := OllamaResponse{
			Model:     "llama3.2",
			CreatedAt: "2024-01-01T12:00:00Z",
			Response:  `{"command": "ls -la", "explanation": "List files in long format", "confidence": 0.9, "alternatives": ["ls -l", "dir"]}`,
			Done:      true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test configuration
	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

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

func TestOllamaProvider_ValidateResult(t *testing.T) {
	// Mock server for Ollama API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock successful validation response
		response := OllamaResponse{
			Model:     "llama3.2",
			CreatedAt: "2024-01-01T12:00:00Z",
			Response:  `{"is_correct": true, "explanation": "Command executed successfully and listed files as requested", "suggestions": ["Consider using ls -lah for human-readable sizes"], "correction": ""}`,
			Done:      true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test configuration
	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

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

func TestOllamaProvider_APIError(t *testing.T) {
	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OllamaResponse{
			Model:     "llama3.2",
			CreatedAt: "2024-01-01T12:00:00Z",
			Response:  "",
			Done:      true,
			Error:     "Model not found",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for API error response")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
		// The error might be wrapped by retry logic, so check the cause
		if nlErr.Cause != nil {
			if causeErr, ok := nlErr.Cause.(*types.NLShellError); ok {
				if !strings.Contains(causeErr.Message, "Model not found") {
					t.Errorf("Expected 'Model not found' in cause error message, got '%s'", causeErr.Message)
				}
			}
		} else if !strings.Contains(nlErr.Message, "Model not found") {
			t.Errorf("Expected 'Model not found' in error message, got '%s'", nlErr.Message)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}

func TestOllamaProvider_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	config := &types.ProviderConfig{
		BaseURL:      "http://invalid-url-that-does-not-exist.com",
		DefaultModel: "llama3.2",
		Timeout:      1 * time.Second, // Short timeout
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

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

func TestOllamaProvider_FallbackParsing(t *testing.T) {
	// Mock server that returns non-JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OllamaResponse{
			Model:     "llama3.2",
			CreatedAt: "2024-01-01T12:00:00Z",
			Response:  "ls -la", // Plain text, not JSON
			Done:      true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

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

	if response.Explanation != "Generated by Ollama" {
		t.Errorf("Expected default explanation, got '%s'", response.Explanation)
	}
}

func TestOllamaProvider_CheckModelAvailability(t *testing.T) {
	// Mock server for model availability check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			// Mock models response
			response := OllamaModelsResponse{
				Models: []OllamaModelInfo{
					{
						Name:       "llama3.2:latest",
						ModifiedAt: "2024-01-01T12:00:00Z",
						Size:       1000000,
						Digest:     "abc123",
					},
					{
						Name:       "codellama:latest",
						ModifiedAt: "2024-01-01T12:00:00Z",
						Size:       2000000,
						Digest:     "def456",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig).(*OllamaProvider)

	ctx := context.Background()

	// Test model availability check - should succeed
	err := provider.CheckModelAvailability(ctx)
	if err != nil {
		t.Errorf("Expected model to be available, got error: %v", err)
	}

	// Test with unavailable model
	config.DefaultModel = "nonexistent-model"
	provider = NewOllamaProvider(config, retryConfig).(*OllamaProvider)

	err = provider.CheckModelAvailability(ctx)
	if err == nil {
		t.Error("Expected error for unavailable model")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeConfiguration {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeConfiguration, nlErr.Type)
		}
		if !strings.Contains(nlErr.Message, "not found") {
			t.Errorf("Expected 'not found' in error message, got '%s'", nlErr.Message)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}

func TestOllamaProvider_GetModel(t *testing.T) {
	// Test with custom model
	config := &types.ProviderConfig{
		DefaultModel: "codellama",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig).(*OllamaProvider)

	model := provider.getModel()
	if model != "codellama" {
		t.Errorf("Expected 'codellama', got '%s'", model)
	}

	// Test with default model
	provider = NewOllamaProvider(&types.ProviderConfig{}, retryConfig).(*OllamaProvider)
	model = provider.getModel()
	if model != "llama3.2" {
		t.Errorf("Expected default 'llama3.2', got '%s'", model)
	}
}

func TestOllamaProvider_GetBaseURL(t *testing.T) {
	// Test with custom base URL
	config := &types.ProviderConfig{
		BaseURL: "http://custom-ollama:11434",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig).(*OllamaProvider)

	baseURL := provider.getBaseURL()
	if baseURL != "http://custom-ollama:11434" {
		t.Errorf("Expected custom URL, got '%s'", baseURL)
	}

	// Test with default base URL
	provider = NewOllamaProvider(&types.ProviderConfig{}, retryConfig).(*OllamaProvider)
	baseURL = provider.getBaseURL()
	if baseURL != "http://localhost:11434" {
		t.Errorf("Expected default URL, got '%s'", baseURL)
	}
}

func TestOllamaProvider_BuildSystemPrompt(t *testing.T) {
	config := &types.ProviderConfig{
		DefaultModel: "llama3.2",
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig).(*OllamaProvider)

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

	prompt := provider.buildSystemPrompt(context)

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

func TestOllamaProvider_HTTPError(t *testing.T) {
	// Mock server that returns HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	config := &types.ProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}

	retryConfig := DefaultRetryConfig()
	provider := NewOllamaProvider(config, retryConfig)

	ctx := context.Background()

	_, err := provider.GenerateCommand(ctx, "test", &types.Context{})
	if err == nil {
		t.Error("Expected error for HTTP error response")
	}

	if nlErr, ok := err.(*types.NLShellError); ok {
		if nlErr.Type != types.ErrTypeProvider {
			t.Errorf("Expected error type %v, got %v", types.ErrTypeProvider, nlErr.Type)
		}
		// The error might be wrapped by retry logic, so check the cause
		if nlErr.Cause != nil {
			if causeErr, ok := nlErr.Cause.(*types.NLShellError); ok {
				if !strings.Contains(causeErr.Message, "status 500") {
					t.Errorf("Expected 'status 500' in cause error message, got '%s'", causeErr.Message)
				}
			}
		} else if !strings.Contains(nlErr.Message, "status 500") {
			t.Errorf("Expected 'status 500' in error message, got '%s'", nlErr.Message)
		}
	} else {
		t.Error("Expected NLShellError")
	}
}
