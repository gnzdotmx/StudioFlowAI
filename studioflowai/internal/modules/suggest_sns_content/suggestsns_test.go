package suggestsnscontent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	services "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	mocks "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock response for successful SNS content generation
const mockSuccessResponse = `sns_content_generation:
  title: "Test Title | Entrevista Exclusiva"
  description: |
    🚀 Test description with emojis
    
    #test #hashtags
  social_media:
    twitter: "Test tweet 🚀"
    instagram_facebook: "Test Instagram post"
    linkedin: "Test LinkedIn post"
  keywords: "test, keywords, content"
  timeline:
    - "00:00 - Introduction"
    - "05:00 - Main content"
    - "10:00 - Conclusion"`

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
func verifyPromptContent(content string, language string, transcript string) bool {
	// Check for required sections
	if !strings.Contains(content, "TÍTULO") {
		return false
	}

	// Check language specification
	if !strings.Contains(content, "Generar en: "+language) {
		return false
	}

	// Check transcript content
	if !strings.Contains(content, transcript) {
		return false
	}

	return true
}

func TestSuggestSNSModule(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "sns_test")
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
		mode    os.FileMode
	}{
		{"transcript.txt", "This is a test transcript content.", 0644},
		{"other.txt", "This should be ignored", 0644},
		{"transcript2.txt", "Another test transcript", 0644},
		{"binary.txt", string([]byte{0x00, 0x01, 0x02, 0x03}), 0644}, // Binary content
		{"readonly.txt", "Read-only file", 0400},                     // Read-only file
	}

	for _, tf := range testFiles {
		if err := os.WriteFile(filepath.Join(inputDir, tf.name), []byte(tf.content), tf.mode); err != nil {
			t.Fatal(err)
		}
	}

	// Create a read-only directory for testing write errors
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnlyDir, 0500); err != nil { // Read + execute, but no write
		t.Fatal(err)
	}

	// Create a custom prompt file
	customPromptPath := filepath.Join(tempDir, "custom_prompt.yaml")
	customPromptContent := `
introduction: "Test introduction"
title:
  length: "50-60 characters"
  description: "Create an impactful title"
  criteria:
    - "Capture main essence"
    - "Include search terms"
conclusion: "Format as YAML"`
	if err := os.WriteFile(customPromptPath, []byte(customPromptContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		params         map[string]interface{}
		setupMock      func(*mocks.MockChatGPTServicer)
		wantErr        bool
		expectedOutput string
		apiKeySet      bool
		errorContains  string
	}{
		{
			name: "successful content generation",
			params: map[string]interface{}{
				"input":     filepath.Join(inputDir, "transcript.txt"),
				"output":    outputDir,
				"model":     "gpt-4",
				"maxTokens": 8000,
				"language":  "Spanish",
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.MatchedBy(func(messages []services.ChatMessage) bool {
						if len(messages) != 2 {
							return false
						}
						return verifyPromptContent(messages[1].Content, "Spanish", "This is a test transcript content.")
					}),
					mock.MatchedBy(func(opts services.CompletionOptions) bool {
						return opts.Model == "gpt-4" && opts.MaxTokens == 8000
					}),
				).Return(mockSuccessResponse, nil)
			},
			apiKeySet:      true,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "transcript_SNS.yaml"),
		},
		{
			name: "no api key set",
			params: map[string]interface{}{
				"input":     filepath.Join(inputDir, "transcript.txt"),
				"output":    outputDir,
				"model":     "gpt-4",
				"maxTokens": 8000,
				"language":  "Spanish",
			},
			setupMock:      func(m *mocks.MockChatGPTServicer) {},
			apiKeySet:      false,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "transcript_SNS.yaml"),
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
			name: "custom output filename",
			params: map[string]interface{}{
				"input":          filepath.Join(inputDir, "transcript.txt"),
				"output":         outputDir,
				"outputFileName": "custom_output",
				"model":          "gpt-4",
				"language":       "Spanish",
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.MatchedBy(func(messages []services.ChatMessage) bool {
						return verifyPromptContent(messages[1].Content, "Spanish", "This is a test transcript content.")
					}),
					mock.Anything,
				).Return(mockSuccessResponse, nil)
			},
			apiKeySet:      true,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "custom_output.yaml"),
		},
		{
			name: "binary file error",
			params: map[string]interface{}{
				"input":    filepath.Join(inputDir, "binary.txt"),
				"output":   outputDir,
				"model":    "gpt-4",
				"language": "Spanish",
			},
			setupMock:     func(m *mocks.MockChatGPTServicer) {},
			apiKeySet:     true,
			wantErr:       true,
			errorContains: "appears to be binary",
		},
		{
			name: "api request timeout",
			params: map[string]interface{}{
				"input":            filepath.Join(inputDir, "transcript.txt"),
				"output":           outputDir,
				"model":            "gpt-4",
				"language":         "Spanish",
				"requestTimeoutMs": 1, // Very short timeout
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).WaitUntil(time.After(10*time.Millisecond)).Return("", context.DeadlineExceeded)
			},
			apiKeySet:     true,
			wantErr:       true,
			errorContains: "deadline exceeded",
		},
		{
			name: "api request error",
			params: map[string]interface{}{
				"input":    filepath.Join(inputDir, "transcript.txt"),
				"output":   outputDir,
				"model":    "gpt-4",
				"language": "Spanish",
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return("", errors.New("API error"))
			},
			apiKeySet:     true,
			wantErr:       true,
			errorContains: "API error",
		},
		{
			name: "write_file_error",
			params: map[string]interface{}{
				"input":    filepath.Join(inputDir, "transcript.txt"),
				"output":   filepath.Join(outputDir, "nonexistent"), // We'll make this directory read-only
				"model":    "gpt-4",
				"language": "Spanish",
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(mockSuccessResponse, nil).Maybe() // Make this optional since we might fail before reaching it
			},
			apiKeySet:     true,
			wantErr:       true,
			errorContains: "permission denied",
		},
		{
			name: "custom prompt file",
			params: map[string]interface{}{
				"input":          filepath.Join(inputDir, "transcript.txt"),
				"output":         outputDir,
				"model":          "gpt-4",
				"language":       "Spanish",
				"promptFilePath": customPromptPath,
			},
			setupMock: func(m *mocks.MockChatGPTServicer) {
				m.EXPECT().GetContent(
					mock.Anything,
					mock.MatchedBy(func(messages []services.ChatMessage) bool {
						return strings.Contains(messages[1].Content, "Test introduction") &&
							strings.Contains(messages[1].Content, "Create an impactful title")
					}),
					mock.Anything,
				).Return(mockSuccessResponse, nil)
			},
			apiKeySet:      true,
			wantErr:        false,
			expectedOutput: filepath.Join(outputDir, "transcript_SNS.yaml"),
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

			// Special setup for write_file_error test
			if tt.name == "write_file_error" {
				nonexistentDir := filepath.Join(outputDir, "nonexistent")
				if err := os.MkdirAll(nonexistentDir, 0755); err != nil {
					t.Fatalf("failed to create test directory: %v", err)
				}
				if err := os.Chmod(nonexistentDir, 0500); err != nil { // Read + execute, but no write
					t.Fatalf("failed to set directory permissions: %v", err)
				}
			}

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
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, result.Outputs["sns_content"])

			// Only verify file contents if we expect the file to be created successfully
			if tt.apiKeySet && !tt.wantErr {
				content, err := os.ReadFile(tt.expectedOutput)
				assert.NoError(t, err)
				assert.Contains(t, string(content), "sns_content_generation")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "sns_validate_test")
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
				"input":  testTranscriptPath,
				"output": outputDir,
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
			name: "missing output",
			params: map[string]interface{}{
				"input": testTranscriptPath,
			},
			wantErr: true,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":  "/nonexistent/path",
				"output": outputDir,
			},
			wantErr: true,
		},
		{
			name: "invalid prompt file path",
			params: map[string]interface{}{
				"input":          testTranscriptPath,
				"output":         outputDir,
				"promptFilePath": "/nonexistent/prompt.yaml",
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
	assert.True(t, len(io.OptionalInputs) >= 4)
	assert.Equal(t, "outputFileName", io.OptionalInputs[0].Name)
	assert.Equal(t, "promptFilePath", io.OptionalInputs[1].Name)
	assert.Equal(t, "model", io.OptionalInputs[2].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "sns_content", io.ProducedOutputs[0].Name)
	assert.Contains(t, io.ProducedOutputs[0].Patterns, ".yaml")
}

func TestFormatSNSYAMLPrompt(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    string
		wantErr bool
	}{
		{
			name: "complete yaml prompt",
			yaml: `
introduction: "Test introduction"
title:
  length: "50-60 characters"
  description: "Create an impactful title"
  criteria:
    - "Capture main essence"
    - "Include search terms"
description:
  length: "2000 characters max"
  description: "Write an engaging description"
  criteria:
    - "Start with hook"
    - "Include emojis"
social_media:
  description: "Create social media posts"
  platforms:
    - "Twitter (280 chars)"
    - "Instagram/Facebook (200 words)"
  requirements:
    - "Include key points"
    - "Use emojis"
keywords:
  count: "25-30 keywords"
  description: "SEO optimized keywords"
  criteria:
    - "Include popular terms"
    - "Mix short and long tail"
timeline:
  description: "Create detailed timeline"
  criteria:
    - "Mark important moments"
    - "Include timestamps"
  example: |
    Parte 1:
    00:00 - Intro
    05:00 - Main topic
conclusion: "Format as YAML"`,
			want:    "Test introduction\n\n## 1. TÍTULO (50-60 characters)\nCreate an impactful title\n- Capture main essence\n- Include search terms\n\n## 2. DESCRIPCIÓN PARA YOUTUBE (2000 characters max)\nWrite an engaging description\n- Start with hook\n- Include emojis\n\n## 3. COPY PARA REDES SOCIALES (3 VERSIONES)\nCreate social media posts\n- Twitter (280 chars)\n- Instagram/Facebook (200 words)\nCada versión debe incluir:\n- Include key points\n- Use emojis\n\n## 4. KEYWORDS PARA SEO (25-30 keywords)\nSEO optimized keywords\n- Include popular terms\n- Mix short and long tail\n\n## 5. TIMELINE DETALLADO\nCreate detailed timeline\n- Mark important moments\n- Include timestamps\n\nEjemplo:\n--------------------------------\nParte 1:\n00:00 - Intro\n05:00 - Main topic\n--------------------------------\n\nFormat as YAML\n",
			wantErr: false,
		},
		{
			name: "minimal yaml prompt",
			yaml: `
introduction: "Test introduction"
conclusion: "Format as YAML"`,
			want:    "Test introduction\n\nFormat as YAML\n",
			wantErr: false,
		},
		{
			name: "title with default length",
			yaml: `
title:
  description: "Create a title"
  criteria:
    - "Test criterion"`,
			want:    "## 1. TÍTULO (50-60 caracteres)\nCreate a title\n- Test criterion\n\n",
			wantErr: false,
		},
		{
			name: "description with default length",
			yaml: `
description:
  description: "Write description"
  criteria:
    - "Test criterion"`,
			want:    "## 2. DESCRIPCIÓN PARA YOUTUBE (2000 caracteres máx)\nWrite description\n- Test criterion\n\n",
			wantErr: false,
		},
		{
			name: "social media without requirements",
			yaml: `
social_media:
  description: "Create posts"
  platforms:
    - "Twitter"
    - "Instagram"`,
			want:    "## 3. COPY PARA REDES SOCIALES (3 VERSIONES)\nCreate posts\n- Twitter\n- Instagram\n\n",
			wantErr: false,
		},
		{
			name: "keywords with default count",
			yaml: `
keywords:
  description: "Generate keywords"
  criteria:
    - "Test criterion"`,
			want:    "## 4. KEYWORDS PARA SEO (25-30 keywords)\nGenerate keywords\n- Test criterion\n\n",
			wantErr: false,
		},
		{
			name: "timeline without example",
			yaml: `
timeline:
  description: "Create timeline"
  criteria:
    - "Test criterion"`,
			want:    "## 5. TIMELINE DETALLADO\nCreate timeline\n- Test criterion\n",
			wantErr: false,
		},
		{
			name: "invalid yaml",
			yaml: `
invalid:
  - [broken yaml
`,
			wantErr: true,
		},
		{
			name:    "empty yaml",
			yaml:    "",
			want:    "",
			wantErr: false,
		},
		{
			name: "non-string criteria",
			yaml: `
title:
  criteria:
    - 123
    - true`,
			want:    "## 1. TÍTULO (50-60 caracteres)\n\n",
			wantErr: false,
		},
		{
			name: "non-string description",
			yaml: `
title:
  description: 123`,
			want:    "## 1. TÍTULO (50-60 caracteres)\n\n",
			wantErr: false,
		},
		{
			name: "non-list criteria",
			yaml: `
title:
  criteria: "not a list"`,
			want:    "## 1. TÍTULO (50-60 caracteres)\n\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatSNSYAMLPrompt([]byte(tt.yaml))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got, "Unexpected formatted prompt")
		})
	}
}

