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

// AnthropicRequest represents the request structure for Anthropic API
type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

// AnthropicMessage represents a message in the Anthropic chat format
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents the response structure from Anthropic API
type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence"`
	Usage        AnthropicUsage     `json:"usage"`
	Error        *AnthropicError    `json:"error,omitempty"`
}

// AnthropicContent represents content in the Anthropic response
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AnthropicUsage represents token usage information
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicError represents an error from the Anthropic API
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// generateCommandInternal implements the actual Anthropic API call for command generation
func (p *AnthropicProvider) generateCommandInternal(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	// Build the system prompt
	systemPrompt := p.buildSystemPrompt(context)

	// Create the request
	request := AnthropicRequest{
		Model:     p.getModel(),
		MaxTokens: 500, // Reasonable limit for shell commands
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Make the API call
	response, err := p.makeAPICall(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the response
	return p.parseCommandResponse(response)
}

// validateResultInternal implements the actual Anthropic API call for result validation
func (p *AnthropicProvider) validateResultInternal(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	// Build the validation prompt
	validationPrompt := p.buildValidationPrompt(command, output, intent)

	// Create the request
	request := AnthropicRequest{
		Model:     p.getModel(),
		MaxTokens: 300,
		System:    "You are an expert system administrator. Analyze command execution results and determine if they match the user's intent. Respond with a JSON object containing 'is_correct' (boolean), 'explanation' (string), 'suggestions' (array of strings), and 'correction' (string if needed).",
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: validationPrompt,
			},
		},
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
func (p *AnthropicProvider) buildSystemPrompt(context *types.Context) string {
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
func (p *AnthropicProvider) buildValidationPrompt(command, output, intent string) string {
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
func (p *AnthropicProvider) getModel() string {
	if p.config != nil && p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "claude-3-haiku-20240307" // Default model
}

// getBaseURL returns the base URL for the Anthropic API
func (p *AnthropicProvider) getBaseURL() string {
	if p.config != nil && p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "https://api.anthropic.com" // Default Anthropic API URL
}

// makeAPICall makes an HTTP request to the Anthropic API
func (p *AnthropicProvider) makeAPICall(ctx context.Context, request AnthropicRequest) (*AnthropicResponse, error) {
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
	url := p.getBaseURL() + "/v1/messages"
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
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if p.config != nil && p.config.APIKey != "" {
		httpReq.Header.Set("x-api-key", p.config.APIKey)
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
			Message: fmt.Sprintf("Anthropic API returned status %d", httpResp.StatusCode),
			Context: map[string]interface{}{
				"http_status":   httpResp.StatusCode,
				"response_body": string(responseBody),
			},
		}
	}

	// Parse response
	var response AnthropicResponse
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
			Message: fmt.Sprintf("Anthropic API error: %s", response.Error.Message),
			Context: map[string]interface{}{
				"error_type": response.Error.Type,
			},
		}
	}

	return &response, nil
}

// parseCommandResponse parses the Anthropic response for command generation
func (p *AnthropicProvider) parseCommandResponse(response *AnthropicResponse) (*types.CommandResponse, error) {
	if len(response.Content) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no content in Anthropic response",
		}
	}

	content := response.Content[0].Text

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
		Explanation: "Generated by Anthropic Claude",
		Confidence:  0.8, // Default confidence when we can't parse JSON
	}, nil
}

// parseValidationResponse parses the Anthropic response for result validation
func (p *AnthropicProvider) parseValidationResponse(response *AnthropicResponse) (*types.ValidationResponse, error) {
	if len(response.Content) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no content in Anthropic response",
		}
	}

	content := response.Content[0].Text

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
