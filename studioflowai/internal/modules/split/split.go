package split

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

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
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "split"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	if p.Input == "" {
		return fmt.Errorf("input path is required")
	}

	if p.Output == "" {
		return fmt.Errorf("output path is required")
	}

	// Ensure the input file or directory exists
	if _, err := os.Stat(p.Input); os.IsNotExist(err) {
		return fmt.Errorf("input path %s does not exist", p.Input)
	}

	return nil
}

// Execute splits audio files into smaller segments
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
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
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(p.Input)
	if err != nil {
		return fmt.Errorf("failed to access input: %w", err)
	}

	if fileInfo.IsDir() {
		// Process all audio files in the directory
		return m.processDirectory(ctx, p)
	}

	// Process a single file
	return m.processFile(ctx, p.Input, p)
}

// processDirectory processes all audio files in a directory
func (m *Module) processDirectory(ctx context.Context, p Params) error {
	entries, err := os.ReadDir(p.Input)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := filepath.Ext(filename)
		if ext != "."+p.AudioFormat {
			continue
		}

		inputPath := filepath.Join(p.Input, filename)
		if err := m.processFile(ctx, inputPath, p); err != nil {
			return err
		}
	}

	return nil
}

// processFile splits a single audio file into segments
func (m *Module) processFile(ctx context.Context, filePath string, p Params) error {
	outputPattern := filepath.Join(p.Output, p.FilePattern+"."+p.AudioFormat)

	utils.LogVerbose("Splitting %s into segments of %d seconds", filePath, p.SegmentTime)

	// Split audio with ffmpeg
	cmd := exec.CommandContext(
		ctx,
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
