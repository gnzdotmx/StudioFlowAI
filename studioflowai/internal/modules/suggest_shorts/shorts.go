package suggestshorts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	chatgpt "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"gopkg.in/yaml.v3"
)

// contextKey is a type for context keys
type contextKey string

// ChatGPTServiceKey is the context key for the ChatGPT service
const ChatGPTServiceKey = contextKey("chatgpt_service")

// Module implements shorts suggestion functionality
type Module struct{}

// Params contains the parameters for shorts suggestion generation
type Params struct {
	Input            string  `json:"input"`            // Path to input transcript file or directory
	Output           string  `json:"output"`           // Path to output directory
	FilePattern      string  `json:"filePattern"`      // File pattern to match in input directory (default: "*_corrected.txt")
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
	ShortTitle  string `yaml:"shortTitle"`  // Short title for the video clip
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

// regexMatchString is a package variable that can be overridden in tests
var regexMatchString = regexp.MatchString

// New creates a new shorts suggestion module
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "suggest_shorts"
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
		utils.LogWarning("OPENAI_API_KEY environment variable is not set. A placeholder file will be generated.")
	}

	// Check if the prompt template file exists
	if p.PromptFilePath != "" {
		if _, err := os.Stat(p.PromptFilePath); os.IsNotExist(err) {
			return fmt.Errorf("prompt template file %s does not exist", p.PromptFilePath)
		}
	}

	// Validate duration parameters
	if p.MinDuration > 0 && p.MaxDuration > 0 && p.MinDuration > p.MaxDuration {
		return fmt.Errorf("minDuration (%d) cannot be greater than maxDuration (%d)", p.MinDuration, p.MaxDuration)
	}

	return nil
}

// getChatGPTService returns a ChatGPT service from context or creates a new one
func (m *Module) getChatGPTService(ctx context.Context) (chatgpt.ChatGPTServicer, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	// Check if service is provided in context
	if service, ok := ctx.Value(ChatGPTServiceKey).(chatgpt.ChatGPTServicer); ok {
		return service, nil
	}

	// Create new service if not in context
	return chatgpt.NewChatGPTService()
}

