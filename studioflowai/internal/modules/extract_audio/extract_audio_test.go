package extractaudio

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
	execCommand = exec.Command
	// Save the original exec.LookPath
	utils.ExecLookPath = exec.LookPath
}

// TestMain sets up and tears down the mock command
func TestMain(m *testing.M) {
	// Run the tests
	result := m.Run()

	// Restore the original exec.Command
	execCommand = exec.Command
	// Restore the original exec.LookPath
	utils.ExecLookPath = exec.LookPath

	os.Exit(result)
}

// fakeExecCommand creates a mock command that does nothing
func fakeExecCommand(command string, args ...string) *exec.Cmd {
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
	assert.Len(t, io.RequiredInputs, 2)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "output", io.RequiredInputs[1].Name)

	// Test optional inputs
	assert.Len(t, io.OptionalInputs, 3)
	assert.Equal(t, "outputName", io.OptionalInputs[0].Name)
	assert.Equal(t, "sampleRate", io.OptionalInputs[1].Name)
	assert.Equal(t, "channels", io.OptionalInputs[2].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "audio", io.ProducedOutputs[0].Name)
}

func TestModule_Validate(t *testing.T) {
	// Replace exec.Command with our mock
	execCommand = fakeExecCommand
	utils.ExecLookPath = fakeLookPath
	defer func() {
		execCommand = exec.Command
		utils.ExecLookPath = exec.LookPath
	}()

	module := New()
	tempDir := t.TempDir()

	// Create a test video file
	videoPath := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(videoPath, []byte("dummy video content"), 0644)
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
				"input":      videoPath,
				"output":     tempDir,
				"sampleRate": 16000,
				"channels":   1,
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
				"input": videoPath,
			},
			wantErr: true,
		},
		{
			name: "invalid input extension",
			params: map[string]interface{}{
				"input":  filepath.Join(tempDir, "test.txt"),
				"output": tempDir,
			},
			wantErr: true,
			setup: func(t *testing.T, tempDir string) {
				invalidPath := filepath.Join(tempDir, "test.txt")
				err := os.WriteFile(invalidPath, []byte("dummy text content"), 0644)
				require.NoError(t, err)
			},
		},
		{
			name: "invalid output name extension",
			params: map[string]interface{}{
				"input":      videoPath,
				"output":     tempDir,
				"outputName": "output.txt",
			},
			wantErr: true,
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
		execCommand = exec.Command
	}()

	module := New()
	tempDir := t.TempDir()

	// Create a test video file
	videoPath := filepath.Join(tempDir, "test.mp4")
	err := os.WriteFile(videoPath, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	// Create a test directory with video files
	videosDir := filepath.Join(tempDir, "videos")
	require.NoError(t, os.MkdirAll(videosDir, 0755))

	videoPath1 := filepath.Join(videosDir, "video1.mp4")
	videoPath2 := filepath.Join(videosDir, "video2.mov")
	err = os.WriteFile(videoPath1, []byte("dummy video 1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(videoPath2, []byte("dummy video 2"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		params         map[string]interface{}
		expectedOutput string
		wantErr        bool
	}{
		{
			name: "process single file",
			params: map[string]interface{}{
				"input":      videoPath,
				"output":     tempDir,
				"sampleRate": 16000,
				"channels":   1,
			},
			expectedOutput: filepath.Join(tempDir, "test"),
			wantErr:        false,
		},
		{
			name: "process directory",
			params: map[string]interface{}{
				"input":      videosDir,
				"output":     tempDir,
				"sampleRate": 16000,
				"channels":   1,
			},
			expectedOutput: filepath.Join(tempDir, "video1"),
			wantErr:        false,
		},
		{
			name: "custom output name",
			params: map[string]interface{}{
				"input":      videoPath,
				"output":     tempDir,
				"outputName": "custom.wav",
				"sampleRate": 16000,
				"channels":   1,
			},
			expectedOutput: filepath.Join(tempDir, "custom.wav"),
			wantErr:        false,
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
			assert.NotEmpty(t, result.Outputs["audio"])
			assert.Equal(t, tt.expectedOutput, result.Outputs["audio"])
		})
	}
}

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "extractaudio", module.Name())
}
