package extractshorts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"gopkg.in/yaml.v3"
)

// Module implements short video extraction functionality
type Module struct{}

// Params contains the parameters for short video extraction
type Params struct {
	Input         string `json:"input"`         // Path to shorts_suggestions.yaml file
	Output        string `json:"output"`        // Path to output directory
	VideoFile     string `json:"videoFile"`     // Path to the source video file
	FFmpegParams  string `json:"ffmpegParams"`  // Additional parameters for FFmpeg
	InputFileName string `json:"inputFileName"` // Specific input file name to process
	QuietFlag     bool   `json:"quietFlag"`     // Suppress ffmpeg output (default: true)
}

// ShortsData represents the structure of the shorts_suggestions.yaml file
type ShortsData struct {
	SourceVideo string      `yaml:"sourceVideo"`
	Shorts      []ShortClip `yaml:"shorts"`
}

// ShortClip represents a single short video clip suggestion
type ShortClip struct {
	Title       string `yaml:"title"`
	StartTime   string `yaml:"startTime"`
	EndTime     string `yaml:"endTime"`
	Description string `yaml:"description"`
	Tags        string `yaml:"tags"`
}

// New creates a new extract shorts module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "extractshorts"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, p.InputFileName); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Validate video file
	if err := utils.ValidateVideoFile(p.VideoFile); err != nil {
		return err
	}

	// Validate FFmpeg dependency
	if err := utils.ValidateRequiredDependency("ffmpeg"); err != nil {
		return err
	}

	return nil
}

// Execute extracts short video clips based on suggestions
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// If input is a directory and we have a specific input filename, join them
	if fileInfo, err := os.Stat(resolvedInput); err == nil && fileInfo.IsDir() && p.InputFileName != "" {
		resolvedInput = filepath.Join(resolvedInput, p.InputFileName)
	}

	// Read and parse the shorts suggestions YAML file
	shortsData, err := m.readShortsFile(resolvedInput)
	if err != nil {
		return err
	}

	// Process each short clip
	for _, short := range shortsData.Shorts {
		if err := m.extractShortClip(ctx, short, p); err != nil {
			return err
		}
	}

	return nil
}

// readShortsFile reads and parses the shorts suggestions YAML file
func (m *Module) readShortsFile(inputPath string) (*ShortsData, error) {
	// Ensure we're reading a file, not a directory
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shorts file: %w", err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("input path is a directory, expected a file: %s", inputPath)
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shorts file: %w", err)
	}

	var shortsData ShortsData
	if err := yaml.Unmarshal(data, &shortsData); err != nil {
		return nil, fmt.Errorf("failed to parse shorts file: %w", err)
	}

	return &shortsData, nil
}

// extractShortClip extracts a single short video clip
func (m *Module) extractShortClip(ctx context.Context, short ShortClip, p Params) error {
	// Convert startTime and endTime to HHMMSS format for filename
	startTimeHHMMSS := convertToHHMMSS(short.StartTime)
	endTimeHHMMSS := convertToHHMMSS(short.EndTime)

	// Create output filename: HHMMSS-HHMMSS.mp4
	outputFilename := fmt.Sprintf("%s-%s.mp4", startTimeHHMMSS, endTimeHHMMSS)
	outputPath := filepath.Join(p.Output, outputFilename)

	// Build FFmpeg command
	args := []string{
		"-ss", short.StartTime,
		"-to", short.EndTime,
	}

	// Add quiet flags if enabled (default behavior)
	if p.QuietFlag {
		args = append(args, "-v", "error", "-stats")
	}

	args = append(args, "-i", p.VideoFile, "-c", "copy") // Copy without re-encoding for speed

	// Add any additional FFmpeg parameters
	if p.FFmpegParams != "" {
		args = append(args, strings.Fields(p.FFmpegParams)...)
	} else {
		// Default video codec settings if no custom parameters provided
		args = append(args, "-c:v", "libx264", "-c:a", "aac", "-b:a", "128k", "-b:v", "2500k")
	}

	// Add output file
	args = append(args, outputPath)

	// Prepare the command
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Configure output handling based on quiet mode
	var stderr bytes.Buffer
	if p.QuietFlag {
		cmd.Stdout = nil
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	utils.LogInfo("Extracting clip: %s (%s to %s)", short.Title, short.StartTime, short.EndTime)

	// Run the FFmpeg command
	if err := cmd.Run(); err != nil {
		if p.QuietFlag && stderr.Len() > 0 {
			// Log the error output if we captured it
			utils.LogError("FFmpeg error: %s", stderr.String())
		}
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Extracted: %s", outputFilename)
	return nil
}

// convertToHHMMSS converts a timestamp to HHMMSS format
func convertToHHMMSS(timestamp string) string {
	// Remove any non-numeric characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, timestamp)

	// Ensure we have at least 6 digits
	if len(digits) < 6 {
		digits = fmt.Sprintf("%06s", digits)
	}

	// Take the first 6 digits
	return digits[:6]
}
