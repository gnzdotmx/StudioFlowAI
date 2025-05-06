package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"gopkg.in/yaml.v3"
)

// ShortsModule defines the ChatGPT-based shorts suggestion module
type ShortsModule struct{}

// ShortsParams contains the parameters for shorts suggestion generation
type ShortsParams struct {
	Input            string  `json:"input"`            // Path to input transcript file or directory
	Output           string  `json:"output"`           // Path to output directory
	FilePattern      string  `json:"filePattern"`      // File pattern to match in input directory (default: "*_corrected.txt")
	InputFileName    string  `json:"inputFileName"`    // Specific input file name to process
	OutputFileName   string  `json:"outputFileName"`   // Custom output file name (without extension)
	Model            string  `json:"model"`            // OpenAI model to use (default: "gpt-4o")
	Temperature      float64 `json:"temperature"`      // Model temperature (default: 0.7)
	MaxTokens        int     `json:"maxTokens"`        // Maximum tokens for the response (default: 4000)
	MinDuration      int     `json:"minDuration"`      // Minimum duration of shorts in seconds (default: 15)
	MaxDuration      int     `json:"maxDuration"`      // Maximum duration of shorts in seconds (default: 60)
	MaxShorts        int     `json:"maxShorts"`        // Maximum number of shorts to generate (default: 10)
	PromptFilePath   string  `json:"promptFilePath"`   // Path to custom prompt YAML file
	RequestTimeoutMs int     `json:"requestTimeoutMs"` // API request timeout in milliseconds (default: 60000)
}

// ShortClip represents a single short video clip suggestion
type ShortClip struct {
	Title       string `yaml:"title"`       // Title/description of the short
	StartTime   string `yaml:"startTime"`   // Start timestamp in HH:MM:SS format
	EndTime     string `yaml:"endTime"`     // End timestamp in HH:MM:SS format
	Description string `yaml:"description"` // Additional description/context
	Tags        string `yaml:"tags"`        // Suggested tags for the short
}

// ShortsOutput defines the structure of the shorts YAML output
type ShortsOutput struct {
	SourceVideo string      `yaml:"sourceVideo"` // Original video file (will be replaced at runtime)
	Shorts      []ShortClip `yaml:"shorts"`      // List of short clips
}

// PromptData represents the structure of a YAML prompt template
type PromptData struct {
	Title       string `yaml:"title"`
	Role        string `yaml:"role"`
	Prompt      string `yaml:"prompt"`
	Description string `yaml:"description"`
}

// NewShorts creates a new shorts suggestion module
func NewShorts() *ShortsModule {
	return &ShortsModule{}
}

// Name returns the module name
func (m *ShortsModule) Name() string {
	return "shorts"
}

// Validate checks if the parameters are valid
func (m *ShortsModule) Validate(params map[string]interface{}) error {
	var p ShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	if p.Input == "" {
		return fmt.Errorf("input path is required")
	}

	if p.Output == "" {
		return fmt.Errorf("output path is required")
	}

	return nil
}

