package suggestshorts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	services "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	mocks "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock response for successful shorts generation
const mockSuccessResponse = `sourceVideo: ${source_video}
shorts:
  - title: "First Short Title"
    startTime: "00:00:00"
    endTime: "00:01:00"
    description: "First short description"
    tags: "tag1 tag2"
    shortTitle: "Short 1"
  - title: "Second Short Title"
    startTime: "00:02:00"
    endTime: "00:03:00"
    description: "Second short description"
    tags: "tag3 tag4"
    shortTitle: "Short 2"`

// testModule is a wrapper around the real module for testing
type testModule struct {
	*Module
	mockService services.ChatGPTServicer
}

// newTestModule creates a new test module with the given mock service
func newTestModule(mockService services.ChatGPTServicer) modules.Module {
	return &testModule{
		Module:      New().(*Module),
		mockService: mockService,
	}
}

// Execute overrides the real module's Execute method to use the mock service
func (m *testModule) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	if m.mockService != nil {
		// Create a new context with the mock service
		ctx = context.WithValue(ctx, ChatGPTServiceKey, m.mockService)
	}
	return m.Module.Execute(ctx, params)
}

// verifyPromptContent checks if the prompt content matches expected values
func verifyPromptContent(content string, minDuration, maxDuration int, transcript string) bool {
	// Check for required sections
	if !strings.Contains(content, "CRITICAL REQUIREMENTS") {
		return false
	}

	// Check duration values
	expectedDuration := fmt.Sprintf("DURATION: Each clip should be between %d and %d seconds", minDuration, maxDuration)
	if !strings.Contains(content, expectedDuration) {
		return false
	}

	// Check transcript content
	if !strings.Contains(content, transcript) {
		return false
	}

	// Check YAML format section
	if !strings.Contains(content, "REQUIRED YAML FORMAT") {
		return false
	}

	return true
}

