package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecLookPath allows us to mock exec.LookPath in tests
var ExecLookPath = exec.LookPath

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateInputPath validates an input path, handling both files and directories
func ValidateInputPath(input, output string, inputFileName string) error {
	if input == "" {
		return &ValidationError{
			Field:   "input",
			Message: "input path is required",
		}
	}

	// Skip validation for files that will be created in the output directory
	if strings.Contains(input, output) || strings.Contains(input, "output") {
		return nil
	}

	// If we have a specific filename, only validate the directory exists
	if inputFileName != "" {
		dir := filepath.Dir(input)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return &ValidationError{
				Field:   "input",
				Message: fmt.Sprintf("input directory does not exist: %s", dir),
				Err:     err,
			}
		}
		return nil
	}

	// Validate the input path exists
	fileInfo, err := os.Stat(input)
	if err != nil {
		return &ValidationError{
			Field:   "input",
			Message: "input path does not exist",
			Err:     err,
		}
	}

	// If it's a directory, ensure inputFileName is provided
	if fileInfo.IsDir() && inputFileName == "" {
		return &ValidationError{
			Field:   "input",
			Message: "input is a directory but no inputFileName specified",
		}
	}

	return nil
}

// ValidateOutputPath validates an output path
func ValidateOutputPath(output string) error {
	if output == "" {
		return &ValidationError{
			Field:   "output",
			Message: "output path is required",
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(output, 0755); err != nil {
		return &ValidationError{
			Field:   "output",
			Message: "failed to create output directory",
			Err:     err,
		}
	}

	return nil
}

// ValidateVideoFile validates a video file path and checks for FFmpeg
func ValidateVideoFile(videoFile string) error {
	if videoFile == "" {
		return &ValidationError{
			Field:   "video",
			Message: "video file path is required",
		}
	}

	// Verify the video file exists
	if _, err := os.Stat(videoFile); os.IsNotExist(err) {
		return &ValidationError{
			Field:   "video",
			Message: fmt.Sprintf("video file does not exist: %s", videoFile),
			Err:     err,
		}
	}

	// Check if FFmpeg is installed
	if _, err := ExecLookPath("ffmpeg"); err != nil {
		return &ValidationError{
			Field:   "ffmpeg",
			Message: "ffmpeg not found in PATH",
			Err:     err,
		}
	}

	return nil
}

// ResolveOutputPath resolves ${output} variable in paths
func ResolveOutputPath(path, outputDir string) string {
	if strings.Contains(path, "${output}") {
		return strings.ReplaceAll(path, "${output}", outputDir)
	}
	return path
}

// ValidateRequiredDependency checks if a required command is available
func ValidateRequiredDependency(cmd string) error {
	if _, err := ExecLookPath(cmd); err != nil {
		return &ValidationError{
			Field:   cmd,
			Message: fmt.Sprintf("%s not found in PATH", cmd),
			Err:     err,
		}
	}
	return nil
}

// ValidateFileExtension checks if a file has one of the allowed extensions
func ValidateFileExtension(filePath string, allowedExts []string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			return nil
		}
	}
	return &ValidationError{
		Field:   "extension",
		Message: fmt.Sprintf("file extension %s not allowed. Allowed extensions: %v", ext, allowedExts),
	}
}

// ValidateTimestampFormat checks if a string matches the HH:MM:SS format
func ValidateTimestampFormat(timestamp string) error {
	parts := strings.Split(timestamp, ":")
	if len(parts) != 3 {
		return &ValidationError{
			Field:   "timestamp",
			Message: fmt.Sprintf("invalid timestamp format: %s (expected HH:MM:SS)", timestamp),
		}
	}

	// Validate each part
	for i, part := range parts {
		if len(part) != 2 {
			return &ValidationError{
				Field:   "timestamp",
				Message: fmt.Sprintf("invalid timestamp part %d: %s (expected 2 digits)", i+1, part),
			}
		}
		if part[0] < '0' || part[0] > '9' || part[1] < '0' || part[1] > '9' {
			return &ValidationError{
				Field:   "timestamp",
				Message: fmt.Sprintf("invalid timestamp part %d: %s (expected digits)", i+1, part),
			}
		}
	}

	return nil
}
