package settitle2shortvideo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Save the original exec.Command
var originalExecCommand = execCommand

// TestMain sets up and tears down the mock command
func TestMain(m *testing.M) {
	// Run the tests
	result := m.Run()

	// Restore the original exec.Command
	execCommand = originalExecCommand

	os.Exit(result)
}

// fakeExecCommand creates a mock command that does nothing
func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is not a real test, it's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Create output file based on the last argument (output path)
	args := os.Args
	outputPath := args[len(args)-1]
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("mock video content"), 0644); err != nil {
		t.Fatalf("Failed to create mock output file: %v", err)
	}

	os.Exit(0)
}

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Test required inputs
	assert.Len(t, io.RequiredInputs, 2)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)

	// Test optional inputs
	assert.Len(t, io.OptionalInputs, 9)
	assert.Equal(t, "videoFile", io.OptionalInputs[0].Name)
	assert.Equal(t, "fontFile", io.OptionalInputs[1].Name)
	assert.Equal(t, "fontSize", io.OptionalInputs[2].Name)
	assert.Equal(t, "fontColor", io.OptionalInputs[3].Name)
	assert.Equal(t, "boxColor", io.OptionalInputs[4].Name)
	assert.Equal(t, "boxBorderW", io.OptionalInputs[5].Name)
	assert.Equal(t, "quietFlag", io.OptionalInputs[6].Name)
	assert.Equal(t, "textX", io.OptionalInputs[7].Name)
	assert.Equal(t, "textY", io.OptionalInputs[8].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "videos", io.ProducedOutputs[0].Name)
}

