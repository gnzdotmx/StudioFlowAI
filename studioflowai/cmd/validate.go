package cmd

import (
	"fmt"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/validator"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate environment setup",
	Long:  `Check if all required external tools and configurations are properly set up.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.LogInfo("Validating environment...")

		// Validate external tools (ffmpeg, etc.)
		if err := validator.ValidateExternalTools(); err != nil {
			return fmt.Errorf("external tools validation failed: %w", err)
		}
		utils.LogSuccess("External tools: OK")

		// Validate environment variables for ChatGPT
		if err := validator.ValidateEnvVars(); err != nil {
			return fmt.Errorf("environment variables validation failed: %w", err)
		}
		utils.LogSuccess("Environment variables: OK")

		utils.LogSuccess("Environment validation completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
