package transcribe

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// Module implements audio transcription functionality
type Module struct{}

// Params contains the parameters for audio transcription
type Params struct {
	Input          string `json:"input"`          // Path to input audio file
	Output         string `json:"output"`         // Path to output directory
	Model          string `json:"model"`          // Transcription model to use (default: "whisper")
	Language       string `json:"language"`       // Language for transcription (default: "auto")
	OutputFormat   string `json:"outputFormat"`   // Output format (default: "txt")
	WhisperParams  string `json:"whisperParams"`  // Additional parameters for Whisper CLI
	OutputFileName string `json:"outputFileName"` // Custom output file name (without extension)
}

// New creates a new transcribe module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "transcribe"
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

	// During validation, we don't check file existence for input files inside an output directory,
	// as they'll be created during workflow execution.
	if strings.Contains(p.Input, "output") ||
		strings.Contains(p.Input, "/audio.wav") ||
		filepath.Base(p.Input) == "audio.wav" {
		// Skip file existence check for expected output files from previous steps
		utils.LogVerbose("Note: Input file %s will be created by a previous step", p.Input)
		return nil
	}

	// Ensure the input file or directory exists
	if _, err := os.Stat(p.Input); os.IsNotExist(err) {
		return fmt.Errorf("input path %s does not exist", p.Input)
	}

	// Check if the selected model is installed - but don't fail if not
	if p.Model == "whisper" {
		if _, err := exec.LookPath("whisper"); err != nil {
			utils.LogWarning("whisper CLI not found in PATH; transcription module will look for existing transcription files instead")
		}
	} else if p.Model != "" && p.Model != "external" {
		return fmt.Errorf("unsupported transcription model: %s", p.Model)
	}

	return nil
}

// Execute transcribes audio files to text
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.Model == "" {
		p.Model = "whisper"
	}
	if p.OutputFormat == "" {
		p.OutputFormat = "srt" // Default to SRT instead of TXT
	}

	// Set default Whisper parameters if none provided
	if p.WhisperParams == "" {
		p.WhisperParams = "--model large-v2 --beam_size 5 --temperature 0.0 --best_of 5 --word_timestamps True --threads 16 --patience 1.0 --condition_on_previous_text True"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if the preferred model is installed
	modelInstalled := true
	if p.Model == "whisper" {
		_, err := exec.LookPath("whisper")
		modelInstalled = (err == nil)
	}

	// If the model isn't installed, look for existing transcription files
	if !modelInstalled {
		utils.LogWarning("Transcription model not available, looking for existing transcription files")
		return m.findExistingTranscripts(ctx, p)
	}

	// Log the exact path we're looking for
	utils.LogVerbose("Looking for input file: %s", p.Input)

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(p.Input)
	if err != nil {
		// Try to handle common path variants
		altPaths := []string{
			// Try without leading ./
			strings.TrimPrefix(p.Input, "./"),
			// Try with absolute path
			filepath.Join(filepath.Dir(p.Output), filepath.Base(p.Input)),
			// Try in output directory
			filepath.Join(p.Output, filepath.Base(p.Input)),
		}

		for _, altPath := range altPaths {
			utils.LogDebug("Trying alternative path: %s", altPath)
			if fileInfo, err = os.Stat(altPath); err == nil {
				p.Input = altPath
				utils.LogVerbose("Found input file at: %s", altPath)
				break
			}
		}

		if err != nil {
			return fmt.Errorf("failed to access input: %w", err)
		}
	}

	if fileInfo.IsDir() {
		// Process all matching audio files in the directory
		return m.processDirectory(ctx, p)
	}

	// Process a single file
	return m.processFile(ctx, p.Input, p)
}

// processDirectory processes all matching audio files in a directory
func (m *Module) processDirectory(ctx context.Context, p Params) error {
	entries, err := filepath.Glob(filepath.Join(p.Input, "*.wav"))
	if err != nil {
		return fmt.Errorf("failed to glob input files: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no matching files found for pattern *.wav")
	}

	for _, entry := range entries {
		if err := m.processFile(ctx, entry, p); err != nil {
			return err
		}
	}

	return nil
}