// Execute generates shorts suggestions from a transcript
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
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
		p.MinDuration = 45
	}
	if p.MaxDuration == 0 {
		p.MaxDuration = 75
	}
	if p.RequestTimeoutMs == 0 {
		p.RequestTimeoutMs = 60000
	}
	if p.OutputFileName == "" {
		p.OutputFileName = "shorts_suggestions"
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Handle input path resolution
	inputPath, err := getInputFilePath(resolvedInput, p.FilePattern)
	if err != nil {
		return modules.ModuleResult{}, err
	}

	// Read transcript
	transcript, err := os.ReadFile(inputPath)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Define output file path
	outputFilePath := filepath.Join(p.Output, p.OutputFileName+".yaml")

	// Check if API key is set, if not, save a placeholder file
	if !chatgpt.IsAPIKeySet() {
		utils.LogWarning("No API key set - saving placeholder file to %s", outputFilePath)
		if err := m.writePlaceholderFile(outputFilePath); err != nil {
			return modules.ModuleResult{}, err
		}
		return modules.ModuleResult{
			Outputs: map[string]string{
				"suggestions": outputFilePath,
			},
			Statistics: map[string]interface{}{
				"status": "placeholder_generated",
			},
		}, nil
	}

	// Get prompt template
	promptTemplate, err := m.getPromptTemplate(p.PromptFilePath)
	if err != nil {
		return modules.ModuleResult{}, err
	}

	// Create prompt with transcript
	prompt := fmt.Sprintf(promptTemplate,
		p.MinDuration,
		p.MaxDuration,
		string(transcript))

	// Create API client timeout context
	apiCtx, cancel := context.WithTimeout(ctx, time.Duration(p.RequestTimeoutMs)*time.Millisecond)
	defer cancel()

	// Initialize ChatGPT service
	chatGPT, err := m.getChatGPTService(ctx)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to initialize ChatGPT service: %w", err)
	}

	// Call OpenAI API
	utils.LogInfo("Generating shorts suggestions using %s model...", p.Model)
	messages := []chatgpt.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := chatGPT.GetContent(apiCtx, messages, chatgpt.CompletionOptions{
		Model:            p.Model,
		Temperature:      p.Temperature,
		MaxTokens:        p.MaxTokens,
		RequestTimeoutMS: p.RequestTimeoutMs,
	})
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("API request failed: %w", err)
	}

	// Parse response to get shorts suggestions
	shorts, err := parseShortsResponse(response)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to parse API response: %w\nResponse preview: %s",
			err, response[:Min(len(response), 1000)])
	}

	// Create output
	outputData := ShortsOutput{
		SourceVideo: "${source_video}", // This will be replaced at runtime
		Shorts:      shorts,
	}

	// Save to YAML file
	yamlData, err := yaml.Marshal(outputData)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to generate YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFilePath, yamlData, 0644); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to write output file: %w", err)
	}

	utils.LogSuccess("Shorts suggestions saved to %s", outputFilePath)

	// Create result with output file information
	result := modules.ModuleResult{
		Outputs: map[string]string{
			"suggestions": outputFilePath,
		},
		Metadata: map[string]interface{}{
			"inputFile":    inputPath,
			"outputFormat": "yaml",
			"numShorts":    len(shorts),
		},
	}

	return result, nil
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
				Name:        "promptFilePath",
				Description: "Path to custom prompt YAML file",
				Type:        string(modules.InputTypeFile),
			},
			{
				Name:        "model",
				Description: "OpenAI model to use",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "minDuration",
				Description: "Minimum duration of shorts in seconds",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "maxDuration",
				Description: "Maximum duration of shorts in seconds",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "suggestions",
				Description: "Generated shorts suggestions file",
				Patterns:    []string{".yaml"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
}

// writePlaceholderFile writes a placeholder YAML file when no API key is available
func (m *Module) writePlaceholderFile(outputPath string) error {
	placeholderOutput := ShortsOutput{
		SourceVideo: "${source_video}",
		Shorts: []ShortClip{
			{
				Title:       "API Key Required - Please Configure",
				StartTime:   "00:00:00",
				EndTime:     "00:00:03",
				Description: "Please set the OPENAI_API_KEY environment variable to generate shorts suggestions.",
				Tags:        "tag1 tag2 tag3",
				ShortTitle:  "Configure API Key",
			},
		},
	}

	// Use yaml.Marshal with proper indentation
	yamlData, err := yaml.Marshal(placeholderOutput)
	if err != nil {
		return fmt.Errorf("failed to generate placeholder YAML: %w", err)
	}

	// Ensure the YAML is properly formatted and clean
	cleanYAML := strings.ReplaceAll(string(yamlData), "\r", "")
	cleanYAML = strings.TrimSpace(cleanYAML)

	if err := os.WriteFile(outputPath, []byte(cleanYAML+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write placeholder file: %w", err)
	}

	utils.LogSuccess("Placeholder shorts suggestions saved to %s", outputPath)
	return nil
}

// getPromptTemplate returns the prompt template from file or default
func (m *Module) getPromptTemplate(promptFilePath string) (string, error) {
	if promptFilePath != "" {
		promptData, err := loadPromptTemplate(promptFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to load prompt template: %w", err)
		}
		utils.LogInfo("Using prompt template: %s", promptFilePath)
		return promptData.Prompt, nil
	}

	utils.LogInfo("Using default prompt template")
	return `## CRITICAL REQUIREMENTS:
1. COMPLETE COVERAGE: Analyze the ENTIRE transcript to the END. NEVER STOP early.
2. SPANISH OUTPUT: Generate ALL content (titles, descriptions, tags, short_title) in SPANISH for Spanish-speaking audiences.
3. TOPIC IDENTIFICATION: Identify all main topics/themes discussed in the video.
4. MINIMUM CLIPS PER TOPIC: Create AT LEAST 3 shorts for EACH identified topic.
5. DISTRIBUTION: Ensure clips are distributed evenly across beginning, middle, and end.
6. DURATION: Each clip should be between %d and %d seconds.
7. YAML FORMAT: Use EXACTLY the format shown in the example - respect indentation with spaces.

## REQUIRED YAML FORMAT (USE EXACTLY THIS FORMAT):
'''yaml
sourceVideo: ${source_video}
shorts:
  - title: "Título atractivo"
    startTime: "hh:mm:ss"
    endTime: "hh:mm:ss"
    description: "Descripción detallada que explica por qué este momento es interesante"
    tags: "Hashtag1, Hashtag2, Hashtag3"
    short_title: "¿Pregunta o descripción corta que se responde en el video?"
'''

## YAML SAFETY GUIDELINES (VERY IMPORTANT):
- RESPECT the INDENTATION exactly as shown in the example (two spaces)
- Use quotes for text with special characters like : or -
- Avoid line breaks within values
- DO NOT INCLUDE COMMENTS like "# Maximum 40 characters" in your final response
- VERIFY that your YAML is valid before submitting
- short_title: must be no more than 40 characters, try to be creative and interesting
- title: must be maximum 100 characters including high impact hashtags between the name using #hashtags format

## SELECTION CRITERIA (at least TWO):
- Hook factor: Captures attention in first 3 seconds
- Viral potential: Motivates sharing/commenting
- Emotional impact: Generates strong emotional response
- Clear value: Offers specific insight or useful teaching
- Self-contained: Understandable without additional context
- Quotable: Contains memorable phrase for text overlay
- Complete story: Mini-narrative with beginning, development, conclusion
- Unique perspective: Surprising or uncommon point of view
- Key moments: Highlights the most impactful or informative segments
- Visual appeal: Contains visually engaging elements or demonstrations
- Action-oriented: Shows clear steps, processes, or demonstrations
- Educational value: Teaches something specific and valuable

## MANDATORY FINAL VERIFICATION:
1. Did you analyze the COMPLETE transcript to the end?
2. Did you identify all main topics from the content?
3. Do you have AT LEAST 3 shorts for EACH identified topic?
4. Is the YAML format EXACTLY as shown in the example?
5. Does the indentation use TWO SPACES (not tabs)?

## IMPORTANT: Your response MUST begin with the required YAML format, without prior explanations.

Transcript:
%s`, nil
}

// getInputFilePath resolves the input file path based on the input directory and pattern
func getInputFilePath(inputPath, filePattern string) (string, error) {
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("input path does not exist: %w", err)
	}

	if !fileInfo.IsDir() {
		return inputPath, nil
	}

	files, err := filepath.Glob(filepath.Join(inputPath, filePattern))
	if err != nil {
		return "", fmt.Errorf("error matching files with pattern: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files matching pattern %s found in %s", filePattern, inputPath)
	}

	if len(files) > 1 {
		utils.LogWarning("Multiple files match pattern %s, using most recent one", filePattern)
	}

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

// validateTimestamp checks if a string is a valid timestamp in HH:MM:SS format
func validateTimestamp(timestamp string) error {
	// Check basic format using regex
	matched, err := regexMatchString(`^\d{2}:\d{2}:\d{2}$`, timestamp)
	if err != nil {
		return fmt.Errorf("failed to validate timestamp format: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid timestamp format: %s (expected HH:MM:SS)", timestamp)
	}

	// Parse the components
	parts := strings.Split(timestamp, ":")
	if len(parts) != 3 {
		return fmt.Errorf("invalid timestamp format: %s (expected HH:MM:SS)", timestamp)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return fmt.Errorf("invalid hours in timestamp: %s (must be 00-23)", timestamp)
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return fmt.Errorf("invalid minutes in timestamp: %s (must be 00-59)", timestamp)
	}

	seconds, err := strconv.Atoi(parts[2])
	if err != nil || seconds < 0 || seconds > 59 {
		return fmt.Errorf("invalid seconds in timestamp: %s (must be 00-59)", timestamp)
	}

	return nil
}

// validateShortClip checks if a short clip has valid required fields
func validateShortClip(clip *ShortClip) error {
	if clip == nil {
		return fmt.Errorf("short clip cannot be nil")
	}

	if clip.Title == "" {
		return fmt.Errorf("short clip title is required")
	}

	if clip.StartTime == "" {
		return fmt.Errorf("short clip start time is required")
	}

	if clip.EndTime == "" {
		return fmt.Errorf("short clip end time is required")
	}

	// Validate timestamps
	if err := validateTimestamp(clip.StartTime); err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	if err := validateTimestamp(clip.EndTime); err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	// Validate that end time is after start time
	startParts := strings.Split(clip.StartTime, ":")
	endParts := strings.Split(clip.EndTime, ":")

	// Convert start time components to seconds
	startHours, err := strconv.Atoi(startParts[0])
	if err != nil {
		return fmt.Errorf("invalid hours in start time: %w", err)
	}
	startMinutes, err := strconv.Atoi(startParts[1])
	if err != nil {
		return fmt.Errorf("invalid minutes in start time: %w", err)
	}
	startSeconds, err := strconv.Atoi(startParts[2])
	if err != nil {
		return fmt.Errorf("invalid seconds in start time: %w", err)
	}

	// Convert end time components to seconds
	endHours, err := strconv.Atoi(endParts[0])
	if err != nil {
		return fmt.Errorf("invalid hours in end time: %w", err)
	}
	endMinutes, err := strconv.Atoi(endParts[1])
	if err != nil {
		return fmt.Errorf("invalid minutes in end time: %w", err)
	}
	endSeconds, err := strconv.Atoi(endParts[2])
	if err != nil {
		return fmt.Errorf("invalid seconds in end time: %w", err)
	}

	// Calculate total seconds
	startTotalSeconds := (startHours * 3600) + (startMinutes * 60) + startSeconds
	endTotalSeconds := (endHours * 3600) + (endMinutes * 60) + endSeconds

	if endTotalSeconds <= startTotalSeconds {
		return fmt.Errorf("end time (%s) must be after start time (%s)", clip.EndTime, clip.StartTime)
	}

	return nil
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
				// Validate each short clip
				for _, clip := range shortsData.Shorts {
					if err := validateShortClip(&clip); err != nil {
						return nil, fmt.Errorf("invalid short clip: %w", err)
					}
				}
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
						// Validate each short clip
						for _, clip := range shortsData.Shorts {
							if err := validateShortClip(&clip); err != nil {
								return nil, fmt.Errorf("invalid short clip: %w", err)
							}
						}
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
					strings.HasPrefix(trimmed, "tags:") ||
					strings.HasPrefix(trimmed, "shortTitle:")) {
					// This is a property of a shorts item
					cleanedLines = append(cleanedLines, "    "+trimmed)
				}
			}

			if len(cleanedLines) > 0 {
				fixedYaml := strings.Join(cleanedLines, "\n")
				var shortsData ShortsOutput
				err := yaml.Unmarshal([]byte(fixedYaml), &shortsData)
				if err == nil && len(shortsData.Shorts) > 0 {
					// Validate each short clip
					for _, clip := range shortsData.Shorts {
						if err := validateShortClip(&clip); err != nil {
							return nil, fmt.Errorf("invalid short clip: %w", err)
						}
					}
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
				// Validate each short clip
				for _, clip := range shorts {
					if err := validateShortClip(&clip); err != nil {
						return nil, fmt.Errorf("invalid short clip: %w", err)
					}
				}
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
				} else if strings.HasPrefix(nextLine, "shortTitle:") {
					shortTitle := strings.TrimSpace(strings.TrimPrefix(nextLine, "shortTitle:"))
					clip.ShortTitle = strings.Trim(shortTitle, "\"'")
				}

				// If we hit a new item, break
				if j > i+1 && strings.HasPrefix(nextLine, "- title:") {
					break
				}
			}

			// If we found the required fields, validate and add the clip
			if foundStart && foundEnd && clip.Title != "" {
				// If we collected description lines and don't have a description already
				if clip.Description == "" && len(descLines) > 0 {
					clip.Description = strings.Join(descLines, " ")
				}

				// Validate the clip
				if err := validateShortClip(&clip); err != nil {
					return nil, fmt.Errorf("invalid short clip: %w", err)
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

				// Validate the clip
				if err := validateShortClip(&clip); err != nil {
					return nil, fmt.Errorf("invalid short clip: %w", err)
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

// Min returns the smaller of x or y
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
