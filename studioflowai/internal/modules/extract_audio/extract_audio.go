package extract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

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
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "extract"
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
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.SampleRate == 0 {
		p.SampleRate = 16000
	}
	if p.Channels == 0 {
		p.Channels = 1
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return fmt.Errorf("failed to access input: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if fileInfo.IsDir() {
		// Process all video files in the directory
		return m.processDirectory(ctx, p)
	}

	// Process a single file
	return m.processFile(ctx, resolvedInput, p)
}

// processDirectory processes all video files in a directory
func (m *Module) processDirectory(ctx context.Context, p Params) error {
	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	entries, err := os.ReadDir(resolvedInput)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
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
		if err := m.processFile(ctx, inputPath, p); err != nil {
			return err
		}
	}

	return nil
}

// processFile extracts audio from a single video file
func (m *Module) processFile(ctx context.Context, filePath string, p Params) error {
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
	cmd := exec.CommandContext(
		ctx,
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
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Successfully extracted audio to %s", audioPath)
	return nil
}
