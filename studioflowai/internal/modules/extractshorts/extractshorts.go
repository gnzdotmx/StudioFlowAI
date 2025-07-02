package extractshorts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"gopkg.in/yaml.v3"
)

// execCommand allows us to mock exec.Command in tests
var execCommand = exec.CommandContext

// Module implements short video extraction functionality
type Module struct{}

// Params contains the parameters for short video extraction
type Params struct {
	Input        string `json:"input"`        // Path to shorts_suggestions.yaml file
	Output       string `json:"output"`       // Path to output directory
	VideoFile    string `json:"videoFile"`    // Path to the source video file
	FFmpegParams string `json:"ffmpegParams"` // Additional parameters for FFmpeg
	QuietFlag    bool   `json:"quietFlag"`    // Suppress ffmpeg output (default: true)
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
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "extract_shorts"
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

	// Validate video file
	if err := utils.ValidateVideoFile(p.VideoFile); err != nil {
		return err
	}

	// Validate FFmpeg dependency
	if err := utils.ValidateRequiredDependency("ffmpeg"); err != nil {
		return err
	}

	// Validate YAML file content
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)
	if _, err := m.readShortsFile(resolvedInput); err != nil {
		return fmt.Errorf("invalid shorts file: %w", err)
	}

	return nil
}

// Execute extracts short video clips based on suggestions
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Read and parse the shorts suggestions YAML file
	shortsData, err := m.readShortsFile(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, err
	}

	// Track extracted clips
	extractedClips := make(map[string]string)
	clipStats := make([]map[string]interface{}, 0)

	// Process each short clip
	for _, short := range shortsData.Shorts {
		clipPath, err := m.extractShortClip(ctx, short, p)
		if err != nil {
			return modules.ModuleResult{}, err
		}

		clipName := filepath.Base(clipPath)
		extractedClips[clipName] = clipPath
		clipStats = append(clipStats, map[string]interface{}{
			"title":       short.Title,
			"start_time":  short.StartTime,
			"end_time":    short.EndTime,
			"output_file": clipPath,
		})
	}

	return modules.ModuleResult{
		Outputs: extractedClips,
		Statistics: map[string]interface{}{
			"input_file":    resolvedInput,
			"source_video":  p.VideoFile,
			"clips_count":   len(shortsData.Shorts),
			"clips_details": clipStats,
			"ffmpeg_params": p.FFmpegParams,
			"process_time":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to shorts suggestions YAML file",
				Patterns:    []string{".yaml"},
				Type:        string(modules.InputTypeFile),
			},
			{
				Name:        "output",
				Description: "Path to output directory",
				Type:        string(modules.InputTypeDirectory),
			},
			{
				Name:        "videoFile",
				Description: "Path to source video file",
				Patterns:    []string{".mp4", ".mov"},
				Type:        string(modules.InputTypeFile),
			},
		},
		OptionalInputs: []modules.ModuleInput{
			{
				Name:        "ffmpegParams",
				Description: "Additional FFmpeg parameters",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "quietFlag",
				Description: "Suppress FFmpeg output",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "clips",
				Description: "Extracted video clips",
				Patterns:    []string{".mp4"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
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
func (m *Module) extractShortClip(ctx context.Context, short ShortClip, p Params) (string, error) {
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
	cmd := execCommand(ctx, "ffmpeg", args...)

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
		return "", fmt.Errorf("ffmpeg command failed: %w", err)
	}

	utils.LogSuccess("Extracted: %s", outputFilename)
	return outputPath, nil
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
