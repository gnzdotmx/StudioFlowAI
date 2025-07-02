package correcttranscript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	services "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	chatgptmocks "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "correct_transcript", module.Name())
}

func TestModule_Validate(t *testing.T) {
	module := New()
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.txt")
	outputDir := filepath.Join(tempDir, "output")
	promptFile := filepath.Join(tempDir, "prompt.yaml")

	// Create test files
	err := os.WriteFile(inputFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(promptFile, []byte("role: transcription corrector"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid params",
			params: map[string]interface{}{
				"input":          inputFile,
				"output":         outputDir,
				"promptTemplate": promptFile,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output": outputDir,
			},
			wantErr: true,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":  "nonexistent.txt",
				"output": outputDir,
			},
			wantErr: true,
		},
		{
			name: "invalid prompt template",
			params: map[string]interface{}{
				"input":          inputFile,
				"output":         outputDir,
				"promptTemplate": "nonexistent.yaml",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestModule_Validate_Errors(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr string
	}{
		{
			name: "invalid params type",
			params: map[string]interface{}{
				"input":  123, // Should be string
				"output": 456, // Should be string
			},
			wantErr: "error unmarshaling params",
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":  "input.txt",
				"output": "/nonexistent/path/that/cannot/be/created",
			},
			wantErr: "input path does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := New()
			err := module.Validate(tt.params)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestModule_Execute(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.txt")
	outputDir := filepath.Join(tempDir, "output")

	// Create test input file
	inputContent := "This is a test transcript.\nIt needs correction."
	err := os.WriteFile(inputFile, []byte(inputContent), 0644)
	require.NoError(t, err)

	// Create output directory
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	// Set up test environment
	t.Setenv("OPENAI_API_KEY", "test-key")

	// Create mock ChatGPT service
	mockService := chatgptmocks.NewMockChatGPTServicer(t)

	// Set up mock expectations
	mockService.On("GetContent", mock.Anything, mock.Anything, mock.MatchedBy(func(opts services.CompletionOptions) bool {
		return opts.Model == "gpt-4" && opts.Temperature == 0.1
	})).Return("This is a corrected test transcript.\nIt has been fixed.", nil)

	// Create test module with mock service
	module := &Module{chatGPTService: mockService}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "successful execution",
			params: map[string]interface{}{
				"input":            inputFile,
				"output":           outputDir,
				"model":            "gpt-4",
				"temperature":      0.1,
				"maxTokens":        4000,
				"targetLanguage":   "English",
				"requestTimeoutMs": 5000,
				"chunkSize":        1000,
			},
			wantErr: false,
		},
		{
			name: "missing required params",
			params: map[string]interface{}{
				"input": inputFile,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result.Outputs["corrected"])

				// Verify the output file exists
				outputFile := result.Outputs["corrected"]
				_, err := os.Stat(outputFile)
				assert.NoError(t, err)

				// Verify statistics
				assert.NotEmpty(t, result.Statistics["model"])
				assert.NotEmpty(t, result.Statistics["processTime"])
			}
		})
	}

	// Verify all mock expectations were met
	mockService.AssertExpectations(t)
}

func TestModule_Execute_Errors(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "nonexistent")
	inputDir := filepath.Join(tempDir, "input")
	require.NoError(t, os.MkdirAll(inputDir, 0755))

	tests := []struct {
		name       string
		setupFiles func() map[string]interface{}
		wantErr    string
	}{
		{
			name: "invalid params type",
			setupFiles: func() map[string]interface{} {
				return map[string]interface{}{
					"input":  123, // Should be string
					"output": 456, // Should be string
				}
			},
			wantErr: "error unmarshaling params",
		},
		{
			name: "input file not found",
			setupFiles: func() map[string]interface{} {
				return map[string]interface{}{
					"input":  filepath.Join(nonExistentDir, "nonexistent.txt"),
					"output": tempDir,
				}
			},
			wantErr: "input file not found",
		},
		{
			name: "input is directory",
			setupFiles: func() map[string]interface{} {
				return map[string]interface{}{
					"input":  inputDir,
					"output": tempDir,
				}
			},
			wantErr: "input must be a file, not a directory",
		},
		{
			name: "custom output filename",
			setupFiles: func() map[string]interface{} {
				inputFile := filepath.Join(tempDir, "test.txt")
				require.NoError(t, os.WriteFile(inputFile, []byte("test content"), 0644))
				return map[string]interface{}{
					"input":          inputFile,
					"output":         tempDir,
					"outputFileName": "custom_output",
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := New()
			result, err := module.Execute(context.Background(), tt.setupFiles())
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				if tt.name == "custom output filename" {
					assert.Contains(t, result.Outputs["corrected"], "custom_output.txt")
				}
			}
		})
	}
}

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Test required inputs
	assert.Len(t, io.RequiredInputs, 2)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)

	// Test optional inputs
	assert.Contains(t, getOptionalInputNames(io), "outputFileName")
	assert.Contains(t, getOptionalInputNames(io), "promptTemplate")
	assert.Contains(t, getOptionalInputNames(io), "model")
	assert.Contains(t, getOptionalInputNames(io), "targetLanguage")

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "corrected", io.ProducedOutputs[0].Name)
}

