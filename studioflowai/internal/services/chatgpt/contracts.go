package services

import (
	"context"
)

// ChatGPTServicer defines the interface for ChatGPT service operations
type ChatGPTServicer interface {
	// Complete sends a completion request to the OpenAI API
	Complete(ctx context.Context, messages []ChatMessage, opts CompletionOptions) (*ChatResponse, error)

	// GetContent is a helper function that returns just the content from the first choice
	GetContent(ctx context.Context, messages []ChatMessage, opts CompletionOptions) (string, error)
}

// Ensure ChatGPTService implements ChatGPTServicer
var _ ChatGPTServicer = (*ChatGPTService)(nil)
