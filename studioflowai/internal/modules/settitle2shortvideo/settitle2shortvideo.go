package settitle2shortvideo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"gopkg.in/yaml.v3"
)

// execCommand allows us to mock exec.Command in tests
var execCommand = exec.CommandContext

// Module implements text overlay functionality
type Module struct{}

// Params contains the parameters for text overlay
type Params struct {
	Input      string `json:"input"`      // Path to input file or directory
	Output     string `json:"output"`     // Path to output directory
	VideoFile  string `json:"videoFile"`  // Path to the source video file
	Text       string `json:"text"`       // Text to overlay
	FontFile   string `json:"fontFile"`   // Path to the font file
	FontSize   int    `json:"fontSize"`   // Font size
	FontColor  string `json:"fontColor"`  // Font color
	Position   string `json:"position"`   // Text position (top, bottom, center)
	BoxColor   string `json:"boxColor"`   // Box color (default: "black@0.5")
	BoxBorderW int    `json:"boxBorderW"` // Box border width (default: 5)
	QuietFlag  bool   `json:"quietFlag"`  // Suppress ffmpeg output (default: true)
	TextX      string `json:"textX"`      // X position of text (default: "(w-text_w)/2")
	TextY      string `json:"textY"`      // Y position of text (default: "(h-text_h)/2")
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
	ShortTitle  string `yaml:"shortTitle"`
}

