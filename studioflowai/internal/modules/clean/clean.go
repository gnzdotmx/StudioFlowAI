package clean

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// Module implements transcript cleaning functionality
type Module struct{}

// Params contains the parameters for transcript cleaning
type Params struct {
	Input           string   `json:"input"`           // Path to input transcript file
	Output          string   `json:"output"`          // Path to output directory
	RemovePatterns  []string `json:"removePatterns"`  // Patterns to remove from each line
	CombineOutput   bool     `json:"combineOutput"`   // Whether to combine all transcripts (default: true) - deprecated
	CleanFileSuffix string   `json:"cleanFileSuffix"` // Suffix for clean files (default: "_clean")
	InputFileName   string   `json:"inputFileName"`   // Specific input file name to process (without extension)
	OutputFileName  string   `json:"outputFileName"`  // Custom output file name (without extension)
}

// New creates a new clean module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "clean"
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
	// Also, don't validate against inputFileName as it may not be resolved to an actual path yet.
	// Just ensure parameters are present.
	if p.InputFileName != "" {
		// If we have a specific filename, validation is sufficient
		// Skip file existence check as this could be created during workflow execution
		return nil
	}

	// Only validate file existence for external input paths
	_, err := os.Stat(p.Input)
	if err != nil {
		// For files that don't exist, check if they might be in the output directory
		// as they could be created by previous steps
		if strings.Contains(p.Input, p.Output) ||
			strings.Contains(p.Input, "output") ||
			filepath.Base(p.Input) == "transcript.srt" {
			// Skip validation for expected output files
			return nil
		}
		return fmt.Errorf("input file does not exist: %w", err)
	}

	// Check if it's a directory but no inputFileName is provided
	fileInfo, err := os.Stat(p.Input)
	if err == nil && fileInfo.IsDir() && p.InputFileName == "" {
		return fmt.Errorf("input is a directory but no inputFileName specified")
	}

	return nil
}

// Execute cleans transcript files
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) error {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.CleanFileSuffix == "" {
		p.CleanFileSuffix = "_clean"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Verify input exists at execution time (now that previous steps have completed)
	fileInfo, err := os.Stat(p.Input)
	if err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("input must be a file, not a directory: %s", p.Input)
	}

	// Process the single file
	filename := filepath.Base(p.Input)

	// Check if the file is a text file, not binary
	if !utils.IsTextFile(p.Input) {
		return fmt.Errorf("file %s appears to be binary, not a text file - skipping", p.Input)
	}

	// Determine output filename
	var outputBaseName string
	if p.OutputFileName != "" {
		outputBaseName = p.OutputFileName
	} else {
		outputBaseName = filename[:len(filename)-len(filepath.Ext(filename))]
	}

	outputPath := filepath.Join(p.Output, outputBaseName+p.CleanFileSuffix+".txt")

	if err := m.cleanFile(p.Input, outputPath, p); err != nil {
		return err
	}

	// Create the additional formats for SRT files
	if filepath.Ext(p.Input) == ".srt" {
		if err := m.createCleanFormats(p.Input, p.Output, p); err != nil {
			return err
		}
	}

	utils.LogSuccess("Cleaned %s -> %s", p.Input, outputPath)

	return nil
}

