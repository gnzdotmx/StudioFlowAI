package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"

	"gopkg.in/yaml.v3"
)

// Module implements the ChatGPT correction functionality
type Module struct{}

// Params contains the parameters for ChatGPT correction
type Params struct {
	Input            string  `json:"input"`            // Path to input transcript file
	Output           string  `json:"output"`           // Path to output directory
	InputFileName    string  `json:"inputFileName"`    // Specific input file name to process
	OutputFileName   string  `json:"outputFileName"`   // Custom output file name (without extension)
	PromptTemplate   string  `json:"promptTemplate"`   // Path to prompt template file
	OutputSuffix     string  `json:"outputSuffix"`     // Suffix for corrected files (default: "_corrected")
	Model            string  `json:"model"`            // OpenAI model to use (default: "gpt-4o")
	Temperature      float64 `json:"temperature"`      // Model temperature (default: 0.1)
	MaxTokens        int     `json:"maxTokens"`        // Maximum tokens for the response (default: 4000)
	TargetLanguage   string  `json:"targetLanguage"`   // Target language for corrections (default: "English")
	RequestTimeoutMS int     `json:"requestTimeoutMs"` // API request timeout in milliseconds (default: 300000)
	ChunkSize        int     `json:"chunkSize"`        // Size of transcript chunks in tokens (default: 120000)
}

// ChatRequest represents an OpenAI API request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatMessage represents a message in the ChatGPT conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// New creates a new ChatGPT module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "chatgpt"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, p.InputFileName); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Check if the API key is set - just warn but don't error
	if os.Getenv("OPENAI_API_KEY") == "" {
		utils.LogWarning("OPENAI_API_KEY environment variable is not set. Original text will be used.")
	}

	// Check if the prompt template exists
	if p.PromptTemplate != "" {
		if _, err := os.Stat(p.PromptTemplate); os.IsNotExist(err) {
			return fmt.Errorf("prompt template %s does not exist", p.PromptTemplate)
		}
	}

	return nil
}

// Execute processes transcript files using the ChatGPT API
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.OutputSuffix == "" {
		p.OutputSuffix = "_corrected"
	}
	if p.Model == "" {
		p.Model = "gpt-4o"
	}
	if p.Temperature == 0 {
		p.Temperature = 0.1
	}
	if p.MaxTokens == 0 {
		p.MaxTokens = 4000
	}
	if p.TargetLanguage == "" {
		p.TargetLanguage = "English"
	}
	if p.RequestTimeoutMS == 0 {
		p.RequestTimeoutMS = 300000 // 5 minutes default
	}
	if p.ChunkSize == 0 {
		p.ChunkSize = 120000 // Default chunk size for GPT-4
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load the prompt template
	promptTemplate, err := m.loadPromptTemplate(p.PromptTemplate)
	if err != nil {
		return fmt.Errorf("failed to load prompt template: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Verify input exists at execution time (now that previous steps have completed)
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("input must be a file, not a directory: %s", resolvedInput)
	}

	// Determine output file name
	var outputPath string
	if p.OutputFileName != "" {
		outputPath = filepath.Join(p.Output, p.OutputFileName+".txt")
	} else {
		baseFilename := filepath.Base(resolvedInput)
		baseFilename = baseFilename[:len(baseFilename)-len(filepath.Ext(baseFilename))]
		outputPath = filepath.Join(p.Output, baseFilename+p.OutputSuffix+".txt")
	}

	if err := m.processFile(ctx, resolvedInput, outputPath, promptTemplate, p); err != nil {
		return err
	}

	fmt.Println(utils.Success(fmt.Sprintf("Corrected file %s -> %s", resolvedInput, outputPath)))
	return nil
}

// loadPromptTemplate loads the prompt template from a file
func (m *Module) loadPromptTemplate(templatePath string) (string, error) {
	// If no template path is provided, use the default prompt
	if templatePath == "" {
		return "You are a helpful assistant that corrects transcript errors. " +
			"Please fix any transcription mistakes, especially words that might have been " +
			"misinterpreted due to multilingual context. For example, the word 'haiti' might actually be 'IT' " +
			"when the context is about technology in Spanish. Keep the meaning intact and improve readability. " +
			"Here is the transcript text to correct:", nil
	}

	// Read the template file
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt template: %w", err)
	}

	// Check file extension to determine format
	ext := strings.ToLower(filepath.Ext(templatePath))

	// Process YAML format
	if ext == ".yaml" || ext == ".yml" {
		return m.formatYAMLPrompt(data)
	}

	// Default: treat as plain text/markdown
	return string(data), nil
}

