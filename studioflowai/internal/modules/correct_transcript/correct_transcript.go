package correcttranscript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	chatgpt "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"

	"gopkg.in/yaml.v3"
)

// Module implements the ChatGPT correction functionality
type Module struct {
	chatGPTService chatgpt.ChatGPTServicer
}

// Params contains the parameters for ChatGPT correction
type Params struct {
	Input            string  `json:"input"`            // Path to input transcript file
	Output           string  `json:"output"`           // Path to output directory
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

// New creates a new ChatGPT correction module
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "correct_transcript"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, ""); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Check if the API key is set - just warn but don't error
	if !chatgpt.IsAPIKeySet() {
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
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
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
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load the prompt template
	promptTemplate, err := m.loadPromptTemplate(p.PromptTemplate)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to load prompt template: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Verify input exists at execution time
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("input file not found: %w", err)
	}

	if fileInfo.IsDir() {
		return modules.ModuleResult{}, fmt.Errorf("input must be a file, not a directory: %s", resolvedInput)
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

	// Process the file
	if err := m.processFile(ctx, resolvedInput, outputPath, promptTemplate, p); err != nil {
		return modules.ModuleResult{}, err
	}

	utils.LogSuccess("Corrected file %s -> %s", resolvedInput, outputPath)

	return modules.ModuleResult{
		Outputs: map[string]string{
			"corrected": outputPath,
		},
		Statistics: map[string]interface{}{
			"model":       p.Model,
			"chunkSize":   p.ChunkSize,
			"language":    p.TargetLanguage,
			"inputFile":   resolvedInput,
			"outputFile":  outputPath,
			"processTime": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to input transcript file",
				Patterns:    []string{".txt", ".srt"},
				Type:        string(modules.InputTypeFile),
			},
			{
				Name:        "output",
				Description: "Path to output directory",
				Type:        string(modules.InputTypeDirectory),
			},
		},
		OptionalInputs: []modules.ModuleInput{
			{
				Name:        "outputFileName",
				Description: "Custom output filename",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "promptTemplate",
				Description: "Path to prompt template file",
				Type:        string(modules.InputTypeFile),
			},
			{
				Name:        "model",
				Description: "OpenAI model to use",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "targetLanguage",
				Description: "Target language for corrections",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "corrected",
				Description: "Corrected transcript file",
				Patterns:    []string{".txt"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
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

// getChatGPTService creates or returns an existing ChatGPT service instance
func (m *Module) getChatGPTService() (chatgpt.ChatGPTServicer, error) {
	if m.chatGPTService != nil {
		return m.chatGPTService, nil
	}

	service, err := chatgpt.NewChatGPTService()
	if err != nil {
		return nil, err
	}

	m.chatGPTService = service
	return service, nil
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
	if !chatgpt.IsAPIKeySet() {
		utils.LogWarning("No API key set - copying original text from %s to %s", inputPath, outputPath)
		if err := utils.WriteTextFile(outputPath, transcript); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		return nil
	}

	utils.LogVerbose("Processing %s with ChatGPT...", filepath.Base(inputPath))

	// Initialize ChatGPT service
	chatGPT, err := m.getChatGPTService()
	if err != nil {
		return fmt.Errorf("failed to initialize ChatGPT service: %w", err)
	}

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
		messages := []chatgpt.ChatMessage{
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
		response, err := chatGPT.GetContent(apiCtx, messages, chatgpt.CompletionOptions{
			Model:            p.Model,
			Temperature:      p.Temperature,
			MaxTokens:        p.MaxTokens,
			RequestTimeoutMS: p.RequestTimeoutMS,
		})
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
