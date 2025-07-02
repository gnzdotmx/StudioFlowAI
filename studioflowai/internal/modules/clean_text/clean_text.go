package cleantext

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// Module implements text cleaning functionality
type Module struct{}

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to input text file",
				Patterns:    []string{".txt", ".srt"},
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
				Name:        "removePatterns",
				Description: "Regex patterns to remove from each line",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "cleanFileSuffix",
				Description: "Suffix for cleaned files (default: _clean)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "inputFileName",
				Description: "Specific input file name to process (without extension)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "outputFileName",
				Description: "Custom output file name (without extension)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "preserveTimestamps",
				Description: "Whether to preserve timestamps in SRT files (default: false)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "preserveLineBreaks",
				Description: "Whether to preserve line breaks (default: true)",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "cleaned",
				Description: "Cleaned text file",
				Patterns:    []string{".txt"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
}

// Params contains the parameters for text cleaning
type Params struct {
	Input             string   `json:"input"`             // Path to input text file
	Output            string   `json:"output"`            // Path to output directory
	RemovePatterns    []string `json:"removePatterns"`    // Patterns to remove from each line
	CleanFileSuffix   string   `json:"cleanFileSuffix"`   // Suffix for cleaned files (default: "_clean")
	InputFileName     string   `json:"inputFileName"`     // Specific input file name to process
	OutputFileName    string   `json:"outputFileName"`    // Custom output file name (without extension)
	PreserveTimestamp bool     `json:"preserveTimestamp"` // Whether to preserve timestamps in SRT files
	PreserveLineBreak bool     `json:"preserveLineBreak"` // Whether to preserve line breaks
}

// New creates a new clean text module
func New() modules.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "clean_text"
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

	// If we have a specific filename, validation is sufficient
	if p.InputFileName != "" {
		return nil
	}

	// Only validate file existence for external input paths
	fileInfo, err := os.Stat(p.Input)
	if err != nil {
		// For files that don't exist, check if they might be in the output directory
		// as they could be created by previous steps
		if strings.Contains(p.Input, p.Output) ||
			strings.Contains(p.Input, "${output}") ||
			strings.Contains(p.Input, "./output") ||
			strings.Contains(p.Input, "/output") ||
			strings.Contains(p.Input, "/transcript") ||
			filepath.Base(p.Input) == "transcript.srt" ||
			filepath.Base(p.Input) == "transcript.txt" {
			// Skip validation for expected output files
			utils.LogVerbose("Note: Input file %s will be created by a previous step", p.Input)
			return nil
		}
		return fmt.Errorf("input file does not exist: %w", err)
	}

	// Check if it's a directory but no inputFileName is provided
	if fileInfo.IsDir() && p.InputFileName == "" {
		return fmt.Errorf("input is a directory but no inputFileName specified")
	}

	return nil
}

// Execute cleans text files
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Set default values
	if p.CleanFileSuffix == "" {
		p.CleanFileSuffix = "_clean"
	}
	if !p.PreserveLineBreak {
		p.PreserveLineBreak = true // Default to preserving line breaks
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Verify input exists at execution time
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("input file not found: %w", err)
	}

	if fileInfo.IsDir() {
		return modules.ModuleResult{}, fmt.Errorf("input must be a file, not a directory: %s", resolvedInput)
	}

	// Check if the file is a text file
	if !utils.IsTextFile(resolvedInput) {
		return modules.ModuleResult{}, fmt.Errorf("file %s appears to be binary, not a text file", resolvedInput)
	}

	// Determine output filename
	var outputBaseName string
	if p.OutputFileName != "" {
		outputBaseName = p.OutputFileName
	} else {
		filename := filepath.Base(resolvedInput)
		outputBaseName = filename[:len(filename)-len(filepath.Ext(filename))]
	}

	outputPath := filepath.Join(p.Output, outputBaseName+p.CleanFileSuffix+".txt")

	if err := m.cleanFile(resolvedInput, outputPath, p); err != nil {
		return modules.ModuleResult{}, err
	}

	utils.LogSuccess("Cleaned %s -> %s", resolvedInput, outputPath)

	// Create result with output file information
	result := modules.ModuleResult{
		Outputs: map[string]string{
			"cleaned": outputPath,
		},
		Metadata: map[string]interface{}{
			"inputFile":       resolvedInput,
			"outputFormat":    "txt",
			"cleanFileSuffix": p.CleanFileSuffix,
		},
	}

	return result, nil
}