func TestModule_Validate(t *testing.T) {
	module := New()
	tempDir := t.TempDir()
	// Create a separate directory for input files
	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "output")
	err := os.MkdirAll(inputDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	// Create test files
	videoPath := filepath.Join(inputDir, "test.mp4")
	err = os.WriteFile(videoPath, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	yamlContent := []byte(`
sourceVideo: test.mp4
shorts:
  - title: "First Clip"
    startTime: "00:00:10"
    endTime: "00:00:20"
    description: "Test clip 1"
    tags: "#test #clip1"
    shortTitle: "Test Short 1"
`)

	yamlPath := filepath.Join(inputDir, "shorts_suggestions.yaml")
	err = os.WriteFile(yamlPath, yamlContent, 0644)
	require.NoError(t, err)

	fontPath := filepath.Join(inputDir, "test.ttf")
	err = os.WriteFile(fontPath, []byte("dummy font content"), 0644)
	require.NoError(t, err)

	// Create invalid YAML file
	invalidYamlPath := filepath.Join(inputDir, "invalid.yaml")
	err = os.WriteFile(invalidYamlPath, []byte("invalid: [yaml: content"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"input":     yamlPath,
				"output":    outputDir,
				"videoFile": videoPath,
				"fontFile":  fontPath,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output":    outputDir,
				"videoFile": videoPath,
			},
			wantErr: true,
		},
		{
			name: "missing output",
			params: map[string]interface{}{
				"input":     yamlPath,
				"videoFile": videoPath,
			},
			wantErr: true,
		},
		{
			name: "invalid yaml file",
			params: map[string]interface{}{
				"input":     invalidYamlPath,
				"output":    outputDir,
				"videoFile": videoPath,
			},
			wantErr: true,
		},
		{
			name: "invalid font file",
			params: map[string]interface{}{
				"input":     yamlPath,
				"output":    outputDir,
				"videoFile": videoPath,
				"fontFile":  "nonexistent.ttf",
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
	// Replace exec.Command with our mock
	execCommand = fakeExecCommand
	defer func() {
		execCommand = originalExecCommand
	}()

	module := New()
	tempDir := t.TempDir()

	// Create test video file
	videoPath := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(videoPath, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Create test font file
	fontPath := filepath.Join(tempDir, "test.ttf")
	err = os.WriteFile(fontPath, []byte("dummy font content"), 0644)
	require.NoError(t, err)

	// Create test YAML file
	yamlContent := []byte(`
sourceVideo: test.mp4
shorts:
  - title: "First Clip"
    startTime: "00:00:10"
    endTime: "00:00:20"
    description: "Test clip 1"
    tags: "#test #clip1"
    shortTitle: "Test Short 1"
  - title: "Second Clip"
    startTime: "00:01:00"
    endTime: "00:01:30"
    description: "Test clip 2"
    tags: "#test #clip2"
    shortTitle: "Test Short 2"
`)

	yamlPath := filepath.Join(tempDir, "shorts_suggestions.yaml")
	err = os.WriteFile(yamlPath, yamlContent, 0644)
	require.NoError(t, err)

	// Create input video clips that would normally be created by extract_shorts
	inputClip1 := filepath.Join(tempDir, "000010-000020.mp4")
	err = os.WriteFile(inputClip1, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	inputClip2 := filepath.Join(tempDir, "000100-000130.mp4")
	err = os.WriteFile(inputClip2, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Create output directory
	err = os.MkdirAll(tempDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name            string
		params          map[string]interface{}
		expectedOutputs []string
		wantErr         bool
	}{
		{
			name: "process shorts with default settings",
			params: map[string]interface{}{
				"input":     yamlPath,
				"output":    tempDir,
				"videoFile": videoPath,
				"fontFile":  fontPath,
				"quietFlag": true,
			},
			expectedOutputs: []string{
				filepath.Join(tempDir, "000010-000020-withtext.mp4"),
				filepath.Join(tempDir, "000100-000130-withtext.mp4"),
			},
			wantErr: false,
		},
		{
			name: "process shorts with custom settings",
			params: map[string]interface{}{
				"input":      yamlPath,
				"output":     tempDir,
				"videoFile":  videoPath,
				"fontFile":   fontPath,
				"fontSize":   32,
				"fontColor":  "yellow",
				"boxColor":   "black@0.7",
				"boxBorderW": 10,
				"textX":      "(w-text_w)/2",
				"textY":      "h-(2*text_h)",
				"quietFlag":  true,
			},
			expectedOutputs: []string{
				filepath.Join(tempDir, "000010-000020-withtext.mp4"),
				filepath.Join(tempDir, "000100-000130-withtext.mp4"),
			},
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
			assert.NotEmpty(t, result.Outputs)
			assert.Len(t, result.Outputs, len(tt.expectedOutputs))

			// Check statistics
			assert.NotNil(t, result.Statistics)
			assert.Equal(t, yamlPath, result.Statistics["input_file"])
			assert.Equal(t, 2, result.Statistics["clips_count"])

			// Check clip details
			clipDetails, ok := result.Statistics["clips_details"].([]map[string]interface{})
			assert.True(t, ok)
			assert.Len(t, clipDetails, 2)

			// Check font settings in statistics
			fontSettings, ok := result.Statistics["font_settings"].(map[string]interface{})
			assert.True(t, ok)
			assert.NotNil(t, fontSettings["size"])
			assert.NotNil(t, fontSettings["color"])
			assert.NotNil(t, fontSettings["box_color"])
			assert.NotNil(t, fontSettings["border_w"])

			// Verify output files exist
			for _, expectedOutput := range tt.expectedOutputs {
				_, err := os.Stat(expectedOutput)
				assert.NoError(t, err, "Output file should exist: %s", expectedOutput)
			}
		})
	}
}

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "set_title_to_short_video", module.Name())
}

func TestConvertToHHMMSS(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		expected  string
	}{
		{
			name:      "standard format",
			timestamp: "00:01:30",
			expected:  "000130",
		},
		{
			name:      "with milliseconds",
			timestamp: "00:01:30.500",
			expected:  "000130",
		},
		{
			name:      "only numbers",
			timestamp: "013000",
			expected:  "013000",
		},
		{
			name:      "short format",
			timestamp: "1:30",
			expected:  "000130",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToHHMMSS(tt.timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}