func getOptionalInputNames(io modules.ModuleIO) []string {
	names := make([]string, len(io.OptionalInputs))
	for i, input := range io.OptionalInputs {
		names[i] = input.Name
	}
	return names
}

func TestModule_FormatYAMLPrompt(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		want     string
		wantErr  bool
	}{
		{
			name: "full yaml prompt",
			yamlData: `
title: Transcript Correction Assistant
role: professional transcript editor
context:
  description: This is a specialized tool for correcting transcription errors.
  error_sources:
    - Multilingual context misinterpretation
    - Technical term confusion
    - Speech recognition errors
instructions:
  description: Please follow these guidelines when correcting the transcript.
  tasks:
    - Fix any misinterpreted technical terms
    - Maintain original meaning while improving clarity
    - Correct grammatical errors
  examples:
    - "Original: 'The haiti team fixed the bug' → Corrected: 'The IT team fixed the bug'"
important_guidelines:
  - Preserve technical accuracy
  - Maintain speaker intent
  - Keep formatting consistent
final_instruction: Please review and correct the following transcript text.`,
			want: `# Transcript Correction Assistant

You are a professional transcript editor. This is a specialized tool for correcting transcription errors.
- Multilingual context misinterpretation
- Technical term confusion
- Speech recognition errors

Please follow these guidelines when correcting the transcript.
1. Fix any misinterpreted technical terms
2. Maintain original meaning while improving clarity
3. Correct grammatical errors

   - Example: Original: 'The haiti team fixed the bug' → Corrected: 'The IT team fixed the bug'

Important:
- Preserve technical accuracy
- Maintain speaker intent
- Keep formatting consistent

Please review and correct the following transcript text.
`,
			wantErr: false,
		},
		{
			name: "minimal yaml prompt",
			yamlData: `
role: editor
context:
  description: Basic transcript correction tool.
final_instruction: Please correct the text.`,
			want: `You are a editor. Basic transcript correction tool.
Please correct the text.
`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			yamlData: `
role: - invalid: yaml
  format: here
- broken: structure`,
			want:    "",
			wantErr: true,
		},
	}

	module := &Module{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := module.formatYAMLPrompt([]byte(tt.yamlData))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModule_LoadPromptTemplate(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	yamlPrompt := `
title: Test Prompt
role: test editor
context:
  description: Test description
final_instruction: Test instruction`

	markdownPrompt := `# Test Prompt
You are a test editor.
Please follow these instructions:
1. First step
2. Second step`

	plainPrompt := "Simple plain text prompt\nWith multiple lines"

	yamlPath := filepath.Join(tempDir, "prompt.yaml")
	mdPath := filepath.Join(tempDir, "prompt.md")
	txtPath := filepath.Join(tempDir, "prompt.txt")
	nonexistentPath := filepath.Join(tempDir, "nonexistent.yaml")

	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlPrompt), 0644))
	require.NoError(t, os.WriteFile(mdPath, []byte(markdownPrompt), 0644))
	require.NoError(t, os.WriteFile(txtPath, []byte(plainPrompt), 0644))

	tests := []struct {
		name         string
		templatePath string
		want         string
		wantErr      bool
	}{
		{
			name:         "empty path returns default prompt",
			templatePath: "",
			want: "You are a helpful assistant that corrects transcript errors. " +
				"Please fix any transcription mistakes, especially words that might have been " +
				"misinterpreted due to multilingual context. For example, the word 'haiti' might actually be 'IT' " +
				"when the context is about technology in Spanish. Keep the meaning intact and improve readability. " +
				"Here is the transcript text to correct:",
			wantErr: false,
		},
		{
			name:         "yaml prompt file",
			templatePath: yamlPath,
			want:         "# Test Prompt\n\nYou are a test editor. Test description\nTest instruction\n",
			wantErr:      false,
		},
		{
			name:         "markdown prompt file",
			templatePath: mdPath,
			want:         markdownPrompt,
			wantErr:      false,
		},
		{
			name:         "plain text prompt file",
			templatePath: txtPath,
			want:         plainPrompt,
			wantErr:      false,
		},
		{
			name:         "nonexistent file",
			templatePath: nonexistentPath,
			want:         "",
			wantErr:      true,
		},
		{
			name:         "invalid yaml file",
			templatePath: filepath.Join(tempDir, "invalid.yaml"),
			want:         "",
			wantErr:      true,
		},
	}

	// Create invalid YAML file after defining tests
	invalidYAML := "invalid:\n\tyaml:\n\t\t- content: [broken"
	require.NoError(t, os.WriteFile(tests[5].templatePath, []byte(invalidYAML), 0644))

	module := &Module{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := module.loadPromptTemplate(tt.templatePath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModule_GetChatGPTService(t *testing.T) {
	tests := []struct {
		name            string
		setupEnv        func()
		existingService bool
		wantErr         bool
	}{
		{
			name: "create new service with valid API key",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			existingService: false,
			wantErr:         false,
		},
		{
			name: "return existing service",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			existingService: true,
			wantErr:         false,
		},
		{
			name: "fail to create service without API key",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "")
			},
			existingService: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset environment before each test
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Errorf("Failed to unset OPENAI_API_KEY: %v", err)
			}

			// Setup test environment
			tt.setupEnv()

			module := &Module{}

			if tt.existingService {
				// First create a service that should be cached
				service, err := module.getChatGPTService()
				require.NoError(t, err)
				require.NotNil(t, service)

				// Store the original service for comparison
				originalService := module.chatGPTService

				// Try getting the service again - should return the same instance
				secondService, err := module.getChatGPTService()
				assert.NoError(t, err)
				assert.NotNil(t, secondService)
				assert.Same(t, originalService, secondService, "should return the cached service instance")
			} else {
				// Try getting a new service
				service, err := module.getChatGPTService()
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, service)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, service)
					assert.NotNil(t, module.chatGPTService, "service should be cached")
				}
			}
		})
	}
}