// cleanFile cleans a single text file
func (m *Module) cleanFile(inputPath, outputPath string, p Params) error {
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
	defer func() {
		if err := inputFile.Close(); err != nil {
			utils.LogWarning("Failed to close input file: %v", err)
		}
	}()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if err := outputFile.Close(); err != nil {
			utils.LogWarning("Failed to close output file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)
	defer func() {
		if err := writer.Flush(); err != nil {
			utils.LogWarning("Failed to flush writer: %v", err)
		}
	}()

	utils.LogVerbose("Cleaning file: %s", inputPath)

	// Process based on file extension
	fileExt := strings.ToLower(filepath.Ext(inputPath))
	if fileExt == ".srt" {
		// Special handling for SRT files
		if err := m.cleanSRTFile(scanner, writer, removeRegexes, timestampRegex, p.PreserveTimestamp); err != nil {
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

			// Remove timestamps if not preserved
			if !p.PreserveTimestamp {
				line = timestampRegex.ReplaceAllString(line, "")
			}

			// Write the cleaned line
			if line != "" || p.PreserveLineBreak {
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

// SRTState represents the state of SRT file processing
type SRTState int

const (
	stateWaitingNumber SRTState = iota
	stateWaitingTimestamp
	stateCollectingText
)

const (
	maxLineLength    = 4096 // Maximum allowed length for a single line
	maxBlockSize     = 100  // Maximum allowed lines in a subtitle block
	maxSubtitleCount = 9999 // Maximum allowed subtitle numbers
)

// SRTBlock represents a subtitle block
type SRTBlock struct {
	number    int
	timestamp string
	text      []string
}

// cleanSRTFile cleans an SRT format subtitle file with security and performance optimizations
func (m *Module) cleanSRTFile(scanner *bufio.Scanner, writer *bufio.Writer, removeRegexes []*regexp.Regexp, timestampRegex *regexp.Regexp, preserveTimestamp bool) error {
	// Set maximum line length to prevent memory exhaustion
	scanner.Buffer(make([]byte, maxLineLength), maxLineLength)

	var number = 0

	// writeBlock writes the current block if it has valid content
	writeBlock := func(block *SRTBlock) error {
		if block == nil || len(block.text) == 0 {
			return nil
		}

		// Write subtitle number
		if _, err := fmt.Fprintf(writer, "%d\n", block.number); err != nil {
			return fmt.Errorf("failed to write subtitle number: %w", err)
		}

		// Write timestamp if preserving
		if preserveTimestamp && block.timestamp != "" {
			if _, err := fmt.Fprintf(writer, "%s\n", block.timestamp); err != nil {
				return fmt.Errorf("failed to write timestamp: %w", err)
			}
		}

		// Write text lines
		for _, line := range block.text {
			// Apply removal patterns to each line
			cleanedLine := line
			for _, re := range removeRegexes {
				cleanedLine = re.ReplaceAllString(cleanedLine, "")
			}

			// Remove parenthetical timestamps if not preserving timestamps
			if !preserveTimestamp {
				cleanedLine = timestampRegex.ReplaceAllString(cleanedLine, "")
			}

			// Only trim left side to preserve trailing spaces
			cleanedLine = strings.TrimLeftFunc(cleanedLine, unicode.IsSpace)

			if cleanedLine != "" {
				if _, err := fmt.Fprintf(writer, "%s\n", cleanedLine); err != nil {
					return fmt.Errorf("failed to write text line: %w", err)
				}
			}
		}

		// Write block separator
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write block separator: %w", err)
		}

		return nil
	}

	// validateLine checks if a line meets security requirements
	validateLine := func(line string) error {
		if len(line) > maxLineLength {
			return fmt.Errorf("line exceeds maximum length of %d characters", maxLineLength)
		}
		return nil
	}

	// Process each line
	var currentBlock *SRTBlock
	state := stateWaitingNumber

	for scanner.Scan() {
		line := scanner.Text()
		if err := validateLine(line); err != nil {
			return err
		}

		trimmedLine := strings.TrimSpace(line)

		// Handle empty lines
		if trimmedLine == "" {
			if currentBlock != nil {
				if err := writeBlock(currentBlock); err != nil {
					return err
				}
				currentBlock = nil
			}
			state = stateWaitingNumber
			continue
		}

		switch state {
		case stateWaitingNumber:
			if isSubtitleNumber(trimmedLine) {
				number++
				currentBlock = &SRTBlock{
					number: number,
					text:   make([]string, 0, 4),
				}
				state = stateWaitingTimestamp
			}

		case stateWaitingTimestamp:
			if isTimestamp(trimmedLine) {
				if preserveTimestamp {
					currentBlock.timestamp = line
				}
				state = stateCollectingText
			} else {
				// If we're not preserving timestamps and find non-timestamp text,
				// treat it as subtitle text
				currentBlock.text = append(currentBlock.text, line)
				state = stateCollectingText
			}

		case stateCollectingText:
			currentBlock.text = append(currentBlock.text, line)
		}
	}

	// Process the final block if any
	if currentBlock != nil {
		if err := writeBlock(currentBlock); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	return nil
}

// isSubtitleNumber checks if a line is likely a subtitle number
func isSubtitleNumber(line string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(line))
	return err == nil
}

// isTimestamp checks if a line matches the SRT timestamp format
func isTimestamp(line string) bool {
	timestampPattern := `^\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}$`
	matched, _ := regexp.MatchString(timestampPattern, strings.TrimSpace(line))
	return matched
}
