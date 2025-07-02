package extractaudio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// execCommand allows us to mock exec.Command in tests
var execCommand = exec.Command

// Module implements the audio extraction functionality
type Module struct{}

// Params contains the parameters for audio extraction
type Params struct {
	Input      string `json:"input"`      // Path to input video file or directory
	Output     string `json:"output"`     // Path to output directory
	OutputName string `json:"outputName"` // Custom output filename (optional)
	SampleRate int    `json:"sampleRate"` // Sample rate in Hz (default: 16000)
	Channels   int    `json:"channels"`   // Number of audio channels (default: 1)
}

// New creates a new extract module
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "extractaudio"
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

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Validate video file extension if input is a file
	fileInfo, err := os.Stat(resolvedInput)
	if err == nil && !fileInfo.IsDir() {
		if err := utils.ValidateFileExtension(resolvedInput, []string{".mp4", ".mov"}); err != nil {
			return err
		}
	}

	// Validate output file extension if outputName is provided
	if p.OutputName != "" {
		if err := utils.ValidateFileExtension(p.OutputName, []string{".wav", ".mp3", ".m4a", ".aac"}); err != nil {
			return err
		}
	}

	// Validate FFmpeg dependency
	if err := utils.ValidateRequiredDependency("ffmpeg"); err != nil {
		return err
	}

	return nil
}

// Execute extracts audio from a video file or processes multiple files in a directory
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Set default values
	if p.SampleRate == 0 {
		p.SampleRate = 16000
	}
	if p.Channels == 0 {
		p.Channels = 1
	}

	// Ensure we have a valid output directory
	if p.Output == "" {
		return modules.ModuleResult{}, fmt.Errorf("output directory path is required")
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to access input: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	if fileInfo.IsDir() {
		// Process all video files in the directory
		return m.processDirectory(p)
	}

	// Process a single file
	return m.processFile(resolvedInput, p)
}

// processDirectory processes all video files in a directory
func (m *Module) processDirectory(p Params) (modules.ModuleResult, error) {
	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	entries, err := os.ReadDir(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := filepath.Ext(filename)
		if ext != ".mp4" && ext != ".mov" {
			continue
		}

		inputPath := filepath.Join(resolvedInput, filename)
		result, err := m.processFile(inputPath, p)
		if err != nil {
			return modules.ModuleResult{}, err
		}
		return result, nil
	}

	return modules.ModuleResult{}, nil
}

// processFile extracts audio from a single video file
func (m *Module) processFile(filePath string, p Params) (modules.ModuleResult, error) {
	var audioPath string

	if p.OutputName != "" {
		// Use the custom output name if provided
		audioPath = filepath.Join(p.Output, p.OutputName)
	} else {
		// Otherwise use the input filename as base
		filename := filepath.Base(filePath)
		baseName := filename[:len(filename)-len(filepath.Ext(filename))]
		audioPath = filepath.Join(p.Output, baseName)
	}

	utils.LogVerbose("Extracting audio from %s to %s", filePath, audioPath)

	// Extract audio with ffmpeg
	cmd := execCommand(
		"ffmpeg",
		"-i", filePath,
		"-vn",
		"-ar", fmt.Sprintf("%d", p.SampleRate),
		"-ac", fmt.Sprintf("%d", p.Channels),
		"-c:a", "pcm_s16le",
		audioPath,
		"-y",
		"-loglevel", "error",
	)

	// Redirect stdout and stderr to suppress output
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Successfully extracted audio to %s", audioPath)
	return modules.ModuleResult{
		Outputs: map[string]string{
			"audio": audioPath,
		},
	}, nil
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to input video file or directory",
				Patterns:    []string{".mp4", ".mov"},
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
				Name:        "outputName",
				Description: "Custom output filename",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "sampleRate",
				Description: "Sample rate in Hz (default: 16000)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "channels",
				Description: "Number of audio channels (default: 1)",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "audio",
				Description: "Extracted audio file",
				Patterns:    []string{".wav", ".mp3", ".m4a", ".aac"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
}
