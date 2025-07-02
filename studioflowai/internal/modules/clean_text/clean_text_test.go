package cleantext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Test required inputs
	assert.Len(t, io.RequiredInputs, 2)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)

	// Test optional inputs
	assert.Len(t, io.OptionalInputs, 6)
	assert.Equal(t, "removePatterns", io.OptionalInputs[0].Name)
	assert.Equal(t, "cleanFileSuffix", io.OptionalInputs[1].Name)
	assert.Equal(t, "inputFileName", io.OptionalInputs[2].Name)
	assert.Equal(t, "outputFileName", io.OptionalInputs[3].Name)
	assert.Equal(t, "preserveTimestamps", io.OptionalInputs[4].Name)
	assert.Equal(t, "preserveLineBreaks", io.OptionalInputs[5].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "cleaned", io.ProducedOutputs[0].Name)
}

func TestModule_Validate(t *testing.T) {
	module := New()
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"input":  filepath.Join(tempDir, "input.txt"),
				"output": tempDir,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output": tempDir,
			},
			wantErr: true,
		},
		{
			name: "missing output",
			params: map[string]interface{}{
				"input": filepath.Join(tempDir, "input.txt"),
			},
			wantErr: true,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":  "/nonexistent/path/input.txt",
				"output": tempDir,
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

func TestModule_Execute(t *testing.T) {
	module := New()
	tempDir := t.TempDir()

	// Create test input file
	inputPath := filepath.Join(tempDir, "test.txt")
	inputContent := `Line 1 with (timestamp)
Line 2 with [brackets]
Line 3 with {curly braces}
Line 4 with <angle brackets>
Line 5 with (another timestamp)`

	err := os.WriteFile(inputPath, []byte(inputContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		params         map[string]interface{}
		expectedOutput string
		wantErr        bool
	}{
		{
			name: "basic text cleaning",
			params: map[string]interface{}{
				"input":  inputPath,
				"output": tempDir,
				"removePatterns": []string{
					`\[.*?\]`,
					`\{.*?\}`,
					`<.*?>`,
				},
			},
			expectedOutput: `Line 1 with (timestamp)
Line 2 with 
Line 3 with 
Line 4 with 
Line 5 with (another timestamp)
`,
			wantErr: false,
		},
		{
			name: "clean with timestamps",
			params: map[string]interface{}{
				"input":             inputPath,
				"output":            tempDir,
				"preserveTimestamp": true,
			},
			expectedOutput: `Line 1 with (timestamp)
Line 2 with [brackets]
Line 3 with {curly braces}
Line 4 with <angle brackets>
Line 5 with (another timestamp)
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, result.Outputs["cleaned"])

			// Read the output file
			outputContent, err := os.ReadFile(result.Outputs["cleaned"])
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, string(outputContent))
		})
	}
}

func TestModule_Execute_SRT(t *testing.T) {
	module := New()
	tempDir := t.TempDir()

	// Create test SRT file with various edge cases
	inputPath := filepath.Join(tempDir, "test.srt")
	inputContent := `1
00:00:01,000 --> 00:00:04,000
First subtitle with (timestamp)
Multiple lines in one subtitle
And another line

2
00:00:05,000 --> 00:00:08,000
Second subtitle with [brackets]
This is a continuation line

3
00:00:09,000 --> 00:00:12,000
Third subtitle with {curly braces}

4
Invalid timestamp format
This should be handled gracefully

5
00:00:15,000 --> 00:00:18,000
Fifth subtitle with empty lines


6
00:00:20,000 --> 00:00:23,000
Last subtitle with special characters: !@#$%^&*()
And some numbers: 123456789
`

	err := os.WriteFile(inputPath, []byte(inputContent), 0644)
	require.NoError(t, err)

	// Create a test file with very long lines
	longInputPath := filepath.Join(tempDir, "long.srt")
	longLine := strings.Repeat("x", 4000) // Create a line close to maxLineLength
	longContent := fmt.Sprintf(`1
00:00:01,000 --> 00:00:04,000
%s

2
00:00:05,000 --> 00:00:08,000
Normal line after long line
`, longLine)

	err = os.WriteFile(longInputPath, []byte(longContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		params         map[string]interface{}
		inputFile      string
		expectedOutput string
		wantErr        bool
	}{
		{
			name: "clean SRT with timestamps and multiple lines",
			params: map[string]interface{}{
				"input":             inputPath,
				"output":            tempDir,
				"preserveTimestamp": true,
			},
			inputFile: inputPath,
			expectedOutput: `1
00:00:01,000 --> 00:00:04,000
First subtitle with (timestamp)
Multiple lines in one subtitle
And another line

2
00:00:05,000 --> 00:00:08,000
Second subtitle with [brackets]
This is a continuation line

3
00:00:09,000 --> 00:00:12,000
Third subtitle with {curly braces}

4
Invalid timestamp format
This should be handled gracefully

5
00:00:15,000 --> 00:00:18,000
Fifth subtitle with empty lines

6
00:00:20,000 --> 00:00:23,000
Last subtitle with special characters: !@#$%^&*()
And some numbers: 123456789

`,
			wantErr: false,
		},
		{
			name: "clean SRT without timestamps and with patterns",
			params: map[string]interface{}{
				"input":             inputPath,
				"output":            tempDir,
				"preserveTimestamp": false,
				"removePatterns": []string{
					`\[.*?\]`,
					`\{.*?\}`,
					`[!@#$%^&*()]`,
				},
			},
			inputFile: inputPath,
			expectedOutput: `1
First subtitle with timestamp
Multiple lines in one subtitle
And another line

2
Second subtitle with 
This is a continuation line

3
Third subtitle with 

4
Invalid timestamp format
This should be handled gracefully

5
Fifth subtitle with empty lines

6
Last subtitle with special characters: 
And some numbers: 123456789

`,
			wantErr: false,
		},
		{
			name: "handle very long lines",
			params: map[string]interface{}{
				"input":             longInputPath,
				"output":            tempDir,
				"preserveTimestamp": true,
			},
			inputFile: longInputPath,
			expectedOutput: fmt.Sprintf(`1
00:00:01,000 --> 00:00:04,000
%s

2
00:00:05,000 --> 00:00:08,000
Normal line after long line

`, longLine),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, result.Outputs["cleaned"])

			// Read the output file
			outputContent, err := os.ReadFile(result.Outputs["cleaned"])
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, string(outputContent))
		})
	}
}

func TestIsSubtitleNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid number", "1", true},
		{"valid number with spaces", "  42  ", true},
		{"invalid number", "abc", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSubtitleNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			"valid timestamp",
			"00:00:01,000 --> 00:00:04,000",
			true,
		},
		{
			"valid timestamp with spaces",
			"  00:00:01,000 --> 00:00:04,000  ",
			true,
		},
		{
			"invalid timestamp",
			"not a timestamp",
			false,
		},
		{
			"empty string",
			"",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
