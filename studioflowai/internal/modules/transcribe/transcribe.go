package transcribe

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"runtime/debug"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// CommandExecutor interface for executing commands
type CommandExecutor interface {
	ExecuteCommand(ctx context.Context, name string, args []string) ([]byte, error)
	LookPath(file string) (string, error)
}

// RealCommandExecutor implements actual command execution
type RealCommandExecutor struct{}

func (e *RealCommandExecutor) ExecuteCommand(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (e *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Module implements audio transcription functionality
type Module struct {
	cmdExecutor CommandExecutor
}

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
func New() modules.Module {
	return &Module{
		cmdExecutor: &RealCommandExecutor{},
	}
}

// NewWithExecutor creates a new transcribe module with a custom command executor
func NewWithExecutor(executor CommandExecutor) modules.Module {
	return &Module{
		cmdExecutor: executor,
	}
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

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, ""); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
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

	// Validate audio file extension if input is a file
	fileInfo, err := os.Stat(p.Input)
	if err == nil && !fileInfo.IsDir() {
		if err := utils.ValidateFileExtension(p.Input, []string{".wav", ".mp3", ".m4a", ".aac"}); err != nil {
			return err
		}
	}

	// Set default model if not specified
	if p.Model == "" {
		p.Model = "whisper"
	}

	// Validate model selection and check if installed
	switch p.Model {
	case "whisper":
		if _, err := m.cmdExecutor.LookPath("whisper"); err != nil {
			utils.LogWarning("whisper CLI not found in PATH; transcription module will look for existing transcription files instead")
		}
	case "whisper-cli":
		if _, err := m.cmdExecutor.LookPath("whisper-cli"); err != nil {
			utils.LogWarning("whisper-cli not found in PATH; transcription module will look for existing transcription files instead")
		}
	case "external":
		// External model is allowed but doesn't need validation
	default:
		return fmt.Errorf("unsupported transcription model: %s", p.Model)
	}

	// Validate output format
	if p.OutputFormat != "" {
		validFormats := map[string]bool{
			"txt":  true,
			"srt":  true,
			"vtt":  true,
			"json": true,
		}
		if !validFormats[p.OutputFormat] {
			return fmt.Errorf("unsupported output format: %s", p.OutputFormat)
		}
	}

	return nil
}

// Execute transcribes audio files to text
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
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
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if the preferred model is installed
	modelInstalled := true
	if p.Model == "whisper" {
		_, err := m.cmdExecutor.LookPath("whisper")
		modelInstalled = (err == nil)
	}

	// If the model isn't installed, look for existing transcription files
	if !modelInstalled {
		utils.LogWarning("Transcription model not available, looking for existing transcription files")
		if err := m.findExistingTranscripts(p); err != nil {
			return modules.ModuleResult{}, err
		}
		return modules.ModuleResult{}, nil
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Log the exact path we're looking for
	utils.LogVerbose("Looking for input file: %s", resolvedInput)

	// Check if input is a directory or a file
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		// Try to handle common path variants
		altPaths := []string{
			// Try without leading ./
			strings.TrimPrefix(resolvedInput, "./"),
			// Try with absolute path
			filepath.Join(filepath.Dir(p.Output), filepath.Base(resolvedInput)),
			// Try in output directory
			filepath.Join(p.Output, filepath.Base(resolvedInput)),
		}

		for _, altPath := range altPaths {
			utils.LogDebug("Trying alternative path: %s", altPath)
			if fileInfo, err = os.Stat(altPath); err == nil {
				resolvedInput = altPath
				utils.LogVerbose("Found input file at: %s", altPath)
				break
			}
		}

		if err != nil {
			return modules.ModuleResult{}, fmt.Errorf("failed to access input: %w", err)
		}
	}

	if fileInfo.IsDir() {
		// Process all matching audio files in the directory
		if err := m.processDirectory(ctx, p); err != nil {
			return modules.ModuleResult{}, err
		}
		return modules.ModuleResult{}, nil
	}

	// Process a single file
	if err := m.processFile(ctx, resolvedInput, p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Create result with output file information
	outputFile := p.OutputFileName
	if outputFile == "" {
		outputFile = filepath.Base(resolvedInput)
		outputFile = outputFile[:len(outputFile)-len(filepath.Ext(outputFile))]
	}
	outputFile = outputFile + "." + p.OutputFormat

	result := modules.ModuleResult{
		Outputs: map[string]string{
			"transcript": filepath.Join(p.Output, outputFile),
		},
		Metadata: map[string]interface{}{
			"model":    p.Model,
			"format":   p.OutputFormat,
			"language": p.Language,
		},
	}

	return result, nil
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

	var err error
	switch p.Model {
	case "whisper":
		args := m.buildWhisperCommand(filePath, outputFile, p)
		cmd := exec.CommandContext(ctx, p.Model, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	case "whisper-cli":
		// For whisper-cli, use the splitting workflow
		err = m.processWhisperCliWithSplitting(ctx, filePath, outputFile, p)
	default:
		return fmt.Errorf("unsupported transcription model: %s", p.Model)
	}

	if err != nil {
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
					if err := os.Remove(outputFile); err != nil && !os.IsNotExist(err) {
						utils.LogWarning("Failed to remove existing file: %v", err)
					}
					// Move the file
					if err := os.Rename(match, outputFile); err != nil {
						utils.LogWarning("Failed to rename file: %v", err)
					}
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

	// Set output directory and format
	outputDir := filepath.Dir(outputFile)
	if !containsParam(args, "--output_dir") {
		args = append(args, "--output_dir", outputDir)
	}
	if !containsParam(args, "--output_format") {
		args = append(args, "--output_format", p.OutputFormat)
	}

	return args
}

// buildWhisperCliCommand constructs the whisper-cli command arguments
func (m *Module) buildWhisperCliCommand(inputFile, outputFile string, p Params) []string {
	var args []string

	// Handle custom parameters first
	if p.WhisperParams != "" {
		args = strings.Fields(p.WhisperParams)
	}

	// Set default parameters if not provided in WhisperParams
	if !containsParam(args, "-t") && !containsParam(args, "--threads") {
		args = append(args, "--threads", "16")
	}
	if !containsParam(args, "-bs") && !containsParam(args, "--beam-size") {
		args = append(args, "--beam-size", "5")
	}
	if !containsParam(args, "-bo") && !containsParam(args, "--best-of") {
		args = append(args, "--best-of", "5")
	}
	if !containsParam(args, "-tp") && !containsParam(args, "--temperature") {
		args = append(args, "--temperature", "0.0")
	}

	// Set language if specified
	if p.Language != "" && p.Language != "auto" {
		args = append(args, "--language", p.Language)
	}

	// Set output format
	switch p.OutputFormat {
	case "txt":
		args = append(args, "--output-txt")
	case "srt":
		args = append(args, "--output-srt")
	case "vtt":
		args = append(args, "--output-vtt")
	case "json":
		args = append(args, "--output-json")
	}

	// Set output file
	if outputFile != "" {
		args = append(args, "--output-file", strings.TrimSuffix(outputFile, filepath.Ext(outputFile)))
	}

	// Add input file at the end
	args = append(args, inputFile)

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
func (m *Module) findExistingTranscripts(p Params) error {
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
	defer func() {
		if cerr := in.Close(); cerr != nil {
			utils.LogWarning("Failed to close input file: %v", cerr)
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			utils.LogWarning("Failed to close output file: %v", cerr)
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

// sortNaturally sorts strings in natural order (e.g., split_1.wav comes before split_10.wav)
func sortNaturally(strs []string) {
	sort.Slice(strs, func(i, j int) bool {
		return naturalLess(strs[i], strs[j])
	})
}

// naturalLess compares strings in natural order
func naturalLess(str1, str2 string) bool {
	// Split strings into chunks of digits and non-digits
	chunks1 := splitIntoChunks(str1)
	chunks2 := splitIntoChunks(str2)

	// Compare chunks
	for i := 0; i < len(chunks1) && i < len(chunks2); i++ {
		// If both chunks are numeric, compare as numbers
		num1, err1 := strconv.Atoi(chunks1[i])
		num2, err2 := strconv.Atoi(chunks2[i])
		if err1 == nil && err2 == nil {
			if num1 != num2 {
				return num1 < num2
			}
			continue
		}
		// Otherwise compare as strings
		if chunks1[i] != chunks2[i] {
			return chunks1[i] < chunks2[i]
		}
	}
	return len(chunks1) < len(chunks2)
}

// splitIntoChunks splits a string into chunks of digits and non-digits
func splitIntoChunks(s string) []string {
	var chunks []string
	var current strings.Builder
	var isDigit bool
	var started bool

	for _, ch := range s {
		currentIsDigit := unicode.IsDigit(ch)
		if !started {
			isDigit = currentIsDigit
			started = true
		}

		if currentIsDigit == isDigit {
			current.WriteRune(ch)
		} else {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteRune(ch)
			isDigit = currentIsDigit
		}
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// splitAudioFile splits an audio file into segments of specified duration (in seconds)
func (m *Module) splitAudioFile(ctx context.Context, inputFile string, outputDir string) ([]string, error) {
	// Create a temporary directory for split files
	splitDir := filepath.Join(outputDir, "splits")
	if err := os.MkdirAll(splitDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create splits directory: %w", err)
	}

	// Construct the ffmpeg command for splitting
	splitPattern := filepath.Join(splitDir, "split_%03d.wav")
	args := []string{
		"-i", inputFile,
		"-f", "segment",
		"-segment_time", "600", // 10 minutes = 600 seconds
		"-c", "copy",
		splitPattern,
	}

	// Run the command
	if output, err := m.cmdExecutor.ExecuteCommand(ctx, "ffmpeg", args); err != nil {
		return nil, fmt.Errorf("failed to split audio: %s, error: %w", string(output), err)
	}

	// Get the list of split files
	splitFiles, err := filepath.Glob(filepath.Join(splitDir, "split_*.wav"))
	if err != nil {
		return nil, fmt.Errorf("failed to list split files: %w", err)
	}

	// Sort the files to ensure they're in order
	sortNaturally(splitFiles)
	return splitFiles, nil
}

// parseTimestamp parses an SRT timestamp into hours, minutes, seconds, and milliseconds
func parseTimestamp(timestamp string) (int, int, int, int, error) {
	var hours, minutes, seconds, milliseconds int
	n, err := fmt.Sscanf(timestamp, "%d:%d:%d,%d", &hours, &minutes, &seconds, &milliseconds)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	if n != 4 {
		return 0, 0, 0, 0, fmt.Errorf("invalid timestamp format")
	}

	// Validate hours (0-99)
	if hours < 0 || hours > 99 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hours value: %d", hours)
	}

	// Validate minutes (0-59)
	if minutes < 0 || minutes > 59 {
		return 0, 0, 0, 0, fmt.Errorf("invalid minutes value: %d", minutes)
	}

	// Validate seconds (0-59)
	if seconds < 0 || seconds > 59 {
		return 0, 0, 0, 0, fmt.Errorf("invalid seconds value: %d", seconds)
	}

	// Validate milliseconds (0-999)
	if milliseconds < 0 || milliseconds > 999 {
		return 0, 0, 0, 0, fmt.Errorf("invalid milliseconds value: %d", milliseconds)
	}

	return hours, minutes, seconds, milliseconds, nil
}

// formatTimestamp formats hours, minutes, seconds, and milliseconds into an SRT timestamp
func formatTimestamp(hours, minutes, seconds, milliseconds int) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}

// adjustTimestamp adds an offset (in seconds) to an SRT timestamp
func adjustTimestamp(timestamp string, offsetSeconds int) (string, error) {
	hours, minutes, seconds, milliseconds, err := parseTimestamp(timestamp)
	if err != nil {
		return "", err
	}

	// Convert everything to milliseconds for easier calculation
	totalMs := (hours*3600+minutes*60+seconds)*1000 + milliseconds
	totalMs += offsetSeconds * 1000

	// Convert back to h:m:s,ms
	newHours := totalMs / (3600 * 1000)
	totalMs %= 3600 * 1000
	newMinutes := totalMs / (60 * 1000)
	totalMs %= 60 * 1000
	newSeconds := totalMs / 1000
	newMilliseconds := totalMs % 1000

	return formatTimestamp(newHours, newMinutes, newSeconds, newMilliseconds), nil
}

// forceMemoryCleanup performs aggressive memory cleanup
func forceMemoryCleanup() {
	// Run garbage collection multiple times to ensure maximum cleanup
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	// Force release of memory to OS
	debug.FreeOSMemory()
}

// waitForMemoryCleanup waits for memory to be cleaned up
func waitForMemoryCleanup(ctx context.Context) error {
	fmt.Printf("\n\033[35m[Memory Cleanup]\033[0m Waiting 5 seconds to clean up RAM memory before next segment...\n")

	// Create a ticker for progress indication
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Create a timer for the total wait time
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	// Start cleanup
	forceMemoryCleanup()

	// Wait and show progress
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			// Final cleanup before continuing
			forceMemoryCleanup()
			return nil
		case <-ticker.C:
			// Run cleanup every 5 seconds while waiting
			forceMemoryCleanup()
			// utils.LogVerbose("Still cleaning memory...")
		}
	}
}

// processWhisperCliWithSplitting handles the complete workflow for whisper-cli with audio splitting
func (m *Module) processWhisperCliWithSplitting(ctx context.Context, inputFile string, outputFile string, p Params) error {
	// Create a temporary directory for processing
	tempDir := filepath.Join(filepath.Dir(outputFile), "temp_transcribe")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		// Clean up temp files
		if err := os.RemoveAll(tempDir); err != nil {
			utils.LogWarning("Failed to remove temp directory: %v", err)
		}
		// Force memory cleanup
		forceMemoryCleanup()
	}()

	// Split the audio file
	splitFiles, err := m.splitAudioFile(ctx, inputFile, tempDir)
	if err != nil {
		return fmt.Errorf("failed to split audio: %w", err)
	}

	// Process each split file and immediately merge to final output
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			utils.LogWarning("Failed to close output file: %v", cerr)
		}
	}()

	var subtitleIndex = 1
	var timeOffset = 0 // offset in seconds

	totalSegments := len(splitFiles)
	for i, splitFile := range splitFiles {
		// If this is not the first segment, wait for memory cleanup
		if i > 0 {
			if err := waitForMemoryCleanup(ctx); err != nil {
				return fmt.Errorf("memory cleanup interrupted: %w", err)
			}
		}

		fmt.Printf("\n\033[36m[Progress]\033[0m Processing segment %d/%d\n", i+1, totalSegments)

		// Generate output path for this segment
		segmentOutput := filepath.Join(tempDir, fmt.Sprintf("segment_%03d.srt", i))

		// Build whisper-cli command for this segment
		args := m.buildWhisperCliCommand(splitFile, segmentOutput, p)

		// Execute the command
		output, err := m.cmdExecutor.ExecuteCommand(ctx, "whisper-cli", args)
		if err != nil {
			return fmt.Errorf("whisper-cli failed for segment %d: %w", i+1, err)
		}

		// Process the output if needed
		if len(output) > 0 {
			utils.LogVerbose("whisper-cli output: %s", string(output))
		}

		// Process this segment's transcription and append to final file
		if err := m.processAndAppendTranscription(segmentOutput, outFile, &subtitleIndex, timeOffset); err != nil {
			return fmt.Errorf("failed to process segment %d: %w", i+1, err)
		}

		// Clean up segment files immediately
		if err := os.Remove(segmentOutput); err != nil {
			utils.LogWarning("Failed to remove segment output: %v", err)
		}
		if err := os.Remove(splitFile); err != nil {
			utils.LogWarning("Failed to remove split file: %v", err)
		}

		// Update time offset for next file (10 minutes = 600 seconds)
		timeOffset += 600

		// Force cleanup after processing each segment
		forceMemoryCleanup()
	}

	fmt.Printf("\n\033[32m[Complete]\033[0m Successfully transcribed all %d segments\n", totalSegments)
	return nil
}

// processAndAppendTranscription processes a single transcription file and appends it to the output
func (m *Module) processAndAppendTranscription(inputFile string, outFile *os.File, subtitleIndex *int, timeOffset int) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", inputFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var currentBlock []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			if len(currentBlock) > 0 {
				// Process and write the current block
				if len(currentBlock) >= 3 {
					// Write subtitle index
					if _, err := fmt.Fprintf(outFile, "%d\n", *subtitleIndex); err != nil {
						return fmt.Errorf("failed to write subtitle index: %w", err)
					}
					*subtitleIndex++

					// Process timestamp line
					timestamps := strings.Split(currentBlock[1], " --> ")
					if len(timestamps) == 2 {
						startTime, err := adjustTimestamp(timestamps[0], timeOffset)
						if err != nil {
							return err
						}
						endTime, err := adjustTimestamp(timestamps[1], timeOffset)
						if err != nil {
							return err
						}
						if _, err := fmt.Fprintf(outFile, "%s --> %s\n", startTime, endTime); err != nil {
							return fmt.Errorf("failed to write timestamps: %w", err)
						}

						// Write subtitle text and display it
						for i := 2; i < len(currentBlock); i++ {
							if _, err := fmt.Fprintln(outFile, currentBlock[i]); err != nil {
								return fmt.Errorf("failed to write subtitle text: %w", err)
							}
						}
						if _, err := fmt.Fprintln(outFile); err != nil {
							return fmt.Errorf("failed to write empty line: %w", err)
						}
					}
				}
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}

	return nil
}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to input audio file",
				Patterns:    []string{".wav", ".mp3", ".m4a", ".aac"},
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
				Name:        "model",
				Description: "Transcription model to use (default: whisper)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "language",
				Description: "Language for transcription (default: auto)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "outputFormat",
				Description: "Output format (default: srt)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "whisperParams",
				Description: "Additional parameters for Whisper CLI",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "outputFileName",
				Description: "Custom output file name (without extension)",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "transcript",
				Description: "Transcription file",
				Patterns:    []string{".txt", ".srt"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
}