// New creates a new settitle2shortvideo module
func New() mod.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "set_title_to_short_video"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
	if err := mod.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Skip validation for files that will be created in the output directory
	if strings.Contains(resolvedInput, p.Output) {
		// Just verify it's a YAML file
		if !strings.HasSuffix(resolvedInput, ".yaml") {
			return fmt.Errorf("input file must be .yaml, got: %s", resolvedInput)
		}
	} else {
		// For existing files, do full validation
		if err := utils.ValidateInputPath(p.Input, p.Output, ""); err != nil {
			return err
		}

		// Try to parse the YAML file
		data, err := os.ReadFile(resolvedInput)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		var shortsData ShortsData
		if err := yaml.Unmarshal(data, &shortsData); err != nil {
			return fmt.Errorf("invalid YAML file: %w", err)
		}
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
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (mod.ModuleResult, error) {
	var p Params
	if err := mod.ParseParams(params, &p); err != nil {
		return mod.ModuleResult{}, err
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
		return mod.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Ensure we're reading a YAML file and specifically shorts_suggestions.yaml
	if !strings.HasSuffix(resolvedInput, ".yaml") {
		return mod.ModuleResult{}, fmt.Errorf("input file must be .yaml, got: %s", resolvedInput)
	}

	// Read and parse the shorts_suggestions.yaml file
	shortsData, err := readShortsFile(resolvedInput)
	if err != nil {
		return mod.ModuleResult{}, fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// Track processed clips and statistics
	processedClips := make(map[string]string)
	clipStats := make([]map[string]interface{}, 0)

	// Process each short clip
	for i, short := range shortsData.Shorts {
		// Validate required fields for this clip
		if short.StartTime == "" || short.EndTime == "" {
			return mod.ModuleResult{}, fmt.Errorf("short clip %d is missing required timing information", i+1)
		}

		// Use Title as ShortTitle if ShortTitle is empty
		if short.ShortTitle == "" {
			utils.LogWarning("Short clip %d is missing shortTitle, using title instead", i+1)
			short.ShortTitle = short.Title
		}

		outputPath, err := m.processShortClip(ctx, short, p)
		if err != nil {
			return mod.ModuleResult{}, fmt.Errorf("failed to process short clip %d: %w", i+1, err)
		}

		clipName := filepath.Base(outputPath)
		processedClips[clipName] = outputPath
		clipStats = append(clipStats, map[string]interface{}{
			"title":        short.Title,
			"short_title":  short.ShortTitle,
			"start_time":   short.StartTime,
			"end_time":     short.EndTime,
			"output_file":  outputPath,
			"font_size":    p.FontSize,
			"font_color":   p.FontColor,
			"box_color":    p.BoxColor,
			"box_border_w": p.BoxBorderW,
		})
	}

	utils.LogSuccess("Successfully processed %d short clips", len(shortsData.Shorts))

	return mod.ModuleResult{
		Outputs: processedClips,
		Statistics: map[string]interface{}{
			"input_file":    resolvedInput,
			"clips_count":   len(shortsData.Shorts),
			"clips_details": clipStats,
			"font_file":     p.FontFile,
			"font_settings": map[string]interface{}{
				"size":       p.FontSize,
				"color":      p.FontColor,
				"box_color":  p.BoxColor,
				"border_w":   p.BoxBorderW,
				"position_x": p.TextX,
				"position_y": p.TextY,
			},
			"process_time": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() mod.ModuleIO {
	return mod.ModuleIO{
		RequiredInputs: []mod.ModuleInput{
			{
				Name:        "input",
				Description: "Path to shorts suggestions YAML file",
				Patterns:    []string{".yaml"},
				Type:        string(mod.InputTypeFile),
			},
			{
				Name:        "output",
				Description: "Path to output directory",
				Type:        string(mod.InputTypeDirectory),
			},
		},
		OptionalInputs: []mod.ModuleInput{
			{
				Name:        "videoFile",
				Description: "Path to source video file (optional when using shorts_suggestions.yaml)",
				Type:        string(mod.InputTypeFile),
			},
			{
				Name:        "fontFile",
				Description: "Path to custom font file",
				Type:        string(mod.InputTypeFile),
			},
			{
				Name:        "fontSize",
				Description: "Font size for text overlay",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "fontColor",
				Description: "Font color for text overlay",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "boxColor",
				Description: "Background box color",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "boxBorderW",
				Description: "Background box border width",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "quietFlag",
				Description: "Suppress FFmpeg output",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "textX",
				Description: "X position of text",
				Type:        string(mod.InputTypeData),
			},
			{
				Name:        "textY",
				Description: "Y position of text",
				Type:        string(mod.InputTypeData),
			},
		},
		ProducedOutputs: []mod.ModuleOutput{
			{
				Name:        "videos",
				Description: "Processed video clips with text overlay",
				Patterns:    []string{"-withtext.mp4"},
				Type:        string(mod.OutputTypeFile),
			},
		},
	}
}

// readShortsFile reads and parses the shorts_suggestions.yaml file
func readShortsFile(filePath string) (*ShortsData, error) {
	// Ensure we're reading a file, not a directory
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shorts file: %w", err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("input path is a directory, expected a file: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shorts file: %w", err)
	}

	var shortsData ShortsData
	if err := yaml.Unmarshal(data, &shortsData); err != nil {
		return nil, fmt.Errorf("failed to parse shorts file: %w", err)
	}

	// Basic validation of required fields
	if len(shortsData.Shorts) == 0 {
		return nil, fmt.Errorf("no shorts found in shorts file")
	}

	return &shortsData, nil
}

// processShortClip adds text overlay to a single short clip
func (m *Module) processShortClip(ctx context.Context, short ShortClip, p Params) (string, error) {
	// Convert startTime and endTime to HHMMSS format for filename
	startTimeHHMMSS := convertToHHMMSS(short.StartTime)
	endTimeHHMMSS := convertToHHMMSS(short.EndTime)

	// Create input and output filenames with .mp4 extension
	inputFilename := fmt.Sprintf("%s-%s.mp4", startTimeHHMMSS, endTimeHHMMSS)
	outputFilename := fmt.Sprintf("%s-%s-withtext.mp4", startTimeHHMMSS, endTimeHHMMSS)
	outputPath := filepath.Join(p.Output, outputFilename)

	// First try to find the input file in the output directory
	inputPath := filepath.Join(p.Output, inputFilename)
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		// If not found in output directory, try the YAML directory
		yamlDir := filepath.Dir(utils.ResolveOutputPath(p.Input, p.Output))
		inputPath = filepath.Join(yamlDir, inputFilename)
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			return "", fmt.Errorf("input video file does not exist in either %s or %s",
				filepath.Join(p.Output, inputFilename),
				filepath.Join(yamlDir, inputFilename))
		}
	}

	// Build FFmpeg command for text overlay
	args := []string{
		"-i", inputPath,
	}

	// Add font file if specified and verify it exists
	fontFileArg := ""
	if p.FontFile != "" {
		if _, err := os.Stat(p.FontFile); os.IsNotExist(err) {
			return "", fmt.Errorf("font file does not exist: %s", p.FontFile)
		}
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
	cmd := execCommand(ctx, "ffmpeg", args...)

	// Configure output handling based on quiet mode
	var stderr strings.Builder
	if p.QuietFlag {
		cmd.Stdout = nil
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Run the FFmpeg command
	if err := cmd.Run(); err != nil {
		if p.QuietFlag && stderr.Len() > 0 {
			// Log the error output if we captured it
			utils.LogError("FFmpeg error: %s", stderr.String())
		}
		return "", fmt.Errorf("ffmpeg command failed: %w", err)
	}

	// Verify the output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("ffmpeg command completed but output file was not created: %s", outputPath)
	}

	utils.LogInfo("Added text overlay to: %s", outputFilename)
	return outputPath, nil
}

// convertToHHMMSS converts a timestamp like "00:01:23" to "000123"
func convertToHHMMSS(timestamp string) string {
	// Remove any non-numeric characters except digits
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, timestamp)

	// Pad with leading zeros if needed
	if len(digits) < 6 {
		digits = fmt.Sprintf("%06s", digits)
	}

	// Take only the first 6 digits
	if len(digits) > 6 {
		digits = digits[:6]
	}

	return digits
}
