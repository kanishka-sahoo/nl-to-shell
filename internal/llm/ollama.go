package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// OllamaRequest represents the request structure for Ollama API
type OllamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options *OllamaOptions `json:"options,omitempty"`
}

// OllamaOptions represents generation options for Ollama
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// OllamaResponse represents the response structure from Ollama API
type OllamaResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
	Error              string `json:"error,omitempty"`
}

// OllamaModelInfo represents information about an Ollama model
type OllamaModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

// OllamaModelsResponse represents the response from the models endpoint
type OllamaModelsResponse struct {
	Models []OllamaModelInfo `json:"models"`
}

// generateCommandInternal implements the actual Ollama API call for command generation
func (p *OllamaProvider) generateCommandInternal(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	// Build the system prompt
	systemPrompt := p.promptBuilder.BuildSystemPrompt(context)

	// Create the request
	request := OllamaRequest{
		Model:  p.getModel(),
		Prompt: systemPrompt + "\n\nUser request: " + prompt,
		Stream: false,
		Options: &OllamaOptions{
			Temperature: 0.1, // Low temperature for more deterministic responses
			NumPredict:  500, // Reasonable limit for shell commands
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

// validateResultInternal implements the actual Ollama API call for result validation
func (p *OllamaProvider) validateResultInternal(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	// Build the validation prompt
	validationPrompt := p.promptBuilder.BuildValidationPrompt(command, output, intent)

	// Create the request
	request := OllamaRequest{
		Model:  p.getModel(),
		Prompt: p.promptBuilder.BuildValidationSystemPrompt() + "\n\n" + validationPrompt,
		Stream: false,
		Options: &OllamaOptions{
			Temperature: 0.1,
			NumPredict:  300,
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

// getModel returns the model to use for this provider
func (p *OllamaProvider) getModel() string {
	if p.config != nil && p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "llama3.2" // Default model
}

// getBaseURL returns the base URL for the Ollama API
func (p *OllamaProvider) getBaseURL() string {
	if p.config != nil && p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "http://localhost:11434" // Default Ollama API URL
}

// CheckModelAvailability checks if the specified model is available
func (p *OllamaProvider) CheckModelAvailability(ctx context.Context) error {
	url := p.getBaseURL() + "/api/tags"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to create HTTP request for model check",
			Cause:   err,
		}
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to check model availability",
			Cause:   err,
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: fmt.Sprintf("Ollama API returned status %d when checking models", httpResp.StatusCode),
			Context: map[string]interface{}{
				"http_status": httpResp.StatusCode,
			},
		}
	}

	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to read models response",
			Cause:   err,
		}
	}

	var modelsResponse OllamaModelsResponse
	if err := json.Unmarshal(responseBody, &modelsResponse); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to parse models response",
			Cause:   err,
		}
	}

	// Check if the requested model is available
	requestedModel := p.getModel()
	for _, model := range modelsResponse.Models {
		if strings.HasPrefix(model.Name, requestedModel) {
			return nil // Model found
		}
	}

	return &types.NLShellError{
		Type:    types.ErrTypeConfiguration,
		Message: fmt.Sprintf("model '%s' not found in Ollama", requestedModel),
		Context: map[string]interface{}{
			"requested_model": requestedModel,
			"available_models": func() []string {
				names := make([]string, len(modelsResponse.Models))
				for i, model := range modelsResponse.Models {
					names[i] = model.Name
				}
				return names
			}(),
		},
	}
}

// makeAPICall makes an HTTP request to the Ollama API
func (p *OllamaProvider) makeAPICall(ctx context.Context, request OllamaRequest) (*OllamaResponse, error) {
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
	url := p.getBaseURL() + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: "failed to create HTTP request",
			Cause:   err,
		}
	}

	// Set headers (Ollama doesn't require authentication by default)
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
			Message: fmt.Sprintf("Ollama API returned status %d", httpResp.StatusCode),
			Context: map[string]interface{}{
				"http_status":   httpResp.StatusCode,
				"response_body": string(responseBody),
			},
		}
	}

	// Parse response
	var response OllamaResponse
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
	if response.Error != "" {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeProvider,
			Message: fmt.Sprintf("Ollama API error: %s", response.Error),
		}
	}

	return &response, nil
}

// parseCommandResponse parses the Ollama response for command generation
func (p *OllamaProvider) parseCommandResponse(response *OllamaResponse) (*types.CommandResponse, error) {
	content := response.Response

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
		Explanation: "Generated by Ollama",
		Confidence:  0.8, // Default confidence when we can't parse JSON
	}, nil
}

// parseValidationResponse parses the Ollama response for result validation
func (p *OllamaProvider) parseValidationResponse(response *OllamaResponse) (*types.ValidationResponse, error) {
	content := response.Response

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