func TestModule_ProcessFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	textContent := "This is a test transcript.\nIt needs correction."
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03} // Non-text content

	textFile := filepath.Join(tempDir, "input.txt")
	binaryFile := filepath.Join(tempDir, "binary.dat")
	outputDir := filepath.Join(tempDir, "output")
	outputFile := filepath.Join(outputDir, "output.txt")

	require.NoError(t, os.WriteFile(textFile, []byte(textContent), 0644))
	require.NoError(t, os.WriteFile(binaryFile, binaryContent, 0644))
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	tests := []struct {
		name           string
		setupEnv       func()
		setupMock      func(*chatgptmocks.MockChatGPTServicer)
		inputFile      string
		outputFile     string
		promptTemplate string
		params         Params
		wantErr        bool
		errorContains  string
	}{
		{
			name: "successful processing with API key",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				m.On("GetContent", mock.Anything, mock.Anything, mock.Anything).
					Return("Corrected: This is a test transcript.", nil)
			},
			inputFile:  textFile,
			outputFile: outputFile,
			params: Params{
				Model:            "gpt-4",
				Temperature:      0.1,
				MaxTokens:        4000,
				TargetLanguage:   "English",
				RequestTimeoutMS: 5000,
				ChunkSize:        1000,
			},
			wantErr: false,
		},
		{
			name: "process without API key - copy original",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				// No mock needed as service won't be called
			},
			inputFile:  textFile,
			outputFile: outputFile,
			params: Params{
				Model:            "gpt-4",
				Temperature:      0.1,
				MaxTokens:        4000,
				TargetLanguage:   "English",
				RequestTimeoutMS: 5000,
				ChunkSize:        1000,
			},
			wantErr: false,
		},
		{
			name: "fail on binary file",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				// No mock needed as it should fail before API call
			},
			inputFile:     binaryFile,
			outputFile:    outputFile,
			wantErr:       true,
			errorContains: "appears to be binary",
		},
		{
			name: "fail on API error",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				m.On("GetContent", mock.Anything, mock.Anything, mock.Anything).
					Return("", fmt.Errorf("API error: rate limit exceeded"))
			},
			inputFile:     textFile,
			outputFile:    outputFile,
			wantErr:       true,
			errorContains: "API error",
		},
		{
			name: "fail on invalid output path",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				m.On("GetContent", mock.Anything, mock.Anything, mock.Anything).
					Return("Corrected text", nil)
			},
			inputFile:     textFile,
			outputFile:    filepath.Join(tempDir, "nonexistent", "output.txt"),
			wantErr:       true,
			errorContains: "failed to write output file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset environment
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Errorf("Failed to unset OPENAI_API_KEY: %v", err)
			}
			tt.setupEnv()

			// Create mock service
			mockService := chatgptmocks.NewMockChatGPTServicer(t)
			tt.setupMock(mockService)

			// Create module with mock service
			module := &Module{chatGPTService: mockService}

			// Process file
			err := module.processFile(context.Background(), tt.inputFile, tt.outputFile, tt.promptTemplate, tt.params)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				// Verify output file exists and contains expected content
				content, err := os.ReadFile(tt.outputFile)
				assert.NoError(t, err)
				assert.NotEmpty(t, content)

				if os.Getenv("OPENAI_API_KEY") == "" {
					// Without API key, should contain original content
					assert.Equal(t, textContent, string(content))
				}
			}

			// Clean up output file between tests
			if err := os.Remove(tt.outputFile); err != nil && !os.IsNotExist(err) {
				t.Errorf("Failed to remove output file: %v", err)
			}
		})
	}
}