// cleanFile cleans a single transcript file
func (m *Module) cleanFile(inputPath, outputPath string, p Params) error {
	// Check if the file is a text file
	if !utils.IsTextFile(inputPath) {
		return fmt.Errorf("file %s appears to be binary, not a text file - skipping", inputPath)
	}

	// Compile removal patterns
	var removeRegexes []*regexp.Regexp
	for _, pattern := range p.RemovePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid removal pattern %s: %w", pattern, err)
		}
		removeRegexes = append(removeRegexes, re)
	}

	// Compile standard timestamp regex (two spaces followed by parenthetical content)
	timestampRegex := regexp.MustCompile(`  \(.*\)`)

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	utils.LogVerbose("Cleaning file: %s", inputPath)

	// Process based on file extension
	fileExt := strings.ToLower(filepath.Ext(inputPath))
	if fileExt == ".srt" {
		// Special handling for SRT files
		if err := m.cleanSRTFile(scanner, writer, removeRegexes, timestampRegex); err != nil {
			return err
		}
	} else {
		// Default handling for other text files
		for scanner.Scan() {
			line := scanner.Text()

			// Apply all removal patterns
			for _, re := range removeRegexes {
				line = re.ReplaceAllString(line, "")
			}

			// Remove timestamps
			line = timestampRegex.ReplaceAllString(line, "")

			// Write the cleaned line
			if line != "" {
				if _, err := writer.WriteString(line + "\n"); err != nil {
					return fmt.Errorf("failed to write to output: %w", err)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
	}

	return nil
}

// cleanSRTFile cleans an SRT format subtitle file
func (m *Module) cleanSRTFile(scanner *bufio.Scanner, writer *bufio.Writer, removeRegexes []*regexp.Regexp, timestampRegex *regexp.Regexp) error {
	/*
		SRT format is:
		1
		00:00:20,000 --> 00:00:24,400
		Text line 1
		Text line 2

		2
		...
	*/

	// Track the current subtitle block we're building
	var currentBlock []string
	var subtitleNumber int

	// Process in 4-line blocks (number, timestamp, text, blank line)
	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is likely a subtitle number line
		if len(currentBlock) == 0 && isSubtitleNumber(line) {
			subtitleNumber++
			currentBlock = append(currentBlock, fmt.Sprintf("%d", subtitleNumber)) // Use our own numbering
			continue
		}

		// If we have a subtitle number, next line should be the timestamp
		if len(currentBlock) == 1 && isTimestamp(line) {
			currentBlock = append(currentBlock, line) // Keep timestamp as is
			continue
		}

		// If we have number + timestamp, next lines are subtitle text until blank line
		if len(currentBlock) >= 2 {
			// Process text with timestamp regex before adding to block
			if !isTimestamp(line) && line != "" {
				// Apply timestamp regex to clean the text
				line = timestampRegex.ReplaceAllString(line, "")
			}

			// Blank line signals end of block
			if line == "" {
				// Write the complete block if it has content
				if len(currentBlock) > 2 { // Only if we have actual text
					for _, blockLine := range currentBlock {
						if _, err := writer.WriteString(blockLine + "\n"); err != nil {
							return fmt.Errorf("failed to write to output file: %w", err)
						}
					}
					// Add blank line
					if _, err := writer.WriteString("\n"); err != nil {
						return fmt.Errorf("failed to write to output file: %w", err)
					}
				}
				currentBlock = nil // Reset for next block
				continue
			}

			// Check if this line should be removed
			skip := false
			for _, regex := range removeRegexes {
				if regex.MatchString(line) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			// Add subtitle text to current block
			currentBlock = append(currentBlock, line)
		}
	}

	// Handle the last block if there is one
	if len(currentBlock) > 2 {
		for _, line := range currentBlock {
			if _, err := writer.WriteString(line + "\n"); err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
		}
		// Add final blank line
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	return nil
}

// createCleanFormats creates the two additional formats requested:
// 1. full_transcript_clean.txt - no chunk numbers, no timestamps
// 2. full_transcript_with_timestamps.txt - no chunk numbers, with timestamps
func (m *Module) createCleanFormats(inputPath, outputDir string, p Params) error {
	utils.LogVerbose("Creating clean formats for %s", inputPath)

	// Extract base name from input file
	filename := filepath.Base(inputPath)
	baseFilename := filename[:len(filename)-len(filepath.Ext(filename))]

	// Determine output file name
	var outputBaseName string
	if p.OutputFileName != "" {
		outputBaseName = p.OutputFileName
	} else {
		outputBaseName = baseFilename
	}

	// Extract only the text content from the SRT file
	textOutputPath := filepath.Join(outputDir, outputBaseName+"_text"+p.CleanFileSuffix+".txt")
	if err := m.extractTextFromSRT(inputPath, textOutputPath); err != nil {
		return fmt.Errorf("failed to extract text from SRT: %w", err)
	}

	utils.LogSuccess("Created text-only format: %s", textOutputPath)
	return nil
}

// isSubtitleNumber checks if a line is likely a subtitle number
func isSubtitleNumber(line string) bool {
	// Try to parse as integer
	_, err := strconv.Atoi(strings.TrimSpace(line))
	return err == nil
}

// isTimestamp checks if a line matches the SRT timestamp format
func isTimestamp(line string) bool {
	// Simple regex to match SRT timestamp format: 00:00:00,000 --> 00:00:00,000
	timestampPattern := `^\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}$`
	matched, _ := regexp.MatchString(timestampPattern, strings.TrimSpace(line))
	return matched
}

// extractTextFromSRT extracts only the text content from an SRT file
func (m *Module) extractTextFromSRT(inputPath, outputPath string) error {
	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open SRT file: %w", err)
	}
	defer inputFile.Close()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	state := 0 // 0: expecting chunk number, 1: expecting timestamp, 2: expecting text
	var textLines []string

	for scanner.Scan() {
		line := scanner.Text()

		switch state {
		case 0: // Expecting chunk number
			// Just skip the chunk number line
			state = 1

		case 1: // Expecting timestamp
			if isTimestamp(line) {
				state = 2
				textLines = nil // Clear text buffer
			} else {
				// Something is wrong, reset
				state = 0
			}

		case 2: // Expecting text
			if line == "" {
				// Empty line means end of text block
				state = 0 // Reset for next chunk

				// Process collected text
				if len(textLines) > 0 {
					// Write text lines to the file
					for _, textLine := range textLines {
						if _, err := writer.WriteString(textLine + "\n"); err != nil {
							return fmt.Errorf("failed to write to output file: %w", err)
						}
					}

					// Add blank line after each block
					if _, err := writer.WriteString("\n"); err != nil {
						return fmt.Errorf("failed to write blank line: %w", err)
					}
				}
			} else {
				textLines = append(textLines, line)
			}
		}
	}

	// Handle last block if any
	if len(textLines) > 0 {
		// Write text lines to the file
		for _, textLine := range textLines {
			if _, err := writer.WriteString(textLine + "\n"); err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	return nil
}
