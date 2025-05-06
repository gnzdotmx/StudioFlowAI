package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/chatgpt"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/extract"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/format"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/split"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/transcribe"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"

	"gopkg.in/yaml.v3"
)

// Step represents a single processing step in a workflow
type Step struct {
	Name       string                 `yaml:"name"`
	Module     string                 `yaml:"module"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// Workflow represents a complete video processing workflow
type Workflow struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Input       string `yaml:"input,omitempty"`
	Output      string `yaml:"output"`
	Steps       []Step `yaml:"steps"`

	// Registry holds all available modules
	registry *modules.ModuleRegistry
}

// NewWorkflow creates a new workflow with the default module registry
func NewWorkflow(registry *modules.ModuleRegistry) *Workflow {
	return &Workflow{
		registry: registry,
	}
}

// LoadFromFile loads a workflow definition from a YAML file
func LoadFromFile(path string) (*Workflow, error) {
	// Create a registry with all available modules
	registry := modules.NewModuleRegistry()

	// Register all available modules
	registerModules(registry)

	// Create a new workflow with the registry
	workflow := NewWorkflow(registry)

	// Read the YAML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Parse the YAML
	if err := yaml.Unmarshal(data, workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate basic workflow structure but don't validate modules yet
	if err := workflow.ValidateStructure(); err != nil {
		return nil, fmt.Errorf("invalid workflow configuration: %w", err)
	}

	return workflow, nil
}

// ValidateStructure checks if the basic workflow structure is valid (without validating modules)
func (w *Workflow) ValidateStructure() error {
	if w.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(w.Steps) == 0 {
		return fmt.Errorf("at least one processing step is required")
	}

	return nil
}

// ValidateBeforeRun performs a complete validation including modules
func (w *Workflow) ValidateBeforeRun() error {
	// First check basic structure
	if err := w.ValidateStructure(); err != nil {
		return err
	}

	// Then check output path
	if w.Output == "" {
		return fmt.Errorf("output path is required")
	}

	// Validate the input for the first step if global input is specified
	if w.Input != "" {
		// Validate that input is a file, not a directory
		fileInfo, err := os.Stat(w.Input)
		if err != nil {
			return fmt.Errorf("input file does not exist: %w", err)
		}
		if fileInfo.IsDir() {
			return fmt.Errorf("input must be a file, not a directory")
		}
	}

	// Validate each step
	for i, step := range w.Steps {
		if step.Module == "" {
			return fmt.Errorf("module name is required for step %d", i+1)
		}

		// Verify the module exists
		module, err := w.registry.Get(step.Module)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		// Add global input/output if not specified in step params
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			// Handle any special parameter substitutions during validation
			if str, ok := v.(string); ok {
				// Handle paths that reference the output directory
				if strings.Contains(str, "./output/") || str == "./output" {
					// Mark these paths as being in the output directory
					// Individual modules will handle validation appropriately
					fmt.Printf("Note: Step %d (%s) uses output from previous steps\n", i+1, step.Name)
				}
			}
			params[k] = v
		}

		// Add input parameter if needed - but only for first step if global input is specified
		if _, ok := params["input"]; !ok && (i == 0 && w.Input != "") {
			params["input"] = w.Input
		}

		if _, ok := params["output"]; !ok {
			params["output"] = w.Output
		}

		// Special case handling for modules that need to combine input directory and filename
		// Similar to what we do in Execute(), handle directory+inputFileName combo during validation
		if inputDir, ok := params["input"].(string); ok {
			// Check if the input is a directory and inputFileName is set
			inputFileInfo, err := os.Stat(inputDir)
			// During validation, the output directory might not exist yet
			// Skip the check if the directory doesn't exist
			if err == nil && inputFileInfo.IsDir() {
				if inputFileName, ok := params["inputFileName"].(string); ok {
					// Construct the full file path by joining the directory and filename
					params["input"] = filepath.Join(inputDir, inputFileName)
					// During validation, this file might not exist yet, which is OK
				}
			}
		}

		// For first step, ensure it has input
		if i == 0 {
			if _, ok := params["input"]; !ok {
				return fmt.Errorf("first step must specify input parameter when global input is not provided")
			}
		}

		// Validate module parameters
		if err := module.Validate(params); err != nil {
			return fmt.Errorf("invalid parameters for step %d (%s): %w", i+1, step.Module, err)
		}
	}

	return nil
}

// ValidateForRetry performs validation for retry operations, skipping input file checks
func (w *Workflow) ValidateForRetry() error {
	// Check basic structure
	if err := w.ValidateStructure(); err != nil {
		return err
	}

	// For retry operations, we don't validate input file existence
	// since we'll be using intermediate files from the previous run

	if w.Output == "" {
		return fmt.Errorf("output path is required")
	}

	// Validate each step's module existence
	for i, step := range w.Steps {
		if step.Module == "" {
			return fmt.Errorf("module name is required for step %d", i+1)
		}

		// Verify the module exists
		_, err := w.registry.Get(step.Module)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
	}

	return nil
}

// Execute runs the workflow
func (w *Workflow) Execute() error {
	// First validate the workflow completely
	if err := w.ValidateBeforeRun(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	utils.LogInfo("Starting workflow: %s", w.Name)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	// Create a timestamp-based subfolder for this run
	timestamp := time.Now().Format("20060102-150405")
	runName := fmt.Sprintf("%s-%s", strings.ReplaceAll(w.Name, " ", "_"), timestamp)
	outputDir := filepath.Join(w.Output, runName)

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	utils.LogDebug("Results will be stored in: %s", outputDir)

	// Execute each step in sequence
	for i, step := range w.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		utils.LogInfo("Executing %s (module: %s)", stepName, step.Module)

		// Get the module
		module, err := w.registry.Get(step.Module)
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to get module for step %s: %v", stepName, err)
			utils.LogError(errorMessage)
			return fmt.Errorf("%s", errorMessage)
		}

		// Add input and output paths to parameters if not already specified
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			// Handle any special parameter substitutions
			if str, ok := v.(string); ok {
				// Handle variable substitution
				// Replace ${output} with the actual output directory
				if strings.Contains(str, "${output}") {
					v = strings.ReplaceAll(str, "${output}", outputDir)
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
				// Handle legacy path formats too
				if str == "./output" {
					v = outputDir
				} else if strings.HasPrefix(str, "./output/") {
					// Replace "./output/filename.ext" with "outputDir/filename.ext"
					v = filepath.Join(outputDir, strings.TrimPrefix(str, "./output/"))
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
			}
			params[k] = v
		}

		// Add input parameter if needed - but only for first step if global input is specified
		if _, ok := params["input"]; !ok {
			if i == 0 && w.Input != "" {
				// First step - use global input if provided
				params["input"] = w.Input
			} else {
				// For subsequent steps, default to the output directory if input not explicitly specified
				params["input"] = outputDir
			}
		}

		// Special case handling for modules that need to combine input directory and filename
		if inputDir, ok := params["input"].(string); ok {
			// Check if the input is a directory and inputFileName is set
			inputFileInfo, err := os.Stat(inputDir)
			if err == nil && inputFileInfo.IsDir() {
				if inputFileName, ok := params["inputFileName"].(string); ok {
					// Construct the full file path by joining the directory and filename
					params["input"] = filepath.Join(inputDir, inputFileName)
					utils.LogVerbose("Using input file: %s", params["input"])
				}
			}
		}

		if _, ok := params["output"]; !ok {
			params["output"] = outputDir
		}

		// Execute the module
		if err := module.Execute(ctx, params); err != nil {
			errorMessage := fmt.Sprintf("Failed to execute step %s: %v", stepName, err)
			utils.LogError(errorMessage)
			return fmt.Errorf("%s", errorMessage)
		}

		utils.LogSuccess("Completed %s", stepName)
	}

	utils.LogSuccess("Workflow completed: %s", w.Name)
	utils.LogDebug("Results stored in: %s", outputDir)
	return nil
}

// registerModules registers all available modules with the registry
func registerModules(registry *modules.ModuleRegistry) {
	// Register modules
	registry.Register(extract.New())
	registry.Register(split.New())
	registry.Register(transcribe.New())
	registry.Register(format.New())
	registry.Register(chatgpt.New())
	registry.Register(chatgpt.NewSNS())
}

// SetInputPath overrides the input path defined in the workflow
func (w *Workflow) SetInputPath(path string) {
	// Verify that the input is a file, not a directory
	fileInfo, err := os.Stat(path)
	if err == nil && fileInfo.IsDir() {
		fmt.Printf("%s\n", utils.Error("Input must be a file, not a directory. Please specify a file path."))
		return
	}
	w.Input = path
}

// ExecuteRetry runs the workflow continuing from a previous failed execution
func (w *Workflow) ExecuteRetry(outputFolderPath, workflowName string) error {
	// Validate the workflow with retry-specific validation
	if err := w.ValidateForRetry(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	utils.LogInfo("Retrying workflow: %s", w.Name)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	// Use the provided output folder instead of creating a new one
	outputDir := outputFolderPath
	utils.LogDebug("Using existing results directory: %s", outputDir)

	// Determine the last successful step by examining the output folder
	// This is a simple heuristic - a more robust solution would be to store progress in a state file
	lastSuccessfulStep := -1

	// Check if each step has produced output files
	for i, step := range w.Steps {
		// Based on the step module, check for expected output files
		// This is a simple check that will work for the workflow in the example
		switch step.Module {
		case "extract":
			// Check if audio file exists
			audioPath := filepath.Join(outputDir, "audio.wav")
			if _, err := os.Stat(audioPath); err == nil {
				lastSuccessfulStep = i
			}
		case "transcribe":
			// Check if transcript file exists
			transcriptPath := filepath.Join(outputDir, "transcript.srt")
			if _, err := os.Stat(transcriptPath); err == nil {
				lastSuccessfulStep = i
			}
		case "format":
			// Check if formatted transcript exists
			formattedPath := filepath.Join(outputDir, "transcript_clean.txt")
			if _, err := os.Stat(formattedPath); err == nil {
				lastSuccessfulStep = i
			}
		case "chatgpt":
			// The prompt template was missing, so chatgpt step likely failed
			// Check if corrected transcript exists
			correctedPath := filepath.Join(outputDir, "transcript_corrected.txt")
			if _, err := os.Stat(correctedPath); err == nil {
				lastSuccessfulStep = i
			}
		case "sns":
			// Check if social media content exists
			snsPath := filepath.Join(outputDir, "social_media_content.txt")
			if _, err := os.Stat(snsPath); err == nil {
				lastSuccessfulStep = i
			}
		}
	}

	// If we couldn't determine the last successful step, start from the beginning
	startStep := lastSuccessfulStep + 1
	if startStep >= len(w.Steps) {
		utils.LogWarning("All steps appear to be complete, starting from the beginning")
		startStep = 0
	} else {
		utils.LogInfo("Resuming from step %d: %s", startStep+1, w.Steps[startStep].Name)
	}

	// Execute only the remaining steps
	for i := startStep; i < len(w.Steps); i++ {
		step := w.Steps[i]
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		utils.LogInfo("Executing %s (module: %s)", stepName, step.Module)

		// Get the module
		module, err := w.registry.Get(step.Module)
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to get module for step %s: %v", stepName, err)
			utils.LogError(errorMessage)
			return fmt.Errorf("%s", errorMessage)
		}

		// Add input and output paths to parameters if not already specified
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			// Handle any special parameter substitutions
			if str, ok := v.(string); ok {
				// Handle variable substitution
				// Replace ${output} with the actual output directory
				if strings.Contains(str, "${output}") {
					v = strings.ReplaceAll(str, "${output}", outputDir)
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
				// Handle legacy path formats too
				if str == "./output" {
					v = outputDir
				} else if strings.HasPrefix(str, "./output/") {
					// Replace "./output/filename.ext" with "outputDir/filename.ext"
					v = filepath.Join(outputDir, strings.TrimPrefix(str, "./output/"))
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
			}
			params[k] = v
		}

		// Add input parameter if needed - but only for first step if global input is specified
		if _, ok := params["input"]; !ok {
			if i == startStep && i == 0 && w.Input != "" {
				// First step in retry - use global input if provided
				params["input"] = w.Input
			} else {
				// For subsequent steps, default to output directory
				params["input"] = outputDir
			}
		}

		// Special case handling for modules that need to combine input directory and filename
		if inputDir, ok := params["input"].(string); ok {
			// Check if the input is a directory and inputFileName is set
			inputFileInfo, err := os.Stat(inputDir)
			if err == nil && inputFileInfo.IsDir() {
				if inputFileName, ok := params["inputFileName"].(string); ok {
					// Construct the full file path by joining the directory and filename
					params["input"] = filepath.Join(inputDir, inputFileName)
					utils.LogVerbose("Using input file: %s", params["input"])
				}
			}
		}

		if _, ok := params["output"]; !ok {
			params["output"] = outputDir
		}

		// Execute the module
		if err := module.Execute(ctx, params); err != nil {
			errorMessage := fmt.Sprintf("Failed to execute step %s: %v", stepName, err)
			utils.LogError(errorMessage)
			return fmt.Errorf("%s", errorMessage)
		}

		utils.LogSuccess("Completed %s", stepName)
	}

	utils.LogSuccess("Workflow completed after retry: %s", w.Name)
	utils.LogDebug("Results stored in: %s", outputDir)
	return nil
}