// Execute generates shorts suggestions from a transcript
func (m *ShortsModule) Execute(ctx context.Context, params map[string]interface{}) error {
	var p ShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.FilePattern == "" {
		p.FilePattern = "*_corrected.txt"
	}
	if p.Model == "" {
		p.Model = "gpt-4o"
	}
	if p.Temperature == 0 {
		p.Temperature = 0.7
	}
	if p.MaxTokens == 0 {
		p.MaxTokens = 4000
	}
	if p.MinDuration == 0 {
		p.MinDuration = 15
	}
	if p.MaxDuration == 0 {
		p.MaxDuration = 60
	}
	if p.RequestTimeoutMs == 0 {
		p.RequestTimeoutMs = 60000
	}
	if p.OutputFileName == "" {
		p.OutputFileName = "shorts_suggestions"
	}

	// Handle input path resolution
	inputPath, err := getInputFilePath(p.Input, p.FilePattern, p.InputFileName)
	if err != nil {
		return err
	}

	// Read transcript
	transcript, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Define output file path
	outputFilePath := filepath.Join(p.Output, p.OutputFileName+".yaml")

	// Check if API key is set, if not, save a placeholder file
	if os.Getenv("OPENAI_API_KEY") == "" {
		utils.LogWarning("No API key set - saving placeholder file to %s", outputFilePath)

		// Create a placeholder shorts output
		placeholderOutput := ShortsOutput{
			SourceVideo: "${source_video}",
			Shorts: []ShortClip{
				{
					Title:       "API Key Required",
					StartTime:   "00:00:00",
					EndTime:     "00:00:15",
					Description: "Please set the OPENAI_API_KEY environment variable to generate shorts suggestions.",
					Tags:        "api-key-missing, setup-required",
				},
			},
		}

		// Marshal to YAML
		yamlData, err := yaml.Marshal(placeholderOutput)
		if err != nil {
			return fmt.Errorf("failed to generate placeholder YAML: %w", err)
		}

		// Write to file
		if err := os.WriteFile(outputFilePath, yamlData, 0644); err != nil {
			return fmt.Errorf("failed to write placeholder file: %w", err)
		}

		utils.LogSuccess("Placeholder shorts suggestions saved to %s", outputFilePath)
		return nil
	}

	// Check if prompt template file exists and use it
	var promptTemplate string
	if p.PromptFilePath != "" {
		promptData, err := loadPromptTemplate(p.PromptFilePath)
		if err != nil {
			return fmt.Errorf("failed to load prompt template: %w", err)
		}
		promptTemplate = promptData.Prompt
	} else {
		// Default prompt if no template is provided
		promptTemplate = getShortsPrompt()
	}

	// Create prompt with transcript
	prompt := fmt.Sprintf(promptTemplate,
		p.MinDuration,
		p.MaxDuration,
		string(transcript))

	// Create API client timeout context
	apiCtx, cancel := context.WithTimeout(ctx, time.Duration(p.RequestTimeoutMs)*time.Millisecond)
	defer cancel()

	// Call OpenAI API
	utils.LogInfo("Generating shorts suggestions using %s model...", p.Model)
	client := NewOpenAIClient(os.Getenv("OPENAI_API_KEY"))
	response, err := client.Complete(apiCtx, CompletionRequest{
		Model:       p.Model,
		Messages:    []CompletionMessage{{Role: "user", Content: prompt}},
		Temperature: p.Temperature,
		MaxTokens:   p.MaxTokens,
	})
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	// Parse response to get shorts suggestions
	content := response.Choices[0].Message.Content
	shorts, err := parseShortsResponse(content)
	if err != nil {
		// Log more detailed error information
		return fmt.Errorf("failed to parse API response: %w\nResponse preview: %s",
			err, content[:Min(len(content), 1000)])
	}

	// Create output
	outputData := ShortsOutput{
		SourceVideo: "${source_video}", // This will be replaced at runtime
		Shorts:      shorts,
	}

	// Save to YAML file
	yamlData, err := yaml.Marshal(outputData)
	if err != nil {
		return fmt.Errorf("failed to generate YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFilePath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	utils.LogSuccess("Shorts suggestions saved to %s", outputFilePath)
	return nil
}

// getInputFilePath resolves the input file path based on the input directory, pattern, and filename
func getInputFilePath(inputPath, filePattern, inputFileName string) (string, error) {
	// Check if input is a file or directory
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("input path does not exist: %w", err)
	}

	// If input is a file, return it directly
	if !fileInfo.IsDir() {
		return inputPath, nil
	}

	// If input is a directory and a specific filename is provided
	if inputFileName != "" {
		return filepath.Join(inputPath, inputFileName), nil
	}

	// If input is a directory, find files matching the pattern
	files, err := filepath.Glob(filepath.Join(inputPath, filePattern))
	if err != nil {
		return "", fmt.Errorf("error matching files with pattern: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files matching pattern %s found in %s", filePattern, inputPath)
	}

	// Sort files by modification time (newest first)
	if len(files) > 1 {
		utils.LogWarning("Multiple files match pattern %s, using most recent one", filePattern)
	}

	// Return the first (or only) matching file
	return files[0], nil
}

// loadPromptTemplate loads a prompt template from a YAML file
func loadPromptTemplate(filePath string) (*PromptData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt template: %w", err)
	}

	var promptData PromptData
	if err := yaml.Unmarshal(data, &promptData); err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}

	return &promptData, nil
}

// NewOpenAIClient creates a new OpenAI API client
type OpenAIClient struct {
	apiKey string
}

// CompletionMessage represents a message in a completion request
type CompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest represents a request to the OpenAI completions API
type CompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []CompletionMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

