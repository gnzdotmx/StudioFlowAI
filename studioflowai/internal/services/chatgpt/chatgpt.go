package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// ChatGPTService provides a centralized way to interact with OpenAI's ChatGPT API
type ChatGPTService struct {
	apiKey string
}

// ChatMessage represents a message in the ChatGPT conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents an OpenAI API request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse represents an OpenAI API response
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatError represents an error from the OpenAI API
type ChatError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// CompletionOptions contains the parameters for a ChatGPT completion request
type CompletionOptions struct {
	Model            string
	Temperature      float64
	MaxTokens        int
	RequestTimeoutMS int
}

// NewChatGPTService creates a new ChatGPT service instance
func NewChatGPTService() (*ChatGPTService, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable is not set")
	}

	return &ChatGPTService{
		apiKey: apiKey,
	}, nil
}

// Complete sends a completion request to the OpenAI API
func (s *ChatGPTService) Complete(ctx context.Context, messages []ChatMessage, opts CompletionOptions) (*ChatResponse, error) {
	// Create a timeout context if RequestTimeoutMS is specified
	if opts.RequestTimeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.RequestTimeoutMS)*time.Millisecond)
		defer cancel()
	}

	// Create the request body
	reqBody := ChatRequest{
		Model:       opts.Model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.LogWarning("Failed to close response body: %v", err)
		}
	}()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		var chatError ChatError
		if err := json.Unmarshal(respBody, &chatError); err == nil {
			return nil, fmt.Errorf("API error: %s", chatError.Error.Message)
		}
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if there are any choices in the response
	if len(chatResp.Choices) == 0 {
		return nil, errors.New("no response from ChatGPT")
	}

	return &chatResp, nil
}

// GetContent is a helper function that returns just the content from the first choice
func (s *ChatGPTService) GetContent(ctx context.Context, messages []ChatMessage, opts CompletionOptions) (string, error) {
	resp, err := s.Complete(ctx, messages, opts)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

// IsAPIKeySet checks if the OpenAI API key is set in the environment
func IsAPIKeySet() bool {
	return os.Getenv("OPENAI_API_KEY") != ""
}

// ValidateAPIKey checks if the API key is set and returns an error if it's not
func ValidateAPIKey() error {
	if !IsAPIKeySet() {
		return errors.New("OPENAI_API_KEY environment variable is not set")
	}
	return nil
}
