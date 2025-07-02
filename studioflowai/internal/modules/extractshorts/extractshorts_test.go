package extractshorts

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Save the original exec.Command
	execCommand = exec.CommandContext
	// Save the original exec.LookPath
	utils.ExecLookPath = exec.LookPath
}

// TestMain sets up and tears down the mock command
func TestMain(m *testing.M) {
	// Run the tests
	result := m.Run()

	// Restore the original exec.Command
	execCommand = exec.CommandContext
	// Restore the original exec.LookPath
	utils.ExecLookPath = exec.LookPath

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

// fakeLookPath always returns success
func fakeLookPath(file string) (string, error) {
	return file, nil
}

// TestHelperProcess is not a real test, it's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Test required inputs
	assert.Len(t, io.RequiredInputs, 3)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)
	assert.Equal(t, "videoFile", io.RequiredInputs[2].Name)

	// Test optional inputs
	assert.Len(t, io.OptionalInputs, 2)
	assert.Equal(t, "ffmpegParams", io.OptionalInputs[0].Name)
	assert.Equal(t, "quietFlag", io.OptionalInputs[1].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "clips", io.ProducedOutputs[0].Name)
}

func TestModule_Validate(t *testing.T) {
	// Replace exec.Command with our mock
	execCommand = fakeExecCommand
	utils.ExecLookPath = fakeLookPath
	defer func() {
		execCommand = exec.CommandContext
		utils.ExecLookPath = exec.LookPath
	}()

	module := New()
	tempDir := t.TempDir()

	// Create test files
	videoPath := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(videoPath, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	yamlContent := []byte(`
sourceVideo: test.mp4
shorts:
  - title: "First Clip"
    startTime: "00:00:10"
    endTime: "00:00:20"
    description: "Test clip 1"
    tags: "#test #clip1"
`)

	yamlPath := filepath.Join(tempDir, "shorts_suggestions.yaml")
	err = os.WriteFile(yamlPath, yamlContent, 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
		setup   func(t *testing.T, tempDir string)
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"input":     yamlPath,
				"output":    tempDir,
				"videoFile": videoPath,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output":    tempDir,
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
			name: "missing video file",
			params: map[string]interface{}{
				"input":  yamlPath,
				"output": tempDir,
			},
			wantErr: true,
		},
		{
			name: "invalid yaml file",
			params: map[string]interface{}{
				"input":     filepath.Join(tempDir, "invalid.yaml"),
				"output":    tempDir,
				"videoFile": videoPath,
			},
			wantErr: true,
			setup: func(t *testing.T, tempDir string) {
				invalidPath := filepath.Join(tempDir, "invalid.yaml")
				err := os.WriteFile(invalidPath, []byte("invalid yaml content"), 0644)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t, tempDir)
			}
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
		execCommand = exec.CommandContext
	}()

	module := New()
	tempDir := t.TempDir()

	// Create test video file
	videoPath := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(videoPath, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Create test YAML files
	yamlContent := []byte(`
sourceVideo: test.mp4
shorts:
  - title: "First Clip"
    startTime: "00:00:10"
    endTime: "00:00:20"
    description: "Test clip 1"
    tags: "#test #clip1"
  - title: "Second Clip"
    startTime: "00:01:00"
    endTime: "00:01:30"
    description: "Test clip 2"
    tags: "#test #clip2"
`)

	yamlPath := filepath.Join(tempDir, "shorts_suggestions.yaml")
	err = os.WriteFile(yamlPath, yamlContent, 0644)
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
				"quietFlag": true,
			},
			expectedOutputs: []string{
				filepath.Join(tempDir, "000010-000020.mp4"),
				filepath.Join(tempDir, "000100-000130.mp4"),
			},
			wantErr: false,
		},
		{
			name: "process shorts with custom ffmpeg params",
			params: map[string]interface{}{
				"input":        yamlPath,
				"output":       tempDir,
				"videoFile":    videoPath,
				"ffmpegParams": "-c:v libx264 -preset fast",
				"quietFlag":    true,
			},
			expectedOutputs: []string{
				filepath.Join(tempDir, "000010-000020.mp4"),
				filepath.Join(tempDir, "000100-000130.mp4"),
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
			assert.Equal(t, videoPath, result.Statistics["source_video"])
			assert.Equal(t, 2, result.Statistics["clips_count"])

			// Check clip details
			clipDetails, ok := result.Statistics["clips_details"].([]map[string]interface{})
			assert.True(t, ok)
			assert.Len(t, clipDetails, 2)
		})
	}
}

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "extract_shorts", module.Name())
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