// CompletionResponse represents a response from the OpenAI completions API
type CompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index        int               `json:"index"`
		Message      CompletionMessage `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
	}
}

// Complete sends a completion request to the OpenAI API
func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	// Create the request body
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var chatResp CompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// parseShortsResponse parses the ChatGPT response to extract shorts data
func parseShortsResponse(content string) ([]ShortClip, error) {
	// Try to identify and extract YAML content - look for sourceVideo and shorts sections
	if strings.Contains(content, "sourceVideo:") && strings.Contains(content, "shorts:") {
		// Try to clean the content to get only the YAML portion
		yamlStart := strings.Index(content, "sourceVideo:")
		if yamlStart != -1 {
			// Extract the YAML portion of the content
			yamlContent := content[yamlStart:]

			// Better cleanup of the YAML content
			// Remove any trailing content after the YAML block
			if idx := strings.Index(yamlContent, "```\n\n"); idx > 0 {
				yamlContent = yamlContent[:idx]
			}

			// Try to parse the yaml content directly
			var shortsData ShortsOutput
			err := yaml.Unmarshal([]byte(yamlContent), &shortsData)
			if err == nil && len(shortsData.Shorts) > 0 {
				return shortsData.Shorts, nil
			}

			// If direct parsing failed, try to extract code blocks
			if strings.Contains(yamlContent, "```") {
				// Find the first code block start after sourceVideo:
				codeStart := strings.Index(yamlContent, "```") + 3
				codeEnd := strings.LastIndex(yamlContent, "```")

				if codeStart != -1 && codeEnd > codeStart {
					// Skip the language identifier if present (e.g. ```yaml)
					if nextLineIdx := strings.Index(yamlContent[codeStart:], "\n"); nextLineIdx != -1 {
						codeStart += nextLineIdx + 1
					}

					cleanYaml := strings.TrimSpace(yamlContent[codeStart:codeEnd])

					// If the extracted YAML doesn't have the expected structure, try to fix it
					if !strings.Contains(cleanYaml, "sourceVideo:") && strings.Contains(cleanYaml, "shorts:") {
						cleanYaml = "sourceVideo: ${source_video}\n" + cleanYaml
					} else if !strings.Contains(cleanYaml, "shorts:") && strings.Contains(cleanYaml, "- title:") {
						cleanYaml = "sourceVideo: ${source_video}\nshorts:\n" + cleanYaml
					}

					err := yaml.Unmarshal([]byte(cleanYaml), &shortsData)
					if err == nil && len(shortsData.Shorts) > 0 {
						return shortsData.Shorts, nil
					}

					// If that still fails, log the error and the cleaned YAML for debugging
					utils.LogDebug("Failed to parse cleaned YAML: %v\nCleaned YAML:\n%s", err, cleanYaml)
				}
			}

			// Try once more with a more aggressive cleanup approach
			// This handles cases where the model outputs the YAML without proper indentation
			lines := strings.Split(yamlContent, "\n")
			var cleanedLines []string
			inShorts := false

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "sourceVideo:") {
					cleanedLines = append(cleanedLines, trimmed)
				} else if strings.HasPrefix(trimmed, "shorts:") {
					cleanedLines = append(cleanedLines, trimmed)
					inShorts = true
				} else if inShorts && strings.HasPrefix(trimmed, "- title:") {
					// This is a new shorts item
					cleanedLines = append(cleanedLines, "  "+trimmed)
				} else if inShorts && (strings.HasPrefix(trimmed, "startTime:") ||
					strings.HasPrefix(trimmed, "endTime:") ||
					strings.HasPrefix(trimmed, "description:") ||
					strings.HasPrefix(trimmed, "tags:")) {
					// This is a property of a shorts item
					cleanedLines = append(cleanedLines, "    "+trimmed)
				}
			}

			if len(cleanedLines) > 0 {
				fixedYaml := strings.Join(cleanedLines, "\n")
				var shortsData ShortsOutput
				err := yaml.Unmarshal([]byte(fixedYaml), &shortsData)
				if err == nil && len(shortsData.Shorts) > 0 {
					return shortsData.Shorts, nil
				}

				// Log the attempt for debugging
				utils.LogDebug("Reconstructed YAML parsing failed: %v\nReconstructed YAML:\n%s", err, fixedYaml)
			}
		}
	}

	// First try to parse as JSON
	var shorts []ShortClip
	if strings.Contains(content, "[") && strings.Contains(content, "]") {
		// Try to extract JSON array from the response
		startIdx := strings.Index(content, "[")
		endIdx := strings.LastIndex(content, "]") + 1
		if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			jsonContent := content[startIdx:endIdx]
			if err := json.Unmarshal([]byte(jsonContent), &shorts); err == nil {
				return shorts, nil
			}
		}
	}

	// If JSON parsing fails, try a simple line-by-line parsing approach
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Look for lines that start with a title field
		if (strings.HasPrefix(line, "- title:") || strings.HasPrefix(line, "title:")) && i+2 < len(lines) {
			var clip ShortClip

			// Extract the title
			title := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- title:"), "title:"))
			// Remove quotes if they exist
			title = strings.Trim(title, "\"'")
			clip.Title = title

			// Look for startTime and endTime in the next few lines
			foundStart := false
			foundEnd := false
			descLines := []string{}

			// Search the next few lines for other fields
			for j := i + 1; j < i+10 && j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])

				if strings.HasPrefix(nextLine, "startTime:") {
					startTime := strings.TrimSpace(strings.TrimPrefix(nextLine, "startTime:"))
					clip.StartTime = strings.Trim(startTime, "\"'")
					foundStart = true
				} else if strings.HasPrefix(nextLine, "endTime:") {
					endTime := strings.TrimSpace(strings.TrimPrefix(nextLine, "endTime:"))
					clip.EndTime = strings.Trim(endTime, "\"'")
					foundEnd = true
				} else if strings.HasPrefix(nextLine, "description:") {
					desc := strings.TrimSpace(strings.TrimPrefix(nextLine, "description:"))
					clip.Description = strings.Trim(desc, "\"'")
				} else if strings.HasPrefix(nextLine, "tags:") {
					tags := strings.TrimSpace(strings.TrimPrefix(nextLine, "tags:"))
					clip.Tags = strings.Trim(tags, "\"'")
				} else if !strings.HasPrefix(nextLine, "-") && !strings.Contains(nextLine, ":") {
					// This might be a continuation of the description
					descLines = append(descLines, nextLine)
				}

				// If we hit a new item, break
				if j > i+1 && strings.HasPrefix(nextLine, "- title:") {
					break
				}
			}

			// If we found the required fields, add the clip
			if foundStart && foundEnd && clip.Title != "" {
				// If we collected description lines and don't have a description already
				if clip.Description == "" && len(descLines) > 0 {
					clip.Description = strings.Join(descLines, " ")
				}
				shorts = append(shorts, clip)
			}

			// Skip ahead to avoid reprocessing these lines
			i += 3
		}
	}

	// If we still have no shorts, try a more aggressive approach - look for pairs of timestamps
	if len(shorts) == 0 {
		for i := 0; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])

			// Look for timestamp patterns like 00:00:00 in the line
			timestampRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
			matches := timestampRegex.FindAllString(line, -1)

			if len(matches) >= 2 {
				// We found a pair of timestamps in this line
				var clip ShortClip
				clip.StartTime = matches[0]
				clip.EndTime = matches[1]

				// Try to find a title before or after this line
				if i > 0 {
					prevLine := strings.TrimSpace(lines[i-1])
					if !timestampRegex.MatchString(prevLine) { // Not another timestamp line
						clip.Title = prevLine
					}
				}

				// If no title found, try the next line
				if clip.Title == "" && i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if !timestampRegex.MatchString(nextLine) { // Not another timestamp line
						clip.Title = nextLine
					}
				}

				// If we still don't have a title, use a default
				if clip.Title == "" {
					clip.Title = fmt.Sprintf("Clip at %s", clip.StartTime)
				}

				shorts = append(shorts, clip)
			}
		}
	}

	// If we still have no shorts, generate an informative error
	if len(shorts) == 0 {
		// Generate a snippet of the content to help debugging
		contentPreview := content
		if len(content) > 500 {
			contentPreview = content[:500] + "... [truncated]"
		}
		return nil, fmt.Errorf("could not parse shorts from API response. Content begins with: %s", contentPreview)
	}

	return shorts, nil
}