func TestSuggestShortsModule(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "shorts_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to cleanup temp dir: %v", err)
		}
	}()

	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test transcript files
	testFiles := []struct {
		name    string
		content string
	}{
		{"transcript_corrected.txt", "This is a test transcript content."},
		{"other.txt", "This should be ignored"},
		{"transcript2_corrected.txt", "Another test transcript"},
	}

	for _, tf := range testFiles {
		if err := os.WriteFile(filepath.Join(inputDir, tf.name), []byte(tf.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name           string
		params         map[string]interface{}
		setupMock      func(*mocks.MockChatGPTServicer)
		wantErr        bool
		expectedOutput string
		apiKeySet      bool
	}{
		{
			name: "successful shorts generation",
			params: map[string]interface{}{
				"input":       filepath.Join(inputDir, "transcript_corrected.txt"),
				"output":      outputDir,
				"model":       "gpt-4",
				"maxTokens":   4000,
				"minDuration": 15,
				"maxDuration": 60,
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.MatchedBy(func(messages []services.ChatMessage) bool {
						if len(messages) != 1 {
							return false
						}
						return verifyPromptContent(messages[0].Content, 15, 60, "This is a test transcript content.")
					}),
					mock.MatchedBy(func(opts services.CompletionOptions) bool {
						return opts.Model == "gpt-4" && opts.MaxTokens == 4000
					}),
				).Return(mockSuccessResponse, nil)
			},
			apiKeySet:      true,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "shorts_suggestions.yaml"),
		},
		{
			name: "no api key set",
			params: map[string]interface{}{
				"input":       filepath.Join(inputDir, "transcript_corrected.txt"),
				"output":      outputDir,
				"model":       "gpt-4",
				"maxTokens":   4000,
				"minDuration": 15,
				"maxDuration": 60,
			},
			setupMock:      func(m *mocks.MockChatGPTServicer) {},
			apiKeySet:      false,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "shorts_suggestions.yaml"),
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":  "/nonexistent/path",
				"output": outputDir,
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {},
			apiKeySet: true,
			wantErr:   true,
		},
		{
			name: "missing required parameters",
			params: map[string]interface{}{
				"output": outputDir,
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {},
			apiKeySet: true,
			wantErr:   true,
		},
		{
			name: "directory with pattern",
			params: map[string]interface{}{
				"input":       inputDir,
				"output":      outputDir,
				"filePattern": "*_corrected.txt",
				"model":       "gpt-4",
				"minDuration": 45,
				"maxDuration": 75,
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.MatchedBy(func(messages []services.ChatMessage) bool {
						if len(messages) != 1 {
							return false
						}
						return verifyPromptContent(messages[0].Content, 45, 75, "Another test transcript")
					}),
					mock.MatchedBy(func(opts services.CompletionOptions) bool {
						return opts.Model == "gpt-4" && opts.MaxTokens == 4000
					}),
				).Return(mockSuccessResponse, nil)
			},
			apiKeySet:      true,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "shorts_suggestions.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original API key
			origAPIKey := os.Getenv("OPENAI_API_KEY")
			defer func() {
				if err := os.Setenv("OPENAI_API_KEY", origAPIKey); err != nil {
					t.Errorf("failed to restore API key: %v", err)
				}
			}()

			var testModule modules.Module

			// Set or unset API key based on test case
			if tt.apiKeySet {
				if err := os.Setenv("OPENAI_API_KEY", "test-api-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
				// Create mock service
				mockService := mocks.NewMockChatGPTServicer(t)
				tt.setupMock(mockService)
				// Create test module with mock
				testModule = newTestModule(mockService)
			} else {
				if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
					t.Fatalf("failed to unset API key: %v", err)
				}
				// Create test module without mock
				testModule = newTestModule(nil)
			}

			// Execute module
			result, err := testModule.Execute(context.Background(), tt.params)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, result.Outputs["suggestions"])

			// If output file was created, verify its contents
			if tt.apiKeySet {
				content, err := os.ReadFile(tt.expectedOutput)
				assert.NoError(t, err)
				assert.Contains(t, string(content), "shorts:")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "shorts_validate_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to cleanup temp dir: %v", err)
		}
	}()

	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test transcript file
	testTranscriptPath := filepath.Join(inputDir, "test.txt")
	if err := os.WriteFile(testTranscriptPath, []byte("test transcript"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"input":       testTranscriptPath,
				"output":      outputDir,
				"minDuration": 15,
				"maxDuration": 60,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output":      outputDir,
				"minDuration": 15,
				"maxDuration": 60,
			},
			wantErr: true,
		},
		{
			name: "missing output",
			params: map[string]interface{}{
				"input":       testTranscriptPath,
				"minDuration": 15,
				"maxDuration": 60,
			},
			wantErr: true,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":       "/nonexistent/path",
				"output":      outputDir,
				"minDuration": 15,
				"maxDuration": 60,
			},
			wantErr: true,
		},
		{
			name: "invalid duration values",
			params: map[string]interface{}{
				"input":       testTranscriptPath,
				"output":      outputDir,
				"minDuration": 60,
				"maxDuration": 15, // min > max
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := New()
			err := module.Validate(tt.params)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Test required inputs
	assert.Len(t, io.RequiredInputs, 2)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Contains(t, io.RequiredInputs[0].Patterns, ".txt")
	assert.Contains(t, io.RequiredInputs[0].Patterns, ".srt")

	// Test optional inputs
	assert.True(t, len(io.OptionalInputs) >= 5)
	assert.Equal(t, "outputFileName", io.OptionalInputs[0].Name)
	assert.Equal(t, "promptFilePath", io.OptionalInputs[1].Name)
	assert.Equal(t, "model", io.OptionalInputs[2].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "suggestions", io.ProducedOutputs[0].Name)
	assert.Contains(t, io.ProducedOutputs[0].Patterns, ".yaml")
}

func TestParseShortsResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantShorts  int
		wantContent map[string]string // map of field name to expected content
	}{
		{
			name: "valid yaml response",
			input: `sourceVideo: ${source_video}
shorts:
  - title: "First Short"
    startTime: "00:00:00"
    endTime: "00:01:00"
    description: "First description"
    tags: "tag1, tag2"
    shortTitle: "Short 1"
  - title: "Second Short"
    startTime: "00:02:00"
    endTime: "00:03:00"
    description: "Second description"
    tags: "tag3, tag4"
    shortTitle: "Short 2"`,
			wantErr:    false,
			wantShorts: 2,
			wantContent: map[string]string{
				"title":       "First Short",
				"startTime":   "00:00:00",
				"endTime":     "00:01:00",
				"description": "First description",
				"tags":        "tag1, tag2",
				"shortTitle":  "Short 1",
			},
		},
		{
			name:       "yaml response with code block",
			input:      "Here's the YAML:\n```yaml\nsourceVideo: ${source_video}\nshorts:\n  - title: \"Test Short\"\n    startTime: \"00:00:00\"\n    endTime: \"00:01:00\"\n    description: \"Test description\"\n    tags: \"test\"\n    shortTitle: \"Test\"\n```",
			wantErr:    false,
			wantShorts: 1,
			wantContent: map[string]string{
				"title":      "Test Short",
				"startTime":  "00:00:00",
				"endTime":    "00:01:00",
				"shortTitle": "Test",
			},
		},
		{
			name: "json array response",
			input: `[
				{
					"title": "JSON Short",
					"startTime": "00:00:00",
					"endTime": "00:01:00",
					"description": "JSON description",
					"tags": "json",
					"shortTitle": "JSON"
				}
			]`,
			wantErr:    false,
			wantShorts: 1,
			wantContent: map[string]string{
				"title":      "JSON Short",
				"startTime":  "00:00:00",
				"shortTitle": "JSON",
			},
		},
		{
			name: "line by line format",
			input: `- title: First Title
startTime: 00:00:00
endTime: 00:01:00
description: Line by line description
tags: line, by, line

- title: Second Title
startTime: 00:02:00
endTime: 00:03:00
description: Another description
tags: more, tags`,
			wantErr:    false,
			wantShorts: 2,
			wantContent: map[string]string{
				"title":       "First Title",
				"startTime":   "00:00:00",
				"endTime":     "00:01:00",
				"description": "Line by line description",
			},
		},
		{
			name: "timestamp pairs format",
			input: `Here are the clips:
Clip 1 (00:00:00 - 00:01:00): First clip description
Clip 2 (00:02:00 - 00:03:00): Second clip description`,
			wantErr:    false,
			wantShorts: 2,
		},
		{
			name: "malformed yaml",
			input: `sourceVideo: ${source_video}
shorts:
  - title: [invalid yaml
    startTime: "broken"`,
			wantErr: true,
		},
		{
			name:    "empty response",
			input:   "",
			wantErr: true,
		},
		{
			name: "partial yaml without source",
			input: `shorts:
  - title: "Test Short"
    startTime: "00:00:00"
    endTime: "00:01:00"`,
			wantErr:    false,
			wantShorts: 1,
		},
		{
			name: "yaml with only required fields",
			input: `- title: "Minimal Short"
startTime: "00:00:00"
endTime: "00:01:00"`,
			wantErr:    false,
			wantShorts: 1,
			wantContent: map[string]string{
				"title":     "Minimal Short",
				"startTime": "00:00:00",
				"endTime":   "00:01:00",
			},
		},
		{
			name: "response with extra text",
			input: `Here's my suggestion:

sourceVideo: ${source_video}
shorts:
  - title: "Extra Text Short"
    startTime: "00:00:00"
    endTime: "00:01:00"

Let me know if you need anything else!`,
			wantErr:    false,
			wantShorts: 1,
			wantContent: map[string]string{
				"title":     "Extra Text Short",
				"startTime": "00:00:00",
				"endTime":   "00:01:00",
			},
		},
		{
			name: "multiple timestamp formats",
			input: `1. First clip from 00:00:00 to 00:01:00
2. Second clip starts at 00:02:00 ends at 00:03:00
3. Third clip (00:04:00 - 00:05:00)`,
			wantErr:    false,
			wantShorts: 3,
		},
		{
			name: "invalid timestamps",
			input: `- title: "Invalid Time"
startTime: "invalid"
endTime: "00:01:00"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shorts, err := parseShortsResponse(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, shorts, tt.wantShorts)

			if len(shorts) > 0 && len(tt.wantContent) > 0 {
				// Check the first short against expected content
				short := shorts[0]
				if title, ok := tt.wantContent["title"]; ok {
					assert.Equal(t, title, short.Title)
				}
				if startTime, ok := tt.wantContent["startTime"]; ok {
					assert.Equal(t, startTime, short.StartTime)
				}
				if endTime, ok := tt.wantContent["endTime"]; ok {
					assert.Equal(t, endTime, short.EndTime)
				}
				if description, ok := tt.wantContent["description"]; ok {
					assert.Equal(t, description, short.Description)
				}
				if tags, ok := tt.wantContent["tags"]; ok {
					assert.Equal(t, tags, short.Tags)
				}
				if shortTitle, ok := tt.wantContent["shortTitle"]; ok {
					assert.Equal(t, shortTitle, short.ShortTitle)
				}
			}

			// Verify all shorts have required fields
			for _, short := range shorts {
				assert.NotEmpty(t, short.Title)
				assert.NotEmpty(t, short.StartTime)
				assert.NotEmpty(t, short.EndTime)
				assert.Regexp(t, `^\d{2}:\d{2}:\d{2}$`, short.StartTime)
				assert.Regexp(t, `^\d{2}:\d{2}:\d{2}$`, short.EndTime)
			}
		})
	}
}

func TestLoadPromptTemplate(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "prompt_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to cleanup temp dir: %v", err)
		}
	}()

	// Test cases
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		wantPrompt  string
		wantTitle   string
		wantRole    string
		setupFile   bool
		invalidPath bool
	}{
		{
			name: "valid prompt template",
			content: `title: "Test Prompt"
role: "user"
prompt: "This is a test prompt with ${variable}"
description: "A test prompt"`,
			setupFile:  true,
			wantErr:    false,
			wantPrompt: "This is a test prompt with ${variable}",
			wantTitle:  "Test Prompt",
			wantRole:   "user",
		},
		{
			name: "invalid yaml format",
			content: `title: "Test Prompt"
role: "user"
prompt: [invalid: yaml: content]`,
			setupFile: true,
			wantErr:   true,
		},
		{
			name:        "nonexistent file",
			invalidPath: true,
			wantErr:     true,
		},
		{
			name: "missing required fields",
			content: `title: "Test Prompt"
description: "Missing prompt and role"`,
			setupFile: true,
			wantErr:   false, // YAML will parse but fields will be empty
			wantTitle: "Test Prompt",
		},
		{
			name:      "empty file",
			content:   ``,
			setupFile: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.invalidPath {
				filePath = filepath.Join(tempDir, "nonexistent.yaml")
			} else {
				filePath = filepath.Join(tempDir, tt.name+".yaml")
				if tt.setupFile {
					if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			promptData, err := loadPromptTemplate(filePath)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, promptData)

			if tt.wantPrompt != "" {
				assert.Equal(t, tt.wantPrompt, promptData.Prompt)
			}
			if tt.wantTitle != "" {
				assert.Equal(t, tt.wantTitle, promptData.Title)
			}
			if tt.wantRole != "" {
				assert.Equal(t, tt.wantRole, promptData.Role)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		x        int
		y        int
		expected int
	}{
		{
			name:     "first number smaller",
			x:        5,
			y:        10,
			expected: 5,
		},
		{
			name:     "second number smaller",
			x:        10,
			y:        5,
			expected: 5,
		},
		{
			name:     "equal numbers",
			x:        7,
			y:        7,
			expected: 7,
		},
		{
			name:     "negative numbers, first smaller",
			x:        -10,
			y:        -5,
			expected: -10,
		},
		{
			name:     "negative numbers, second smaller",
			x:        -5,
			y:        -10,
			expected: -10,
		},
		{
			name:     "zero and positive",
			x:        0,
			y:        5,
			expected: 0,
		},
		{
			name:     "zero and negative",
			x:        -5,
			y:        0,
			expected: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Min(tt.x, tt.y)
			assert.Equal(t, tt.expected, result, "Min(%d, %d) = %d; want %d", tt.x, tt.y, result, tt.expected)
		})
	}
}

func TestGetPromptTemplate(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to cleanup temp dir: %v", err)
		}
	}()

	// Create a test module
	module := &Module{}

	// Create a valid custom prompt template file
	validPromptPath := filepath.Join(tempDir, "valid_prompt.yaml")
	validPromptContent := `title: "Custom Prompt"
role: "user"
prompt: "Custom prompt with ${minDuration} and ${maxDuration} and transcript: %s"
description: "A custom prompt template"`
	if err := os.WriteFile(validPromptPath, []byte(validPromptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an invalid YAML file
	invalidPromptPath := filepath.Join(tempDir, "invalid_prompt.yaml")
	invalidPromptContent := `title: "Invalid
role: [broken yaml`
	if err := os.WriteFile(invalidPromptPath, []byte(invalidPromptContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		promptFilePath string
		wantErr        bool
		wantContains   []string // Strings that should be in the result
		wantExclude    []string // Strings that should not be in the result
	}{
		{
			name:           "default template when no path provided",
			promptFilePath: "",
			wantErr:        false,
			wantContains: []string{
				"CRITICAL REQUIREMENTS",
				"SPANISH OUTPUT",
				"YAML FORMAT",
				"DURATION: Each clip should be between %d and %d seconds",
				"Transcript:",
			},
		},
		{
			name:           "valid custom template",
			promptFilePath: validPromptPath,
			wantErr:        false,
			wantContains: []string{
				"Custom prompt with ${minDuration} and ${maxDuration}",
				"transcript: %s",
			},
		},
		{
			name:           "invalid yaml file",
			promptFilePath: invalidPromptPath,
			wantErr:        true,
		},
		{
			name:           "nonexistent file",
			promptFilePath: filepath.Join(tempDir, "nonexistent.yaml"),
			wantErr:        true,
		},
		{
			name:           "directory instead of file",
			promptFilePath: tempDir,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.getPromptTemplate(tt.promptFilePath)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// Check for required content
			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}

			// Check for excluded content
			for _, exclude := range tt.wantExclude {
				assert.NotContains(t, result, exclude)
			}

			// Test the template can be used with fmt.Sprintf
			if strings.Contains(result, "%d") || strings.Contains(result, "%s") {
				// Try formatting with some test values
				formatted := fmt.Sprintf(result, 30, 60, "test transcript")
				assert.NotEmpty(t, formatted)
				assert.NotContains(t, formatted, "%d")
				assert.NotContains(t, formatted, "%s")
			}
		})
	}
}

func TestGetChatGPTService(t *testing.T) {
	module := &Module{}

	// Store original API key
	origAPIKey := os.Getenv("OPENAI_API_KEY")
	defer func() {
		if err := os.Setenv("OPENAI_API_KEY", origAPIKey); err != nil {
			t.Errorf("failed to restore API key: %v", err)
		}
	}()

	tests := []struct {
		name        string
		setupCtx    func() context.Context
		setupEnv    func()
		wantErr     bool
		wantService bool // true if we expect a specific mock service
	}{
		{
			name: "service from context",
			setupCtx: func() context.Context {
				mockService := mocks.NewMockChatGPTServicer(t)
				return context.WithValue(context.Background(), ChatGPTServiceKey, mockService)
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantErr:     false,
			wantService: true,
		},
		{
			name: "create new service",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantErr:     false,
			wantService: false,
		},
		{
			name: "invalid service type in context",
			setupCtx: func() context.Context {
				// Put a non-ChatGPTServicer value in context
				return context.WithValue(context.Background(), ChatGPTServiceKey, "not a service")
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantErr:     false, // Should not error, just create new service
			wantService: false,
		},
		{
			name: "nil context",
			setupCtx: func() context.Context {
				return nil
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantErr: true,
		},
		{
			name: "no api key set",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupEnv: func() {
				if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
					t.Fatalf("failed to unset API key: %v", err)
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			tt.setupEnv()

			// Set up context
			var ctx context.Context
			if tt.setupCtx != nil {
				ctx = tt.setupCtx()
			}

			service, err := module.getChatGPTService(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, service)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, service)

			if tt.wantService {
				// Check if it's our mock service
				_, ok := service.(*mocks.MockChatGPTServicer)
				assert.True(t, ok, "expected mock service but got %T", service)
			} else {
				// Check if it's a real service
				_, ok := service.(*services.ChatGPTService)
				assert.True(t, ok, "expected real service but got %T", service)
			}
		})
	}
}

func TestValidateTimestamp(t *testing.T) {
	// Store original function
	origMatchString := regexMatchString
	defer func() {
		// Restore original function
		regexMatchString = origMatchString
	}()

	tests := []struct {
		name      string
		timestamp string
		wantErr   bool
		errMsg    string
		mockRegex bool // whether to mock regex function to return error
	}{
		{
			name:      "valid timestamp",
			timestamp: "12:34:56",
			wantErr:   false,
		},
		{
			name:      "valid timestamp with zeros",
			timestamp: "00:00:00",
			wantErr:   false,
		},
		{
			name:      "valid timestamp max values",
			timestamp: "23:59:59",
			wantErr:   false,
		},
		{
			name:      "invalid format - missing colons",
			timestamp: "123456",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 123456 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - wrong separators",
			timestamp: "12-34-56",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12-34-56 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - too many parts",
			timestamp: "12:34:56:78",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:34:56:78 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - too few parts",
			timestamp: "12:34",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:34 (expected HH:MM:SS)",
		},
		{
			name:      "invalid hours - too high",
			timestamp: "24:00:00",
			wantErr:   true,
			errMsg:    "invalid hours in timestamp: 24:00:00 (must be 00-23)",
		},
		{
			name:      "invalid hours - negative",
			timestamp: "-1:00:00",
			wantErr:   true,
			errMsg:    "invalid timestamp format: -1:00:00 (expected HH:MM:SS)",
		},
		{
			name:      "invalid minutes - too high",
			timestamp: "12:60:00",
			wantErr:   true,
			errMsg:    "invalid minutes in timestamp: 12:60:00 (must be 00-59)",
		},
		{
			name:      "invalid minutes - negative",
			timestamp: "12:-1:00",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:-1:00 (expected HH:MM:SS)",
		},
		{
			name:      "invalid seconds - too high",
			timestamp: "12:34:60",
			wantErr:   true,
			errMsg:    "invalid seconds in timestamp: 12:34:60 (must be 00-59)",
		},
		{
			name:      "invalid seconds - negative",
			timestamp: "12:34:-1",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:34:-1 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - non-numeric values",
			timestamp: "ab:cd:ef",
			wantErr:   true,
			errMsg:    "invalid timestamp format: ab:cd:ef (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - empty string",
			timestamp: "",
			wantErr:   true,
			errMsg:    "invalid timestamp format:  (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - single digit hours",
			timestamp: "1:23:45",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 1:23:45 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - single digit minutes",
			timestamp: "12:3:45",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:3:45 (expected HH:MM:SS)",
		},
		{
			name:      "invalid format - single digit seconds",
			timestamp: "12:34:5",
			wantErr:   true,
			errMsg:    "invalid timestamp format: 12:34:5 (expected HH:MM:SS)",
		},
		{
			name:      "regex match error",
			timestamp: "12:34:56",
			mockRegex: true,
			wantErr:   true,
			errMsg:    "failed to validate timestamp format: mock regex error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockRegex {
				// Mock regex function to return error
				regexMatchString = func(pattern string, s string) (bool, error) {
					return false, fmt.Errorf("mock regex error")
				}
			} else {
				// Use original function
				regexMatchString = origMatchString
			}

			err := validateTimestamp(tt.timestamp)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateShortClip(t *testing.T) {
	tests := []struct {
		name    string
		clip    *ShortClip
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid clip",
			clip: &ShortClip{
				Title:       "Test Title",
				StartTime:   "00:00:00",
				EndTime:     "00:01:00",
				Description: "Test Description",
				Tags:        "test, tags",
				ShortTitle:  "Short Title",
			},
			wantErr: false,
		},
		{
			name:    "nil clip",
			clip:    nil,
			wantErr: true,
			errMsg:  "short clip cannot be nil",
		},
		{
			name: "missing title",
			clip: &ShortClip{
				StartTime: "00:00:00",
				EndTime:   "00:01:00",
			},
			wantErr: true,
			errMsg:  "short clip title is required",
		},
		{
			name: "missing start time",
			clip: &ShortClip{
				Title:   "Test Title",
				EndTime: "00:01:00",
			},
			wantErr: true,
			errMsg:  "short clip start time is required",
		},
		{
			name: "missing end time",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
			},
			wantErr: true,
			errMsg:  "short clip end time is required",
		},
		{
			name: "invalid start time format",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "invalid",
				EndTime:   "00:01:00",
			},
			wantErr: true,
			errMsg:  "invalid start time: invalid timestamp format: invalid (expected HH:MM:SS)",
		},
		{
			name: "invalid end time format",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "invalid",
			},
			wantErr: true,
			errMsg:  "invalid end time: invalid timestamp format: invalid (expected HH:MM:SS)",
		},
		{
			name: "invalid start time hours",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "24:00:00",
				EndTime:   "00:01:00",
			},
			wantErr: true,
			errMsg:  "invalid start time: invalid hours in timestamp: 24:00:00 (must be 00-23)",
		},
		{
			name: "invalid start time minutes",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:60:00",
				EndTime:   "00:01:00",
			},
			wantErr: true,
			errMsg:  "invalid start time: invalid minutes in timestamp: 00:60:00 (must be 00-59)",
		},
		{
			name: "invalid start time seconds",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:60",
				EndTime:   "00:01:00",
			},
			wantErr: true,
			errMsg:  "invalid start time: invalid seconds in timestamp: 00:00:60 (must be 00-59)",
		},
		{
			name: "invalid end time hours",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "24:00:00",
			},
			wantErr: true,
			errMsg:  "invalid end time: invalid hours in timestamp: 24:00:00 (must be 00-23)",
		},
		{
			name: "invalid end time minutes",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "00:60:00",
			},
			wantErr: true,
			errMsg:  "invalid end time: invalid minutes in timestamp: 00:60:00 (must be 00-59)",
		},
		{
			name: "invalid end time seconds",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "00:00:60",
			},
			wantErr: true,
			errMsg:  "invalid end time: invalid seconds in timestamp: 00:00:60 (must be 00-59)",
		},
		{
			name: "end time before start time",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "01:00:00",
				EndTime:   "00:59:59",
			},
			wantErr: true,
			errMsg:  "end time (00:59:59) must be after start time (01:00:00)",
		},
		{
			name: "end time equals start time",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "01:00:00",
				EndTime:   "01:00:00",
			},
			wantErr: true,
			errMsg:  "end time (01:00:00) must be after start time (01:00:00)",
		},
		{
			name: "invalid start time - non-numeric",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "aa:00:00",
				EndTime:   "01:00:00",
			},
			wantErr: true,
			errMsg:  "invalid start time: invalid timestamp format: aa:00:00 (expected HH:MM:SS)",
		},
		{
			name: "invalid end time - non-numeric",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "aa:00:00",
			},
			wantErr: true,
			errMsg:  "invalid end time: invalid timestamp format: aa:00:00 (expected HH:MM:SS)",
		},
		{
			name: "valid clip - minimal time difference",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "00:00:01",
			},
			wantErr: false,
		},
		{
			name: "valid clip - large time difference",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:00:00",
				EndTime:   "23:59:59",
			},
			wantErr: false,
		},
		{
			name: "valid clip - cross hour boundary",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "01:59:59",
				EndTime:   "02:00:00",
			},
			wantErr: false,
		},
		{
			name: "valid clip - cross minute boundary",
			clip: &ShortClip{
				Title:     "Test Title",
				StartTime: "00:01:59",
				EndTime:   "00:02:00",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShortClip(tt.clip)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
