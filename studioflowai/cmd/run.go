package cmd

import (
	"fmt"
	"os"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/validator"
	"github.com/gnzdotmx/studioflowai/studioflowai/pkg/workflow"

	"github.com/spf13/cobra"
)

var (
	workflowFilePath  string
	inputFileOverride string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a video processing workflow",
	Long:  `Execute a video processing workflow defined in a YAML file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate that external dependencies are installed
		if err := validator.ValidateExternalTools(); err != nil {
			return fmt.Errorf("dependency validation failed: %w", err)
		}

		// Load the workflow without full validation
		wf, err := workflow.LoadFromFile(workflowFilePath)
		if err != nil {
			return fmt.Errorf("failed to load workflow: %w", err)
		}

		// Override input file if specified
		if inputFileOverride != "" {
			// Verify that input is a file, not a directory
			fileInfo, err := os.Stat(inputFileOverride)
			if err != nil {
				return fmt.Errorf("input file does not exist: %s", inputFileOverride)
			}
			if fileInfo.IsDir() {
				return fmt.Errorf("input must be a file, not a directory: %s", inputFileOverride)
			}

			wf.SetInputPath(inputFileOverride)
			utils.LogInfo("Using input file from CLI: %s", inputFileOverride)
		}

		// Execute the workflow - validation will happen inside Execute
		if err := wf.Execute(); err != nil {
			return fmt.Errorf("workflow execution failed: %w", err)
		}

		utils.LogInfo("Workflow completed successfully")
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&workflowFilePath, "workflow", "w", "", "Path to workflow YAML file (required)")
	runCmd.Flags().StringVarP(&inputFileOverride, "input", "i", "", "Input file path (overrides the one in workflow file)")
	_ = runCmd.MarkFlagRequired("workflow")
	rootCmd.AddCommand(runCmd)
}