func TestModule_ProcessFile_Errors(t *testing.T) {
	tempDir := t.TempDir()
	unreadableFile := filepath.Join(tempDir, "unreadable.txt")
	unwriteableDir := filepath.Join(tempDir, "unwriteable")

	// Create a text file with some content first
	require.NoError(t, os.WriteFile(unreadableFile, []byte("This is a text file that will be unreadable"), 0644))
	// Then make it unreadable
	require.NoError(t, os.Chmod(unreadableFile, 0000))

	// Create an unwriteable directory
	require.NoError(t, os.MkdirAll(unwriteableDir, 0555))

	tests := []struct {
		name           string
		setupEnv       func()
		setupMock      func(*chatgptmocks.MockChatGPTServicer)
		inputPath      string
		outputPath     string
		promptTemplate string
		params         Params
		wantErr        string
	}{
		{
			name: "failed to read transcript file",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "test-key")
			},
			setupMock:  func(mock *chatgptmocks.MockChatGPTServicer) {},
			inputPath:  unreadableFile,
			outputPath: filepath.Join(tempDir, "output.txt"),
			wantErr:    "appears to be binary",
		},
		{
			name: "failed to write output file without API key",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "")
			},
			setupMock:  func(mock *chatgptmocks.MockChatGPTServicer) {},
			inputPath:  filepath.Join(tempDir, "input.txt"),
			outputPath: filepath.Join(unwriteableDir, "output.txt"),
			wantErr:    "failed to write output file",
		},
		{
			name: "failed to initialize ChatGPT service",
			setupEnv: func() {
				t.Setenv("OPENAI_API_KEY", "invalid-key")
			},
			setupMock: func(m *chatgptmocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.MatchedBy(func(ctx context.Context) bool { return true }),
					mock.MatchedBy(func(msgs []services.ChatMessage) bool { return true }),
					mock.MatchedBy(func(opts services.CompletionOptions) bool { return true })).
					Return("", fmt.Errorf("failed to initialize service"))
			},
			inputPath:  filepath.Join(tempDir, "input.txt"),
			outputPath: filepath.Join(tempDir, "output.txt"),
			wantErr:    "ChatGPT API request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset environment
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Errorf("Failed to unset OPENAI_API_KEY: %v", err)
			}
			tt.setupEnv()

			// Create input file if it doesn't exist
			if tt.inputPath != unreadableFile {
				require.NoError(t, os.WriteFile(tt.inputPath, []byte("test content"), 0644))
			}

			// Create mock service
			mockService := chatgptmocks.NewMockChatGPTServicer(t)
			tt.setupMock(mockService)

			// Create module with mock service
			module := &Module{chatGPTService: mockService}

			// Process file
			err := module.processFile(context.Background(), tt.inputPath, tt.outputPath, tt.promptTemplate, tt.params)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestModule_SplitTranscript_LargeChunks(t *testing.T) {
	// Create a large transcript with multiple paragraphs
	var transcript strings.Builder
	for i := 0; i < 10; i++ {
		// Each paragraph is about 100 characters (≈ 25 tokens)
		transcript.WriteString(fmt.Sprintf("This is paragraph %d with some content that needs to be processed by the ChatGPT API for correction and improvement.\n\n", i))
	}

	module := &Module{}
	chunks := module.splitTranscript(transcript.String(), 50) // Small chunk size to force splitting

	// Should split into multiple chunks since each paragraph is about 25 tokens
	assert.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		// Rough token estimate (4 chars ≈ 1 token)
		tokens := len(chunk) / 4
		assert.LessOrEqual(t, tokens, 50)
	}
}