func TestGetChatGPTService(t *testing.T) {
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
		wantMocked  bool
		errContains string
	}{
		{
			name: "nil context",
			setupCtx: func() context.Context {
				return nil
			},
			setupEnv:    func() {},
			wantErr:     true,
			errContains: "context cannot be nil",
		},
		{
			name: "context with mock service",
			setupCtx: func() context.Context {
				mockService := mocks.NewMockChatGPTServicer(t)
				return context.WithValue(context.Background(), ChatGPTServiceKey, mockService)
			},
			setupEnv:   func() {},
			wantMocked: true,
		},
		{
			name: "context without service - with API key",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-api-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantMocked: false,
		},
		{
			name: "context without service - no API key",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupEnv: func() {
				if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
					t.Fatalf("failed to unset API key: %v", err)
				}
			},
			wantErr:     true,
			errContains: "OPENAI_API_KEY environment variable is not set",
		},
		{
			name: "context with wrong type",
			setupCtx: func() context.Context {
				// Put a non-service value in the context
				return context.WithValue(context.Background(), ChatGPTServiceKey, "not a service")
			},
			setupEnv: func() {
				if err := os.Setenv("OPENAI_API_KEY", "test-api-key"); err != nil {
					t.Fatalf("failed to set API key: %v", err)
				}
			},
			wantMocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			tt.setupEnv()

			module := &Module{}
			ctx := tt.setupCtx()
			service, err := module.getChatGPTService(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, service)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, service)

			if tt.wantMocked {
				_, ok := service.(*mocks.MockChatGPTServicer)
				assert.True(t, ok, "Expected a mock service")
			} else {
				// Should be a real service
				_, ok := service.(*services.ChatGPTService)
				assert.True(t, ok, "Expected a real service")
			}
		})
	}
}
