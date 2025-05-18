package addtext

import (
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

// Module implements text overlay functionality
type Module struct{}

// Params contains the parameters for text overlay
type Params struct {
	Input         string `json:"input"`         // Path to input file or directory
	Output        string `json:"output"`        // Path to output directory
	VideoFile     string `json:"videoFile"`     // Path to the source video file
	Text          string `json:"text"`          // Text to overlay
	FontFile      string `json:"fontFile"`      // Path to the font file
	FontSize      int    `json:"fontSize"`      // Font size
	FontColor     string `json:"fontColor"`     // Font color
	Position      string `json:"position"`      // Text position (top, bottom, center)
	InputFileName string `json:"inputFileName"` // Specific input file name to process
	BoxColor      string `json:"boxColor"`      // Box color (default: "black@0.5")
	BoxBorderW    int    `json:"boxBorderW"`    // Box border width (default: 5)
	QuietFlag     bool   `json:"quietFlag"`     // Suppress ffmpeg output (default: true)
	TextX         string `json:"textX"`         // X position of text (default: "(w-text_w)/2")
	TextY         string `json:"textY"`         // Y position of text (default: "(h-text_h)/2")
}

// DefaultFontPath is the path to the default font file
const DefaultFontPath = "/System/Library/Fonts/Supplemental/Arial.ttf"

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
	ShortTitle  string `yaml:"short_title"`
}

// New creates a new addtext module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "addtext"
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

	// Check if we have a shorts suggestions file
	if strings.HasSuffix(p.Input, "shorts_suggestions.yaml") {
		// If we have a shorts suggestions file, we don't need a video file
		// The video files will be created by extractshorts
		return nil
	}

	// If we don't have a shorts suggestions file, we need a video file
	if p.VideoFile == "" {
		return &utils.ValidationError{
			Field:   "video",
			Message: "video file path is required when not using shorts suggestions",
		}
	}

	// Validate video file
	if err := utils.ValidateVideoFile(p.VideoFile); err != nil {
		return err
	}

	// Validate font file if specified
	if p.FontFile != "" && p.FontFile != DefaultFontPath {
		if _, err := os.Stat(p.FontFile); os.IsNotExist(err) {
			return fmt.Errorf("font file does not exist: %s", p.FontFile)
		}
	}

	return nil
}

// Execute adds text overlays to short video clips
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.FontSize == 0 {
		p.FontSize = 24
	}
	if p.FontColor == "" {
		p.FontColor = "white"
	}
	if p.BoxColor == "" {
		p.BoxColor = "black@0.5"
	}
	if p.BoxBorderW == 0 {
		p.BoxBorderW = 5
	}
	if p.TextX == "" {
		p.TextX = "(w-text_w)/2"
	}
	if p.TextY == "" {
		p.TextY = "(h-text_h)/2"
	}
	if p.FontFile == "" {
		p.FontFile = DefaultFontPath
	}

	// Default to quiet mode (no ffmpeg output) unless explicitly set to false
	if _, exists := params["quietFlag"]; !exists {
		p.QuietFlag = true
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Read and parse the shorts_suggestions.yaml file
	shortsData, err := readShortsFile(resolvedInput)
	if err != nil {
		return fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// Process each short clip
	for i, short := range shortsData.Shorts {
		if err := m.processShortClip(ctx, short, p); err != nil {
			return fmt.Errorf("failed to process short clip %d: %w", i+1, err)
		}
	}

	utils.LogSuccess("Successfully processed %d short clips", len(shortsData.Shorts))
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

// processShortClip adds text overlay to a single short clip
func (m *Module) processShortClip(ctx context.Context, short ShortClip, p Params) error {
	// Convert startTime and endTime to HHMMSS format for filename
	startTimeHHMMSS := convertToHHMMSS(short.StartTime)
	endTimeHHMMSS := convertToHHMMSS(short.EndTime)

	// Create input and output filenames with .mp4 extension
	inputFilename := fmt.Sprintf("%s-%s.mp4", startTimeHHMMSS, endTimeHHMMSS)
	inputPath := filepath.Join(p.Output, inputFilename)
	outputFilename := fmt.Sprintf("%s-%s-withtext.mp4", startTimeHHMMSS, endTimeHHMMSS)
	outputPath := filepath.Join(p.Output, outputFilename)

	// Build FFmpeg command for text overlay
	args := []string{
		"-i", inputPath,
	}

	// Add font file if specified
	fontFileArg := ""
	if p.FontFile != "" {
		fontFileArg = fmt.Sprintf("fontfile=%s:", p.FontFile)
	}

	// Escape special characters in the short_title text
	escapedText := strings.ReplaceAll(short.ShortTitle, "'", "\\'")
	escapedText = strings.ReplaceAll(escapedText, ":", "\\:")
	escapedText = strings.ReplaceAll(escapedText, "\\", "\\\\")

	// Build the drawtext filter
	drawtextFilter := fmt.Sprintf(
		"drawtext=%stext='%s':fontcolor=%s:fontsize=%d:box=1:boxcolor=%s:boxborderw=%d:x=%s:y=%s:line_spacing=10",
		fontFileArg,
		escapedText,
		p.FontColor,
		p.FontSize,
		p.BoxColor,
		p.BoxBorderW,
		p.TextX,
		p.TextY,
	)

	// Add the filter to the command
	args = append(args, "-vf", drawtextFilter)

	// Add quiet flags if enabled
	if p.QuietFlag {
		args = append(args, "-v", "error", "-stats")
	}

	// Add output file with video codec settings
	args = append(args, "-c:v", "libx264", "-c:a", "aac", "-b:a", "128k", "-b:v", "2500k", outputPath)

	// Prepare the command
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Configure output handling based on quiet mode
	var stderr strings.Builder
	if p.QuietFlag {
		cmd.Stdout = nil
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	utils.LogInfo("Adding text overlay to clip: %s", short.Title)

	// Run the FFmpeg command
	if err := cmd.Run(); err != nil {
		if p.QuietFlag && stderr.Len() > 0 {
			// Log the error output if we captured it
			utils.LogError("FFmpeg error: %s", stderr.String())
		}
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Added text overlay to: %s", outputFilename)
	return nil
}

// convertToHHMMSS converts a timestamp like "00:01:23" to "000123"
func convertToHHMMSS(timestamp string) string {
	// Remove colons
	return strings.ReplaceAll(timestamp, ":", "")
}
