package split

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/stretchr/testify/assert"
)

// mockCmd is used to simulate command execution during tests
type mockCmd struct {
	cmd  string
	args []string
}

var (
	// Store executed commands for verification
	executedCmds []mockCmd
)

func init() {
	// Save the original exec.Command
	execCommand = exec.Command
	// Save the original exec.LookPath
	utils.ExecLookPath = exec.LookPath
}

// fakeExecCommand creates a fake exec.Command that records its args
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	executedCmds = append(executedCmds, mockCmd{cmd: command, args: args})
	return exec.Command("echo", "test") // Use echo as a harmless command
}

// fakeLookPath always returns success
func fakeLookPath(file string) (string, error) {
	return file, nil
}

func TestMain(m *testing.M) {
	// Setup
	origExecCommand := execCommand
	origExecLookPath := utils.ExecLookPath
	defer func() {
		execCommand = origExecCommand
		utils.ExecLookPath = origExecLookPath
	}()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func TestSplitModule(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "split_test")
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

	// Create a single test audio file for single file tests
	singleTestDir := filepath.Join(tempDir, "single")
	if err := os.MkdirAll(singleTestDir, 0755); err != nil {
		t.Fatal(err)
	}
	testAudioPath := filepath.Join(singleTestDir, "test.wav")
	if err := os.WriteFile(testAudioPath, []byte("test audio content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create multiple audio files for directory testing
	testFiles := []struct {
		name    string
		content string
	}{
		{"audio1.wav", "test audio 1"},
		{"audio2.wav", "test audio 2"},
		{"ignored.mp3", "should be ignored"},
		{"audio3.wav", "test audio 3"},
	}

	for _, tf := range testFiles {
		if err := os.WriteFile(filepath.Join(inputDir, tf.name), []byte(tf.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a subdirectory (should be ignored)
	if err := os.MkdirAll(filepath.Join(inputDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		params       map[string]interface{}
		mockSetup    func()
		wantErr      bool
		wantOutput   string
		expectedCmds int // Number of expected ffmpeg commands
		setupFiles   func() error
		cleanupFiles func() error
	}{
		{
			name: "successful split single file",
			params: map[string]interface{}{
				"input":       testAudioPath,
				"output":      outputDir,
				"segmentTime": 1800,
				"filePattern": "splited%03d",
				"audioFormat": "wav",
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			wantErr:      false,
			wantOutput:   outputDir,
			expectedCmds: 1,
		},
		{
			name: "successful split directory",
			params: map[string]interface{}{
				"input":       inputDir,
				"output":      outputDir,
				"segmentTime": 1800,
				"filePattern": "splited%03d",
				"audioFormat": "wav",
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			wantErr:      false,
			wantOutput:   outputDir,
			expectedCmds: 3, // Should process only the 3 .wav files in inputDir
		},
		{
			name: "directory with no matching files",
			params: map[string]interface{}{
				"input":       inputDir,
				"output":      outputDir,
				"segmentTime": 1800,
				"filePattern": "splited%03d",
				"audioFormat": "m4a", // No matching m4a files in test directory
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			wantErr:      false,
			wantOutput:   outputDir,
			expectedCmds: 0,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":       "/nonexistent/path",
				"output":      outputDir,
				"segmentTime": 1800,
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			wantErr: true,
		},
		{
			name: "missing required parameters",
			params: map[string]interface{}{
				"output": outputDir,
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			wantErr: true,
		},
		{
			name: "unreadable directory",
			params: map[string]interface{}{
				"input":       filepath.Join(tempDir, "unreadable"),
				"output":      outputDir,
				"segmentTime": 1800,
				"audioFormat": "wav",
			},
			mockSetup: func() {
				execCommand = fakeExecCommand
				executedCmds = nil
			},
			setupFiles: func() error {
				unreadableDir := filepath.Join(tempDir, "unreadable")
				if err := os.MkdirAll(unreadableDir, 0755); err != nil {
					return err
				}
				return os.Chmod(unreadableDir, 0000)
			},
			cleanupFiles: func() error {
				unreadableDir := filepath.Join(tempDir, "unreadable")
				if err := os.Chmod(unreadableDir, 0755); err != nil {
					return err
				}
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files if needed
			if tt.setupFiles != nil {
				err := tt.setupFiles()
				if err != nil {
					t.Fatal(err)
				}
			}

			// Cleanup after test
			defer func() {
				if tt.cleanupFiles != nil {
					if err := tt.cleanupFiles(); err != nil {
						t.Errorf("failed to cleanup test files: %v", err)
					}
				}
			}()

			// Setup mock
			tt.mockSetup()

			// Create module instance
			module := New()

			// Execute module
			result, err := module.Execute(context.Background(), tt.params)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantOutput, result.Outputs["segments"])

			// Verify the number of ffmpeg commands executed
			assert.Len(t, executedCmds, tt.expectedCmds, "unexpected number of ffmpeg commands executed")

			// For successful cases with commands, verify the ffmpeg command was called correctly
			if !tt.wantErr && tt.expectedCmds > 0 {
				for _, cmd := range executedCmds {
					assert.Equal(t, "ffmpeg", cmd.cmd)
					assert.Contains(t, cmd.args, "-segment_time")
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// Replace exec.Command with our mock
	execCommand = fakeExecCommand
	utils.ExecLookPath = fakeLookPath
	defer func() {
		execCommand = exec.Command
		utils.ExecLookPath = exec.LookPath
	}()

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "split_validate_test")
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

	// Create a test audio file
	testAudioPath := filepath.Join(inputDir, "test.wav")
	if err := os.WriteFile(testAudioPath, []byte("test audio content"), 0644); err != nil {
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
				"input":       testAudioPath,
				"output":      outputDir,
				"segmentTime": 1800,
			},
			wantErr: false,
		},
		{
			name: "missing input",
			params: map[string]interface{}{
				"output":      outputDir,
				"segmentTime": 1800,
			},
			wantErr: true,
		},
		{
			name: "missing output",
			params: map[string]interface{}{
				"input":       testAudioPath,
				"segmentTime": 1800,
			},
			wantErr: true,
		},
		{
			name: "invalid input path",
			params: map[string]interface{}{
				"input":       "/nonexistent/path",
				"output":      outputDir,
				"segmentTime": 1800,
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
	assert.Len(t, io.RequiredInputs, 1)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Contains(t, io.RequiredInputs[0].Patterns, "*.wav")

	// Test optional inputs
	assert.Len(t, io.OptionalInputs, 3)
	assert.Equal(t, "segmentTime", io.OptionalInputs[0].Name)
	assert.Equal(t, "filePattern", io.OptionalInputs[1].Name)
	assert.Equal(t, "audioFormat", io.OptionalInputs[2].Name)

	// Test produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "segments", io.ProducedOutputs[0].Name)
	assert.Contains(t, io.ProducedOutputs[0].Patterns, "splited*.wav")
}
