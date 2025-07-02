package split

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// execCommand allows us to mock exec.Command in tests
var execCommand = exec.Command

// Module implements audio splitting functionality
type Module struct{}

// Params contains the parameters for audio splitting
type Params struct {
	Input       string `json:"input"`       // Path to input audio file or directory
	Output      string `json:"output"`      // Path to output directory
	SegmentTime int    `json:"segmentTime"` // Segment duration in seconds (default: 1800 = 30 minutes)
	FilePattern string `json:"filePattern"` // Output file pattern (default: "splited%03d")
	AudioFormat string `json:"audioFormat"` // Output audio format (default: "wav")
}

// New creates a new split module
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "split"
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Input audio file or directory",
				Patterns:    []string{"*.wav", "*.mp3", "*.m4a", "*.aac"},
				Type:        string(modules.InputTypeFile),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "segments",
				Description: "Split audio segments",
				Patterns:    []string{"splited*.wav"},
				Type:        string(modules.OutputTypeFile),
			},
		},
		OptionalInputs: []modules.ModuleInput{
			{
				Name:        "segmentTime",
				Description: "Segment duration in seconds (default: 1800 = 30 minutes)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "filePattern",
				Description: "Output file pattern (default: 'splited%03d')",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "audioFormat",
				Description: "Output audio format (default: 'wav')",
				Type:        string(modules.InputTypeData),
			},
		},
	}
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

	// Validate FFmpeg dependency
	if err := utils.ValidateRequiredDependency("ffmpeg"); err != nil {
		return err
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Validate audio file extension if input is a file
	fileInfo, err := os.Stat(resolvedInput)
	if err == nil && !fileInfo.IsDir() {
		if err := utils.ValidateFileExtension(resolvedInput, []string{".wav", ".mp3", ".m4a", ".aac"}); err != nil {
			return err
		}
	}

	return nil
}

// Execute splits audio files into smaller segments
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Set default values
	if p.SegmentTime == 0 {
		p.SegmentTime = 1800 // 30 minutes default
	}
	if p.FilePattern == "" {
		p.FilePattern = "splited%03d"
	}
	if p.AudioFormat == "" {
		p.AudioFormat = "wav"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to access input: %w", err)
	}

	if fileInfo.IsDir() {
		// Process all audio files in the directory
		if err := m.processDirectory(p); err != nil {
			return modules.ModuleResult{}, err
		}
	} else {
		// Process a single file
		if err := m.processFile(resolvedInput, p); err != nil {
			return modules.ModuleResult{}, err
		}
	}

	// Create result with output information
	result := modules.ModuleResult{
		Outputs: map[string]string{
			"segments": p.Output,
		},
		Metadata: map[string]interface{}{
			"segmentTime": p.SegmentTime,
			"audioFormat": p.AudioFormat,
		},
		Statistics: map[string]interface{}{
			"inputFile": resolvedInput,
		},
	}

	return result, nil
}

// processDirectory processes all audio files in a directory
func (m *Module) processDirectory(p Params) error {
	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	entries, err := os.ReadDir(resolvedInput)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Define supported input formats
	supportedFormats := map[string]bool{
		".wav": true,
		".mp3": true,
		".m4a": true,
		".aac": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := strings.ToLower(filepath.Ext(filename))

		// Skip files that don't match the requested output format
		if ext != "."+strings.ToLower(p.AudioFormat) {
			continue
		}

		// Skip unsupported input formats
		if !supportedFormats[ext] {
			continue
		}

		inputPath := filepath.Join(resolvedInput, filename)
		if err := m.processFile(inputPath, p); err != nil {
			return err
		}
	}

	return nil
}

// processFile splits a single audio file into segments
func (m *Module) processFile(filePath string, p Params) error {
	outputPattern := filepath.Join(p.Output, p.FilePattern+"."+p.AudioFormat)

	utils.LogVerbose("Splitting %s into segments of %d seconds", filePath, p.SegmentTime)

	// Split audio with ffmpeg using the mockable execCommand
	cmd := execCommand(
		"ffmpeg",
		"-i", filePath,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", p.SegmentTime),
		"-c", "copy",
		"-loglevel", "error",
		outputPattern,
	)

	// Redirect stdout and stderr to suppress output
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Successfully split %s into segments", filePath)
	return nil
}
