package validator

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// ExternalTool represents an external command-line tool requirement
type ExternalTool struct {
	Name        string
	VersionArgs []string
	Validate    func(output string) bool
}

// requiredTools is a list of external tools that must be installed
var requiredTools = []ExternalTool{
	{
		Name:        "ffmpeg",
		VersionArgs: []string{"-version"},
		Validate: func(output string) bool {
			return strings.Contains(output, "ffmpeg version")
		},
	},
	// Add other required tools as needed
}

// optionalTools lists tools that are checked but not required
var optionalTools = []ExternalTool{
	{
		Name:        "whisper",
		VersionArgs: []string{"--help"},
		Validate: func(output string) bool {
			return strings.Contains(output, "usage") || strings.Contains(output, "Usage") || strings.Contains(output, "options")
		},
	},
}

// requiredEnvVars lists required environment variables
var requiredEnvVars = []string{
	"OPENAI_API_KEY",
}

// ValidateExternalTools checks if all required external tools are installed
func ValidateExternalTools() error {
	for _, tool := range requiredTools {
		// Check if the tool exists
		path, err := exec.LookPath(tool.Name)
		if err != nil {
			return fmt.Errorf("tool %s not found in PATH: %w", tool.Name, err)
		}

		// Check the version
		cmd := exec.Command(path, tool.VersionArgs...)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to run %s: %w", tool.Name, err)
		}

		// Validate the output
		if !tool.Validate(string(output)) {
			return fmt.Errorf("invalid version of %s detected", tool.Name)
		}

		utils.LogVerbose("✓ %s found at %s", tool.Name, path)
	}

	// Check optional tools
	for _, tool := range optionalTools {
		path, err := exec.LookPath(tool.Name)
		if err != nil {
			utils.LogVerbose("ℹ️ Optional tool %s not found: %v", tool.Name, err)
			continue
		}

		// Check the version
		cmd := exec.Command(path, tool.VersionArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			utils.LogVerbose("ℹ️ Optional tool %s found but couldn't verify version: %v", tool.Name, err)
			continue
		}

		// Validate the output
		if !tool.Validate(string(output)) {
			utils.LogVerbose("ℹ️ Optional tool %s found but may not be the correct version", tool.Name)
			continue
		}

		utils.LogVerbose("✓ Optional tool %s found at %s", tool.Name, path)
	}

	return nil
}

// ValidateEnvVars checks if all required environment variables are set
func ValidateEnvVars() error {
	for _, envVar := range requiredEnvVars {
		value := os.Getenv(envVar)
		if value == "" {
			return fmt.Errorf("environment variable %s not set", envVar)
		}

		// Don't print the actual value for security
		utils.LogVerbose("✓ %s is set", envVar)
	}

	return nil
}
