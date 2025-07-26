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

// GeminiRequest represents the request structure for Gemini API
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent represents content in the Gemini request
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

// GeminiPart represents a part of the content
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig represents generation configuration
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiResponse represents the response structure from Gemini API
type GeminiResponse struct {
	Candidates    []GeminiCandidate `json:"candidates"`
	UsageMetadata *GeminiUsage      `json:"usageMetadata,omitempty"`
	Error         *GeminiError      `json:"error,omitempty"`
}

// GeminiCandidate represents a candidate in the Gemini response
type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

// GeminiUsage represents token usage information
type GeminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiError represents an error from the Gemini API
type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// generateCommandInternal implements the actual Gemini API call for command generation
func (p *GeminiProvider) generateCommandInternal(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	// Build the system prompt
	systemPrompt := p.buildSystemPrompt(context)

	// Create the request
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: systemPrompt + "\n\nUser request: " + prompt},
				},
				Role: "user",
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.1, // Low temperature for more deterministic responses
			MaxOutputTokens: 500, // Reasonable limit for shell commands
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

// validateResultInternal implements the actual Gemini API call for result validation
func (p *GeminiProvider) validateResultInternal(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	// Build the validation prompt
	validationPrompt := p.buildValidationPrompt(command, output, intent)

	// Create the request
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: "You are an expert system administrator. Analyze command execution results and determine if they match the user's intent. Respond with a JSON object containing 'is_correct' (boolean), 'explanation' (string), 'suggestions' (array of strings), and 'correction' (string if needed).\n\n" + validationPrompt},
				},
				Role: "user",
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 300,
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
func (p *GeminiProvider) buildSystemPrompt(context *types.Context) string {
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
func (p *GeminiProvider) buildValidationPrompt(command, output, intent string) string {
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
func (p *GeminiProvider) getModel() string {
	if p.config != nil && p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "gemini-1.5-flash" // Default model
}

// getBaseURL returns the base URL for the Gemini API
func (p *GeminiProvider) getBaseURL() string {
	if p.config != nil && p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "https://generativelanguage.googleapis.com/v1beta" // Default Gemini API URL
}

// makeAPICall makes an HTTP request to the Gemini API
func (p *GeminiProvider) makeAPICall(ctx context.Context, request GeminiRequest) (*GeminiResponse, error) {
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
	url := fmt.Sprintf("%s/models/%s:generateContent", p.getBaseURL(), p.getModel())
	if p.config != nil && p.config.APIKey != "" {
		url += "?key=" + p.config.APIKey
	}

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
			Message: fmt.Sprintf("Gemini API returned status %d", httpResp.StatusCode),
			Context: map[string]interface{}{
				"http_status":   httpResp.StatusCode,
				"response_body": string(responseBody),
			},
		}
	}

	// Parse response
	var response GeminiResponse
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
			Message: fmt.Sprintf("Gemini API error: %s", response.Error.Message),
			Context: map[string]interface{}{
				"error_code":   response.Error.Code,
				"error_status": response.Error.Status,
			},
		}
	}

	return &response, nil
}

// parseCommandResponse parses the Gemini response for command generation
func (p *GeminiProvider) parseCommandResponse(response *GeminiResponse) (*types.CommandResponse, error) {
	if len(response.Candidates) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no candidates in Gemini response",
		}
	}

	candidate := response.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no parts in Gemini candidate",
		}
	}

	content := candidate.Content.Parts[0].Text

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
		Explanation: "Generated by Google Gemini",
		Confidence:  0.8, // Default confidence when we can't parse JSON
	}, nil
}

// parseValidationResponse parses the Gemini response for result validation
func (p *GeminiProvider) parseValidationResponse(response *GeminiResponse) (*types.ValidationResponse, error) {
	if len(response.Candidates) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no candidates in Gemini response",
		}
	}

	candidate := response.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "no parts in Gemini candidate",
		}
	}

	content := candidate.Content.Parts[0].Text

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