// processFile transcribes a single audio file
func (m *Module) processFile(ctx context.Context, filePath string, p Params) error {
	filename := filepath.Base(filePath)
	baseName := filename[:len(filename)-len(filepath.Ext(filename))]

	// Use custom output file name if provided, otherwise use the base name
	outputBaseName := baseName
	if p.OutputFileName != "" {
		outputBaseName = p.OutputFileName
	}

	// Match the original script's output naming convention - keep the same base filename
	outputFile := filepath.Join(p.Output, outputBaseName+"."+p.OutputFormat)

	utils.LogVerbose("Transcribing %s to %s", filePath, outputFile)

	var args []string

	// Construct command based on the selected model
	if p.Model == "whisper" {
		args = m.buildWhisperCommand(filePath, outputFile, p)
	}

	// Prepare the command
	cmd := exec.CommandContext(ctx, p.Model, args...)

	// Show Whisper output in the console, but don't store in logs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the transcription command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("transcription command failed: %w", err)
	}

	// Whisper sometimes adds a suffix for the language detected
	// Check for any files that contain the base name and move them if needed
	if p.OutputFormat == "txt" || p.OutputFormat == "srt" {
		// Look for any files with the base name in the output directory
		matches, err := filepath.Glob(filepath.Join(p.Output, baseName+"*."+p.OutputFormat))
		if err == nil && len(matches) > 0 {
			// If there's a different file than what we expect, rename it
			for _, match := range matches {
				if match != outputFile {
					utils.LogVerbose("Found additional output file: %s, moving to %s", match, outputFile)
					// Remove existing file if it exists
					os.Remove(outputFile)
					// Move the file
					os.Rename(match, outputFile)
					break
				}
			}
		}
	}

	utils.LogSuccess("Successfully transcribed %s", filePath)
	return nil
}

// buildWhisperCommand constructs the Whisper CLI command arguments
func (m *Module) buildWhisperCommand(inputFile, outputFile string, p Params) []string {
	// Start with any custom parameters
	var args []string
	if p.WhisperParams != "" {
		args = strings.Fields(p.WhisperParams)
	}

	// Add the input file as the first argument
	args = append([]string{inputFile}, args...)

	// Only add language if explicitly specified
	// if p.Language != "" && !containsParam(args, "--language") {
	// 	args = append(args, "--language", p.Language)
	// }

	// Check if output format is already specified
	if !containsParam(args, "--output_format") {
		args = append(args, "--output_format", p.OutputFormat)
	}

	// Check if output directory is already specified
	if !containsParam(args, "--output_dir") {
		args = append(args, "--output_dir", p.Output)
	}

	return args
}

// containsParam checks if a parameter is already in the arguments list
func containsParam(args []string, param string) bool {
	for _, arg := range args {
		if arg == param {
			return true
		}
	}
	return false
}

// findExistingTranscripts tries to find existing transcription files that match the audio files
func (m *Module) findExistingTranscripts(ctx context.Context, p Params) error {
	// This is the fallback when transcription tools aren't installed
	// Check if there are already transcription files for the audio
	baseDir := filepath.Dir(p.Input)

	// In the original script, transcription files were expected to be in the same directory as audio files
	// with the same base name but different extension
	wavPattern := filepath.Join(baseDir, "*.wav")
	wavFiles, err := filepath.Glob(wavPattern)
	if err != nil {
		return fmt.Errorf("failed to find audio files: %w", err)
	}

	if len(wavFiles) == 0 {
		return fmt.Errorf("no audio files found matching %s", wavPattern)
	}

	// For each audio file, look for a corresponding transcript file
	found := false
	for _, wavFile := range wavFiles {
		basename := filepath.Base(wavFile)
		baseWithoutExt := basename[:len(basename)-len(filepath.Ext(basename))]

		// Look for both SRT and TXT files
		possibleExtensions := []string{".srt", ".txt"}
		for _, ext := range possibleExtensions {
			transcriptFile := filepath.Join(baseDir, baseWithoutExt+ext)

			// Check if the transcript exists
			if _, err := os.Stat(transcriptFile); err == nil {
				// Use custom output file name if provided
				outBaseName := baseWithoutExt
				if p.OutputFileName != "" {
					outBaseName = p.OutputFileName
				}

				// Copy the transcript to the output directory - convert extension if needed
				outExt := filepath.Ext(transcriptFile)
				if p.OutputFormat != "" {
					outExt = "." + p.OutputFormat
				}

				outFile := filepath.Join(p.Output, outBaseName+outExt)
				if err := copyFile(transcriptFile, outFile); err != nil {
					utils.LogWarning("Failed to copy transcript %s: %v", transcriptFile, err)
					continue
				}
				utils.LogVerbose("Found existing transcript: %s", transcriptFile)
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("no existing transcription files found matching the audio files")
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