// formatYAMLPrompt parses a YAML prompt template and formats it as text
func (m *Module) formatYAMLPrompt(yamlData []byte) (string, error) {
	var promptData map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &promptData); err != nil {
		return "", fmt.Errorf("failed to parse YAML prompt template: %w", err)
	}

	var result strings.Builder

	// Add title if present
	if title, ok := promptData["title"].(string); ok {
		result.WriteString("# " + title + "\n\n")
	}

	// Add role if present
	if role, ok := promptData["role"].(string); ok {
		result.WriteString("You are a " + role + ". ")
	}

	// Process context section
	if context, ok := promptData["context"].(map[string]interface{}); ok {
		if desc, ok := context["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if sources, ok := context["error_sources"].([]interface{}); ok {
			for _, source := range sources {
				if str, ok := source.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
			result.WriteString("\n")
		}
	}

	// Process instructions section
	if instructions, ok := promptData["instructions"].(map[string]interface{}); ok {
		if desc, ok := instructions["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if tasks, ok := instructions["tasks"].([]interface{}); ok {
			for i, task := range tasks {
				if str, ok := task.(string); ok {
					result.WriteString(fmt.Sprintf("%d. %s\n", i+1, str))
				}
			}
			result.WriteString("\n")
		}

		if examples, ok := instructions["examples"].([]interface{}); ok {
			for _, example := range examples {
				if str, ok := example.(string); ok {
					result.WriteString("   - Example: " + str + "\n")
				}
			}
			result.WriteString("\n")
		}
	}

	// Process important guidelines
	if guidelines, ok := promptData["important_guidelines"].([]interface{}); ok {
		result.WriteString("Important:\n")
		for _, guideline := range guidelines {
			if str, ok := guideline.(string); ok {
				result.WriteString("- " + str + "\n")
			}
		}
		result.WriteString("\n")
	}

	// Add final instruction
	if instruction, ok := promptData["final_instruction"].(string); ok {
		result.WriteString(instruction + "\n")
	}

	return result.String(), nil
}

// processFile sends a transcript file to ChatGPT for correction
func (m *Module) processFile(ctx context.Context, inputPath, outputPath, promptTemplate string, p Params) error {
	// First check if the file is a text file
	if !utils.IsTextFile(inputPath) {
		return fmt.Errorf("file %s appears to be binary, not a text file - skipping", inputPath)
	}

	// Read the transcript file
	transcript, err := utils.ReadTextFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Check if API key is set, if not, just copy the original text
	if os.Getenv("OPENAI_API_KEY") == "" {
		utils.LogWarning("No API key set - copying original text from %s to %s", inputPath, outputPath)
		if err := utils.WriteTextFile(outputPath, transcript); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		return nil
	}

	utils.LogVerbose("Processing %s with ChatGPT...", filepath.Base(inputPath))

	// Split transcript into chunks if needed
	chunks := m.splitTranscript(transcript, p.ChunkSize)
	var correctedChunks []string

	// Process each chunk
	for i, chunk := range chunks {
		utils.LogVerbose("Processing chunk %d/%d...", i+1, len(chunks))

		// Create a timeout context for the API request
		apiCtx, cancel := context.WithTimeout(ctx, time.Duration(p.RequestTimeoutMS)*time.Millisecond)
		defer cancel()

		// Construct the full prompt for this chunk
		fullPrompt := promptTemplate
		if !strings.HasSuffix(fullPrompt, ":") && !strings.HasSuffix(fullPrompt, "\n") {
			fullPrompt += "\n\n"
		}
		fullPrompt += fmt.Sprintf("Target language: %s\n\n", p.TargetLanguage)
		fullPrompt += fmt.Sprintf("Processing chunk %d of %d:\n\n", i+1, len(chunks))
		fullPrompt += chunk

		// Create the API request
		messages := []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant that corrects transcription errors.",
			},
			{
				Role:    "user",
				Content: fullPrompt,
			},
		}

		// Send the request to ChatGPT
		response, err := m.callChatGPT(apiCtx, messages, p)
		if err != nil {
			return fmt.Errorf("ChatGPT API request failed for chunk %d: %w", i+1, err)
		}

		correctedChunks = append(correctedChunks, response)
	}

	// Combine all corrected chunks
	correctedText := strings.Join(correctedChunks, "\n\n")

	// Write the corrected transcript to the output file
	if err := utils.WriteTextFile(outputPath, correctedText); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	utils.LogSuccess("Corrected file %s -> %s", p.Input, outputPath)
	return nil
}

// splitTranscript splits a transcript into chunks of approximately the specified token size
func (m *Module) splitTranscript(transcript string, chunkSize int) []string {
	// Simple splitting by paragraphs first
	paragraphs := strings.Split(transcript, "\n\n")
	var chunks []string
	var currentChunk strings.Builder
	currentSize := 0

	for _, paragraph := range paragraphs {
		// Rough estimate of tokens (4 characters ≈ 1 token)
		paragraphSize := len(paragraph) / 4

		if currentSize+paragraphSize > chunkSize && currentSize > 0 {
			// Current chunk is full, start a new one
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentSize = 0
		}

		currentChunk.WriteString(paragraph)
		currentChunk.WriteString("\n\n")
		currentSize += paragraphSize
	}

	// Add the last chunk if it's not empty
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// callChatGPT sends a request to the OpenAI API
func (m *Module) callChatGPT(ctx context.Context, messages []ChatMessage, p Params) (string, error) {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// This shouldn't happen as we check earlier, but just in case
		// Return empty string and let the caller handle it
		return "", errors.New("OPENAI_API_KEY environment variable is not set")
	}

	// Create the request body
	reqBody := ChatRequest{
		Model:       p.Model,
		Messages:    messages,
		Temperature: p.Temperature,
		MaxTokens:   p.MaxTokens,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		var chatError ChatError
		if err := json.Unmarshal(respBody, &chatError); err == nil {
			return "", fmt.Errorf("API error: %s", chatError.Error.Message)
		}
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if there are any choices in the response
	if len(chatResp.Choices) == 0 {
		return "", errors.New("no response from ChatGPT")
	}

	// Return the content of the first choice
	return chatResp.Choices[0].Message.Content, nil
}
