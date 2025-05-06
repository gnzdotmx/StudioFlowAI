package extractshorts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	OutputFormat  string `json:"outputFormat"`  // Output format (default: "mp4")
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

	if p.Input == "" {
		return fmt.Errorf("input path is required")
	}

	if p.Output == "" {
		return fmt.Errorf("output path is required")
	}

	if p.VideoFile == "" {
		return fmt.Errorf("videoFile is required - specify the source video file")
	}

	// Check if FFmpeg is installed
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	return nil
}

// Execute extracts short video clips based on the shorts_suggestions.yaml file
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.OutputFormat == "" {
		p.OutputFormat = "mp4"
	}

	// Default to quiet mode (no ffmpeg output) unless explicitly set to false
	if _, exists := params["quietFlag"]; !exists {
		p.QuietFlag = true
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Handle input path resolution
	inputPath, err := getInputFilePath(p.Input, p.InputFileName)
	if err != nil {
		return err
	}

	// Read and parse the shorts_suggestions.yaml file
	shortsData, err := readShortsFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// Extract each short clip
	for i, short := range shortsData.Shorts {
		if err := m.extractShortClip(ctx, short, p, i+1); err != nil {
			return fmt.Errorf("failed to extract short clip %d: %w", i+1, err)
		}
	}

	utils.LogSuccess("Successfully extracted %d short clips", len(shortsData.Shorts))
	return nil
}

// readShortsFile reads and parses the shorts_suggestions.yaml file
func readShortsFile(filePath string) (*ShortsData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var shortsData ShortsData
	if err := yaml.Unmarshal(data, &shortsData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &shortsData, nil
}

// extractShortClip extracts a single short clip using FFmpeg
func (m *Module) extractShortClip(ctx context.Context, short ShortClip, p Params, index int) error {
	// Sanitize the title for use in filename
	safeTitle := sanitizeFilename(short.Title)
	if safeTitle == "" {
		safeTitle = fmt.Sprintf("clip%d", index)
	}

	// Convert startTime to HHMMSS format for filename
	startTimeHHMMSS := convertToHHMMSS(short.StartTime)

	// Create output filename: HHMMSS-Title.mp4
	outputFilename := fmt.Sprintf("%s-%s.%s", startTimeHHMMSS, safeTitle, p.OutputFormat)
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

// getInputFilePath resolves the input file path
func getInputFilePath(inputPath, inputFileName string) (string, error) {
	// Check if input is a file or directory
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("input path does not exist: %w", err)
	}

	// If input is a file, return it directly
	if !fileInfo.IsDir() {
		return inputPath, nil
	}

	// If input is a directory and a specific filename is provided
	if inputFileName != "" {
		return filepath.Join(inputPath, inputFileName), nil
	}

	// If input is a directory, look for shorts_suggestions.yaml
	defaultFile := filepath.Join(inputPath, "shorts_suggestions.yaml")
	if _, err := os.Stat(defaultFile); err == nil {
		return defaultFile, nil
	}

	return "", fmt.Errorf("no shorts_suggestions.yaml file found in %s", inputPath)
}

// sanitizeFilename removes or replaces characters that are not safe for filenames
func sanitizeFilename(filename string) string {
	// Replace spaces and special characters with hyphens
	re := regexp.MustCompile(`[^\w\s-]`)
	safe := re.ReplaceAllString(filename, "-")

	// Replace multiple spaces with a single hyphen
	re = regexp.MustCompile(`[\s]+`)
	safe = re.ReplaceAllString(safe, "-")

	// Trim hyphens from beginning and end
	return strings.Trim(safe, "-")
}

// convertToHHMMSS converts a timestamp like "00:01:23" to "000123"
func convertToHHMMSS(timestamp string) string {
	// Remove colons
	return strings.ReplaceAll(timestamp, ":", "")
}