// getShortsPrompt returns the default prompt for shorts suggestions
func getShortsPrompt() string {
	return `Analyze the following video transcript deeply to identify segments that would make exceptional short-form videos of %d-%d seconds in length.

IMPORTANT: Quality over quantity. Instead of generating a fixed number of suggestions, carefully evaluate the entire transcript and only suggest clips that are truly engaging, educational, or entertaining. Each short must be able to stand alone as compelling content.

For each high-quality suggestion:
1. Identify segments with genuinely memorable content that will resonate with viewers
2. Provide a catchy, clickable title for each short
3. Give precise start and end timestamps in HH:MM:SS format
4. Write a compelling description of what happens in this segment and why it's engaging
5. Suggest 3-5 strategic hashtags or keywords to maximize reach and engagement

Format your response as a list of JSON objects with these fields:
- title: A catchy, attention-grabbing title for the short
- startTime: Start timestamp in HH:MM:SS format
- endTime: End timestamp in HH:MM:SS format
- description: A detailed description of what makes this segment compelling
- tags: Strategic hashtags or keywords for the short

SELECTION CRITERIA - Only include segments that meet at least two of these criteria:
- Contain powerful insights or knowledge bombs that provide clear value
- Include emotional or surprising moments that create strong reactions
- Feature exceptionally clear explanations of complex topics
- Would be interesting and make sense even without additional context
- Contain a compelling story or anecdote with a clear beginning and end
- Feature a quotable statement or memorable line that viewers would share

IMPORTANT: After identifying potential segments, carefully review each one to confirm it truly stands on its own as compelling short-form content. Only include your highest quality suggestions.

Transcript:
%s`
}

// Min returns the smaller of x or y
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
