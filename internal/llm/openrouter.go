package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// OpenRouterRequest represents the request structure for OpenRouter API
type OpenRouterRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenRouterMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream"`
}

// OpenRouterMessage represents a message in the OpenRouter chat format
type OpenRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenRouterResponse represents the response structure from OpenRouter API
type OpenRouterResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenRouterChoice `json:"choices"`
	Usage   OpenRouterUsage    `json:"usage"`
	Error   *OpenRouterError   `json:"error,omitempty"`
}

// OpenRouterChoice represents a choice in the OpenRouter response
type OpenRouterChoice struct {
	Index        int               `json:"index"`
	Message      OpenRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenRouterUsage represents token usage information
type OpenRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenRouterError represents an error from the OpenRouter API
type OpenRouterError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// generateCommandInternal implements the actual OpenRouter API call for command generation
func (p *OpenRouterProvider) generateCommandInternal(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	// Build the system prompt
	systemPrompt := p.buildSystemPrompt(context)

	// Create the request
	request := OpenRouterRequest{
		Model: p.getModel(),
		Messages: []OpenRouterMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.1, // Low temperature for more deterministic responses
		MaxTokens:   500, // Reasonable limit for shell commands
		Stream:      false,
	}

	// Make the API call
	response, err := p.makeAPICall(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the response
	return p.parseCommandResponse(response)
}

// validateResultInternal implements the actual OpenRouter API call for result validation
func (p *OpenRouterProvider) validateResultInternal(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	// Build the validation prompt
	validationPrompt := p.buildValidationPrompt(command, output, intent)

	// Create the request
	request := OpenRouterRequest{
		Model: p.getModel(),
		Messages: []OpenRouterMessage{
			{
				Role:    "system",
				Content: "You are an expert system administrator. Analyze command execution results and determine if they match the user's intent. Respond with a JSON object containing 'is_correct' (boolean), 'explanation' (string), 'suggestions' (array of strings), and 'correction' (string if needed).",
			},
			{
				Role:    "user",
				Content: validationPrompt,
			},
		},
		Temperature: 0.1,
		MaxTokens:   300,
		Stream:      false,
	}

	// Make the API call
	response, err := p.makeAPICall(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the validation response
	return p.parseValidationResponse(response)
}

// buildSystemPrompt creates the system prompt for command generation
func (p *OpenRouterProvider) buildSystemPrompt(context *types.Context) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert shell command generator. Convert natural language requests into safe, accurate shell commands.\n\n")
	prompt.WriteString("Rules:\n")
	prompt.WriteString("1. Generate only the shell command, no explanations unless asked\n")
	prompt.WriteString("2. Prefer safe, non-destructive commands\n")
	prompt.WriteString("3. Use standard Unix/Linux commands when possible\n")
	prompt.WriteString("4. Consider the current context when generating commands\n\n")

	if context != nil {
		if context.WorkingDirectory != "" {
			prompt.WriteString(fmt.Sprintf("Current directory: %s\n", context.WorkingDirectory))
		}

		if len(context.Files) > 0 {
			prompt.WriteString("Files in current directory:\n")
			for _, file := range context.Files {
				if file.IsDir {
					prompt.WriteString(fmt.Sprintf("  %s/ (directory)\n", file.Name))
				} else {
					prompt.WriteString(fmt.Sprintf("  %s (file, %d bytes)\n", file.Name, file.Size))
				}
			}
		}

		if context.GitInfo != nil && context.GitInfo.IsRepository {
			prompt.WriteString(fmt.Sprintf("Git repository: branch '%s'", context.GitInfo.CurrentBranch))
			if context.GitInfo.HasUncommittedChanges {
				prompt.WriteString(" (has uncommitted changes)")
			}
			prompt.WriteString("\n")
		}
	}

	prompt.WriteString("\nRespond with a JSON object containing:\n")
	prompt.WriteString("- 'command': the shell command\n")
	prompt.WriteString("- 'explanation': brief explanation of what the command does\n")
	prompt.WriteString("- 'confidence': confidence level (0.0-1.0)\n")
	prompt.WriteString("- 'alternatives': array of alternative commands (optional)\n")

	return prompt.String()
}

// buildValidationPrompt creates the prompt for result validation
func (p *OpenRouterProvider) buildValidationPrompt(command, output, intent string) string {
	return fmt.Sprintf(`Analyze this command execution:

Intent: %s
Command: %s
Output: %s

Determine if the command output matches the user's intent. Consider:
1. Did the command execute successfully?
2. Does the output indicate the intended action was completed?
3. Are there any error messages or warnings?
4. Does the result align with what the user wanted to achieve?

Respond with JSON containing:
- is_correct: boolean indicating if the result matches intent
- explanation: detailed explanation of your analysis
- suggestions: array of improvement suggestions (if any)
- correction: corrected command if the original was wrong (optional)`, intent, command, output)
}

// getModel returns the model to use for this provider
func (p *OpenRouterProvider) getModel() string {
	if p.config != nil && p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "openai/gpt-3.5-turbo" // Default model
}

// getBaseURL returns the base URL for the OpenRouter API
func (p *OpenRouterProvider) getBaseURL() string {
	if p.config != nil && p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "https://openrouter.ai/api/v1" // Default OpenRouter API URL
}

// makeAPICall makes an HTTP request to the OpenRouter API
func (p *OpenRouterProvider) makeAPICall(ctx context.Context, request OpenRouterRequest) (*OpenRouterResponse, error) {
	// Serialize the request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to serialize request",
			Cause:   err,
		}
	}

	// Create HTTP request
	url := p.getBaseURL() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to create HTTP request",
			Cause:   err,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if p.config != nil && p.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	// Make the request
	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to make HTTP request",
			Cause:   err,
		}
	}
	defer httpResp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to read response body",
			Cause:   err,
		}
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: fmt.Sprintf("OpenRouter API returned status %d", httpResp.StatusCode),
			Context: map[string]interface{}{
				"http_status":   httpResp.StatusCode,
				"response_body": string(responseBody),
			},
		}
	}

	// Parse response
	var response OpenRouterResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to parse response",
			Cause:   err,
			Context: map[string]interface{}{
				"response_body": string(responseBody),
			},
		}
	}

	// Check for API errors
	if response.Error != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: fmt.Sprintf("OpenRouter API error: %s", response.Error.Message),
			Context: map[string]interface{}{
				"error_type": response.Error.Type,
				"error_code": response.Error.Code,
			},
		}
	}

	return &response, nil
}

