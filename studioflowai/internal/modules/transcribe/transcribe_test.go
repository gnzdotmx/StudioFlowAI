package transcribe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCommandExecutor is a mock implementation of CommandExecutor
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) ExecuteCommand(ctx context.Context, name string, args []string) ([]byte, error) {
	// Call the mock with just the name and args
	ret := m.Called(name, args)
	return ret.Get(0).([]byte), ret.Error(1)
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	ret := m.Called(file)
	return ret.String(0), ret.Error(1)
}

// Helper function to create test files
func createTestFile(t *testing.T, path string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "transcribe", module.Name())
}

func TestModule_Validate(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "transcribe_validate_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create test files
	testWavFile := filepath.Join(tempDir, "test.wav")
	testInvalidFile := filepath.Join(tempDir, "test.invalid")
	outputDir := filepath.Join(tempDir, "output")

	createTestFile(t, testWavFile)
	createTestFile(t, testInvalidFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing required input",
			params: map[string]interface{}{
				"output": outputDir,
			},
			wantErr: true,
		},
		{
			name: "missing required output",
			params: map[string]interface{}{
				"input": testWavFile,
			},
			wantErr: true,
		},
		{
			name: "invalid output format",
			params: map[string]interface{}{
				"input":        testWavFile,
				"output":       outputDir,
				"outputFormat": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid file extension",
			params: map[string]interface{}{
				"input":  testInvalidFile,
				"output": outputDir,
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

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	assert.Len(t, io.RequiredInputs, 2)
	assert.Len(t, io.OptionalInputs, 5)
	assert.Len(t, io.ProducedOutputs, 1)

	// Verify required inputs
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)

	// Verify optional inputs
	optionalInputNames := []string{"model", "language", "outputFormat", "whisperParams", "outputFileName"}
	for i, name := range optionalInputNames {
		assert.Equal(t, name, io.OptionalInputs[i].Name)
	}

	// Verify produced outputs
	assert.Equal(t, "transcript", io.ProducedOutputs[0].Name)
}

func TestSortNaturally(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "numeric sorting",
			input:    []string{"file10.wav", "file2.wav", "file1.wav"},
			expected: []string{"file1.wav", "file2.wav", "file10.wav"},
		},
		{
			name:     "mixed content sorting",
			input:    []string{"split_10.wav", "split_2.wav", "split_1.wav"},
			expected: []string{"split_1.wav", "split_2.wav", "split_10.wav"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortNaturally(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestContainsParam(t *testing.T) {
	tests := []struct {
		name     string
		params   []string
		param    string
		expected bool
	}{
		{
			name:     "param exists",
			params:   []string{"--model", "large-v2", "--beam_size", "5"},
			param:    "--beam_size",
			expected: true,
		},
		{
			name:     "param does not exist",
			params:   []string{"--model", "large-v2", "--beam_size", "5"},
			param:    "--temperature",
			expected: false,
		},
		{
			name:     "empty params",
			params:   []string{},
			param:    "--beam_size",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsParam(tt.params, tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "copy_file_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create source file
	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "destination.txt")

	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		src           string
		dst           string
		expectedError bool
	}{
		{
			name:          "successful copy",
			src:           srcFile,
			dst:           dstFile,
			expectedError: false,
		},
		{
			name:          "source file does not exist",
			src:           "nonexistent.txt",
			dst:           dstFile,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := copyFile(tt.src, tt.dst)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify file was copied
				copiedContent, err := os.ReadFile(tt.dst)
				assert.NoError(t, err)
				assert.Equal(t, content, copiedContent)
			}
		})
	}
}

func TestSplitIntoChunks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "valid input",
			input:    "1\n2\n3\n4\n5\n6",
			expected: []string{"1", "\n", "2", "\n", "3", "\n", "4", "\n", "5", "\n", "6"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "single line",
			input:    "1",
			expected: []string{"1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitIntoChunks(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		h        int
		m        int
		s        int
		ms       int
		expected string
	}{
		{
			name:     "valid timestamp",
			h:        1,
			m:        30,
			s:        45,
			ms:       500,
			expected: "01:30:45,500",
		},
		{
			name:     "zero values",
			h:        0,
			m:        0,
			s:        0,
			ms:       0,
			expected: "00:00:00,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimestamp(tt.h, tt.m, tt.s, tt.ms)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestForceMemoryCleanup(t *testing.T) {
	// This is a simple test to ensure the function doesn't panic
	t.Run("does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			forceMemoryCleanup()
		})
	})
}

func TestWaitForMemoryCleanup(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		expectedError bool
	}{
		{
			name:          "timeout occurs",
			timeout:       100 * time.Millisecond,
			expectedError: true,
		},
		{
			name:          "zero timeout",
			timeout:       0,
			expectedError: true,
		},
		{
			name:          "negative timeout",
			timeout:       -1 * time.Second,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			err := waitForMemoryCleanup(ctx)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNaturalLess(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "numeric comparison",
			a:        "file1",
			b:        "file2",
			expected: true,
		},
		{
			name:     "numeric comparison with different lengths",
			a:        "file2",
			b:        "file10",
			expected: true,
		},
		{
			name:     "same strings",
			a:        "file1",
			b:        "file1",
			expected: false,
		},
		{
			name:     "non-numeric comparison",
			a:        "filea",
			b:        "fileb",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := naturalLess(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		wantH     int
		wantM     int
		wantS     int
		wantMs    int
		wantErr   bool
	}{
		{
			name:      "valid timestamp",
			timestamp: "01:30:45,500",
			wantH:     1,
			wantM:     30,
			wantS:     45,
			wantMs:    500,
			wantErr:   false,
		},
		{
			name:      "invalid format",
			timestamp: "1:30:45.500",
			wantErr:   true,
		},
		{
			name:      "invalid hours",
			timestamp: "100:30:45,500",
			wantErr:   true,
		},
		{
			name:      "invalid minutes",
			timestamp: "01:60:45,500",
			wantErr:   true,
		},
		{
			name:      "invalid seconds",
			timestamp: "01:30:60,500",
			wantErr:   true,
		},
		{
			name:      "invalid milliseconds",
			timestamp: "01:30:45,1000",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, m, s, ms, err := parseTimestamp(tt.timestamp)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantH, h)
				assert.Equal(t, tt.wantM, m)
				assert.Equal(t, tt.wantS, s)
				assert.Equal(t, tt.wantMs, ms)
			}
		})
	}
}

func TestAdjustTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   string
		offsetSecs  int
		expected    string
		expectError bool
	}{
		{
			name:        "add 60 seconds",
			timestamp:   "00:00:30,000",
			offsetSecs:  60,
			expected:    "00:01:30,000",
			expectError: false,
		},
		{
			name:        "add 3600 seconds (1 hour)",
			timestamp:   "00:30:00,000",
			offsetSecs:  3600,
			expected:    "01:30:00,000",
			expectError: false,
		},
		{
			name:        "subtract 30 seconds",
			timestamp:   "00:01:00,000",
			offsetSecs:  -30,
			expected:    "00:00:30,000",
			expectError: false,
		},
		{
			name:        "invalid timestamp",
			timestamp:   "invalid",
			offsetSecs:  60,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adjustTimestamp(tt.timestamp, tt.offsetSecs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
