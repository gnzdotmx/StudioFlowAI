package cmd

import (
	"fmt"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/config"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/validator"
	"github.com/gnzdotmx/studioflowai/studioflowai/pkg/workflow"

	"github.com/spf13/cobra"
)

var (
	workflowFilePath  string
	inputFileOverride string
	retryFlag         bool
	outputFolderPath  string
	workflowName      string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a video processing workflow",
	Long:  `Execute a video processing workflow defined in a YAML file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create input configuration
		inputConfig, err := config.NewInputConfig(
			inputFileOverride,
			outputFolderPath,
			workflowFilePath,
			retryFlag,
			workflowName,
		)
		if err != nil {
			return fmt.Errorf("invalid input configuration: %w", err)
		}

		// Validate that external dependencies are installed
		if err := validator.ValidateExternalTools(); err != nil {
			return fmt.Errorf("dependency validation failed: %w", err)
		}

		// Load the workflow without full validation
		wf, err := workflow.LoadFromFile(inputConfig.WorkflowPath)
		if err != nil {
			return fmt.Errorf("failed to load workflow: %w", err)
		}

		// Set input and output paths
		if inputConfig.InputPath != "" {
			wf.SetInputPath(inputConfig.InputPath)
		}
		if inputConfig.OutputPath != "" {
			wf.SetOutputPath(inputConfig.OutputPath)
		}

		// Execute the workflow
		if inputConfig.RetryMode {
			utils.LogInfo("Retrying workflow %s in output folder %s", inputConfig.WorkflowName, inputConfig.OutputPath)
			if err := wf.ExecuteRetry(inputConfig.OutputPath, inputConfig.WorkflowName); err != nil {
				return fmt.Errorf("workflow retry execution failed: %w", err)
			}
		} else {
			if err := wf.Execute(); err != nil {
				return fmt.Errorf("workflow execution failed: %w", err)
			}
		}

		utils.LogInfo("Workflow completed successfully")
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&workflowFilePath, "workflow", "w", "", "Path to workflow YAML file (required)")
	runCmd.Flags().StringVarP(&inputFileOverride, "input", "i", "", "Input file path (overrides the one in workflow file)")
	runCmd.Flags().BoolVarP(&retryFlag, "retry", "r", false, "Retry a failed workflow execution")
	runCmd.Flags().StringVarP(&outputFolderPath, "output-folder", "o", "", "Output folder path with timestamp (required with --retry)")
	runCmd.Flags().StringVarP(&workflowName, "workflow-name", "n", "", "Name of the specific step to resume from (required with --retry)")
	_ = runCmd.MarkFlagRequired("workflow")
	rootCmd.AddCommand(runCmd)
}