// parseCommandResponse parses the OpenRouter response for command generation
func (p *OpenRouterProvider) parseCommandResponse(response *OpenRouterResponse) (*types.CommandResponse, error) {
	if len(response.Choices) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no choices in OpenRouter response",
		}
	}

	content := response.Choices[0].Message.Content

	// Try to parse as JSON first
	var jsonResponse struct {
		Command      string   `json:"command"`
		Explanation  string   `json:"explanation"`
		Confidence   float64  `json:"confidence"`
		Alternatives []string `json:"alternatives"`
	}

	if err := json.Unmarshal([]byte(content), &jsonResponse); err == nil {
		return &types.CommandResponse{
			Command:      jsonResponse.Command,
			Explanation:  jsonResponse.Explanation,
			Confidence:   jsonResponse.Confidence,
			Alternatives: jsonResponse.Alternatives,
		}, nil
	}

	// Fallback: treat the entire content as the command
	return &types.CommandResponse{
		Command:     strings.TrimSpace(content),
		Explanation: "Generated by OpenRouter",
		Confidence:  0.8, // Default confidence when we can't parse JSON
	}, nil
}

// parseValidationResponse parses the OpenRouter response for result validation
func (p *OpenRouterProvider) parseValidationResponse(response *OpenRouterResponse) (*types.ValidationResponse, error) {
	if len(response.Choices) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no choices in OpenRouter response",
		}
	}

	content := response.Choices[0].Message.Content

	// Try to parse as JSON
	var jsonResponse struct {
		IsCorrect   bool     `json:"is_correct"`
		Explanation string   `json:"explanation"`
		Suggestions []string `json:"suggestions"`
		Correction  string   `json:"correction"`
	}

	if err := json.Unmarshal([]byte(content), &jsonResponse); err == nil {
		return &types.ValidationResponse{
			IsCorrect:   jsonResponse.IsCorrect,
			Explanation: jsonResponse.Explanation,
			Suggestions: jsonResponse.Suggestions,
			Correction:  jsonResponse.Correction,
		}, nil
	}

	// Fallback: treat the content as explanation
	return &types.ValidationResponse{
		IsCorrect:   false, // Conservative default
		Explanation: strings.TrimSpace(content),
	}, nil
}
