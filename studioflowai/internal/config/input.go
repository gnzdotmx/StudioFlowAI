package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InputConfig holds the configuration for input files and directories
type InputConfig struct {
	InputPath     string
	OutputPath    string
	WorkflowPath  string
	RetryMode     bool
	WorkflowName  string
	InputFileName string
	InputFileType string
	InputFileExt  string
}

// NewInputConfig creates a new input configuration
func NewInputConfig(inputPath, outputPath, workflowPath string, retryMode bool, workflowName string) (*InputConfig, error) {
	config := &InputConfig{
		InputPath:    inputPath,
		OutputPath:   outputPath,
		WorkflowPath: workflowPath,
		RetryMode:    retryMode,
		WorkflowName: workflowName,
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// validate performs comprehensive validation of the input configuration
func (c *InputConfig) validate() error {
	// Validate workflow path
	if c.WorkflowPath == "" {
		return fmt.Errorf("workflow path is required")
	}
	if _, err := os.Stat(c.WorkflowPath); os.IsNotExist(err) {
		return fmt.Errorf("workflow file does not exist: %s", c.WorkflowPath)
	}

	// Validate input path if provided
	if c.InputPath != "" {
		fileInfo, err := os.Stat(c.InputPath)
		if err != nil {
			return fmt.Errorf("input path does not exist: %w", err)
		}
		if fileInfo.IsDir() {
			return fmt.Errorf("input must be a file, not a directory: %s", c.InputPath)
		}
		c.InputFileName = filepath.Base(c.InputPath)
		c.InputFileExt = strings.ToLower(filepath.Ext(c.InputPath))
	}

	// Validate output path
	if c.OutputPath != "" {
		fileInfo, err := os.Stat(c.OutputPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to access output path: %w", err)
			}
			// Create output directory if it doesn't exist
			if err := os.MkdirAll(c.OutputPath, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		} else if !fileInfo.IsDir() {
			return fmt.Errorf("output must be a directory, not a file: %s", c.OutputPath)
		}
	}

	// Validate retry mode requirements
	if c.RetryMode {
		if c.OutputPath == "" {
			return fmt.Errorf("output path is required when using retry mode")
		}
		if c.WorkflowName == "" {
			return fmt.Errorf("workflow name is required when using retry mode")
		}
	}

	return nil
}

// IsValidVideoFile checks if the input file is a valid video file
func (c *InputConfig) IsValidVideoFile() bool {
	validVideoExts := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
	}
	return validVideoExts[c.InputFileExt]
}

// IsValidAudioFile checks if the input file is a valid audio file
func (c *InputConfig) IsValidAudioFile() bool {
	validAudioExts := map[string]bool{
		".wav": true,
		".mp3": true,
		".m4a": true,
		".aac": true,
	}
	return validAudioExts[c.InputFileExt]
}

// GetInputType returns the type of input file
func (c *InputConfig) GetInputType() string {
	if c.IsValidVideoFile() {
		return "video"
	}
	if c.IsValidAudioFile() {
		return "audio"
	}
	return "unknown"
}
