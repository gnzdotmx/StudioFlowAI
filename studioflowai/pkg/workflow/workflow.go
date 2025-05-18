package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/config"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/addtext"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/chatgpt"
	extract "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/extract_audio"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/extractshorts"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/format"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/split"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/transcribe"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/youtube"
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
	registry    *modules.ModuleRegistry
	inputConfig *config.InputConfig
}

// NewWorkflow creates a new workflow with the default module registry
func NewWorkflow(registry *modules.ModuleRegistry, inputConfig *config.InputConfig) *Workflow {
	return &Workflow{
		registry:    registry,
		inputConfig: inputConfig,
	}
}

// LoadFromFile loads a workflow definition from a YAML file
func LoadFromFile(inputConfig *config.InputConfig) (*Workflow, error) {
	// Create a registry with all available modules
	registry := modules.NewModuleRegistry()

	// Register all available modules
	registerModules(registry)

	// Create a new workflow with the registry
	workflow := NewWorkflow(registry, inputConfig)

	// Read the YAML file
	data, err := os.ReadFile(inputConfig.WorkflowPath)
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

// ModuleIO defines the expected inputs and outputs for a module
type ModuleIO struct {
	// InputPatterns defines patterns to match input files
	InputPatterns []string
	// OutputPatterns defines patterns to match output files
	OutputPatterns []string
}

// ModuleIORegistry maps module names to their expected file patterns
var ModuleIORegistry = map[string]ModuleIO{
	"extract": {
		InputPatterns:  []string{"*.mp4", "*.mov", "*.avi", "*.mkv", "*.webm"},
		OutputPatterns: []string{"*.wav"},
	},
	"transcribe": {
		InputPatterns:  []string{"*.wav"},
		OutputPatterns: []string{"*.srt"},
	},
	"format": {
		InputPatterns:  []string{"*.srt"},
		OutputPatterns: []string{"*.txt"},
	},
	"chatgpt": {
		InputPatterns:  []string{"*.txt"},
		OutputPatterns: []string{"*.txt"},
	},
	"sns": {
		InputPatterns:  []string{"*.txt"},
		OutputPatterns: []string{"*.txt"},
	},
	"shorts": {
		InputPatterns:  []string{"*.txt"},
		OutputPatterns: []string{"*.yaml"},
	},
	"extractshorts": {
		InputPatterns:  []string{"*.yaml", "*.mp4", "*.mov", "*.avi", "*.mkv", "*.webm"},
		OutputPatterns: []string{"shorts/*"},
	},
	"addtext": {
		InputPatterns:  []string{"*.yaml", "*.mp4", "*.mov", "*.avi", "*.mkv", "*.webm"},
		OutputPatterns: []string{"shorts_with_text/*"},
	},
	"uploadyoutubeshorts": {
		InputPatterns:  []string{"*.yaml", "*.mp4"},
		OutputPatterns: []string{"*.json"}, // For storing upload status and metadata
	},
}

// matchFilePattern checks if a file matches any of the given patterns
func matchFilePattern(file string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(file))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// findMatchingFile looks for a file matching the patterns in the given directory
func findMatchingFile(dir string, patterns []string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if matchFilePattern(file.Name(), patterns) {
			return filepath.Join(dir, file.Name()), nil
		}
	}
	return "", fmt.Errorf("no matching file found for patterns %v", patterns)
}

// ValidateBeforeRun performs a complete validation including modules
func (w *Workflow) ValidateBeforeRun() error {
	// First check basic structure
	if err := w.ValidateStructure(); err != nil {
		return err
	}

	// Set input and output paths from inputConfig if provided
	if w.inputConfig != nil {
		if w.inputConfig.InputPath != "" {
			w.Input = w.inputConfig.InputPath
		}
		if w.inputConfig.OutputPath != "" {
			w.Output = w.inputConfig.OutputPath
		}
	}

	// Validate the input for the first step if global input is specified
	if w.Input != "" {
		// Validate that input is a file, not a directory
		fileInfo, err := os.Stat(w.Input)
		if err != nil {
			return &utils.ValidationError{
				Field:   "input",
				Message: "input file does not exist",
				Err:     err,
			}
		}
		if fileInfo.IsDir() {
			return &utils.ValidationError{
				Field:   "input",
				Message: "input must be a file, not a directory",
			}
		}

		// Only validate video file extension for video-related modules
		if len(w.Steps) > 0 {
			firstModule := w.Steps[0].Module
			if firstModule == "extract" || firstModule == "extractshorts" || firstModule == "addtext" {
				ext := strings.ToLower(filepath.Ext(w.Input))
				validVideoExts := map[string]bool{
					".mp4":  true,
					".mov":  true,
					".avi":  true,
					".mkv":  true,
					".webm": true,
				}
				if !validVideoExts[ext] {
					return &utils.ValidationError{
						Field:   "input",
						Message: "input file must be a video file (supported formats: mp4, mov, avi, mkv, webm)",
					}
				}
			}
		}
	}

	// Determine the base output directory from the input file
	var baseOutputDir string
	if w.Input != "" {
		// Use the input file's directory
		baseOutputDir = filepath.Join(filepath.Dir(w.Input), "output")
	} else if len(w.Steps) > 0 {
		// Use the first step's input directory
		if input, ok := w.Steps[0].Parameters["input"].(string); ok {
			baseOutputDir = filepath.Join(filepath.Dir(input), "output")
		} else {
			return &utils.ValidationError{
				Field:   "output",
				Message: "could not determine output directory: no input file specified",
			}
		}
	} else {
		return &utils.ValidationError{
			Field:   "output",
			Message: "could not determine output directory: no steps defined",
		}
	}

	// Create a timestamp-based subfolder for this run
	timestamp := time.Now().Format("20060102-150405")
	runName := fmt.Sprintf("%s-%s", strings.ReplaceAll(w.Name, " ", "_"), timestamp)
	outputDir := filepath.Join(baseOutputDir, runName)

	// Set the output directory
	w.Output = outputDir

	// Track available outputs for chain validation
	availableOutputs := make(map[string]bool)
	if w.Input != "" {
		availableOutputs["video"] = true
	}

	// Validate each step
	for i, step := range w.Steps {
		if step.Module == "" {
			return &utils.ValidationError{
				Field:   fmt.Sprintf("step_%d", i+1),
				Message: "module name is required",
			}
		}

		// Verify the module exists
		module, err := w.registry.Get(step.Module)
		if err != nil {
			return &utils.ValidationError{
				Field:   fmt.Sprintf("step_%d", i+1),
				Message: fmt.Sprintf("module %s not found", step.Module),
				Err:     err,
			}
		}

		// Get module IO requirements
		moduleIO, ok := ModuleIORegistry[step.Module]
		if !ok {
			return &utils.ValidationError{
				Field:   fmt.Sprintf("step_%d", i+1),
				Message: fmt.Sprintf("unknown module type %s", step.Module),
			}
		}

		// Add global input/output if not specified in step params
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			params[k] = v
		}

		// For first step, always use the CLI input if provided
		if i == 0 {
			if w.Input != "" {
				if step.Module == "extractshorts" || step.Module == "addtext" {
					params["videoFile"] = w.Input
					params["input"] = step.Parameters["input"].(string)
					utils.LogDebug("[ValidateBeforeRun] Using CLI input as videoFile: %s", w.Input)
					utils.LogDebug("[ValidateBeforeRun] Using CLI input as input: %s", step.Parameters["input"])
				} else {
					params["input"] = w.Input
					utils.LogDebug("Using CLI input file: %s", w.Input)
				}
			}
		} else {
			// For subsequent steps, try to find matching input files from previous outputs
			// Get required inputs from module configuration
			if inputs, ok := params["inputs"].([]interface{}); ok {
				for _, input := range inputs {
					inputStr, ok := input.(string)
					if !ok {
						continue
					}
					if !availableOutputs[inputStr] {
						// Try to find a matching file in the output directory
						if matchingFile, err := findMatchingFile(w.Output, moduleIO.InputPatterns); err == nil {
							params["input"] = matchingFile
							utils.LogDebug("Found matching input file: %s", matchingFile)
							availableOutputs[inputStr] = true
						} else {
							return &utils.ValidationError{
								Field:   fmt.Sprintf("step_%d", i+1),
								Message: fmt.Sprintf("required input %s not available from previous steps", inputStr),
							}
						}
					}
				}
			}
		}

		// Special handling for video-related modules
		if step.Module == "extractshorts" || step.Module == "addtext" {
			// For extractshorts, require videoFile
			if step.Module == "extractshorts" {
				if w.Input != "" {
					params["videoFile"] = w.Input
					utils.LogDebug("Using CLI input as videoFile: %s", w.Input)
				} else if step.Parameters["videoFile"] != nil {
					params["videoFile"] = step.Parameters["videoFile"].(string)
					utils.LogDebug("Using videoFile from step parameters: %s", step.Parameters["videoFile"])
				} else {
					return &utils.ValidationError{
						Field:   fmt.Sprintf("step_%d", i+1),
						Message: "videoFile is required for extractshorts module",
					}
				}
			}
			// For addtext, only require videoFile if it's not after extractshorts
			if step.Module == "addtext" {
				// Check if we have shorts output from previous step
				if !availableOutputs["shorts/"] {
					// Only require videoFile if we don't have shorts output
					if w.Input != "" {
						params["videoFile"] = w.Input
						utils.LogDebug("Using CLI input as videoFile: %s", w.Input)
					} else if step.Parameters["videoFile"] != nil {
						params["videoFile"] = step.Parameters["videoFile"].(string)
						utils.LogDebug("Using videoFile from step parameters: %s", step.Parameters["videoFile"])
					}
				}
			}
		}

		// Always set the output parameter
		params["output"] = outputDir

		// Validate module parameters
		if err := module.Validate(params); err != nil {
			return &utils.ValidationError{
				Field:   fmt.Sprintf("step_%d", i+1),
				Message: fmt.Sprintf("invalid parameters for module %s", step.Module),
				Err:     err,
			}
		}

		// Add this step's outputs to available outputs
		if outputs, ok := params["outputs"].([]interface{}); ok {
			for _, output := range outputs {
				if outputStr, ok := output.(string); ok {
					availableOutputs[outputStr] = true
				}
			}
		}
	}

	return nil
}

// ValidateForRetry performs validation for retry operations, skipping input file checks
func (w *Workflow) ValidateForRetry(workflowName string) error {
	// Check basic structure
	if err := w.ValidateStructure(); err != nil {
		return err
	}

	// Set input and output paths from inputConfig if provided
	if w.inputConfig != nil {
		if w.inputConfig.InputPath != "" {
			w.Input = w.inputConfig.InputPath
		}
		if w.inputConfig.OutputPath != "" {
			w.Output = w.inputConfig.OutputPath
		}
	}

	// Initialize validate flag before the loop
	validate := false
	foundStep := false

	// Validate each step's module existence
	for i, step := range w.Steps {
		if step.Module == "" {
			return fmt.Errorf("module name is required for step %d", i+1)
		}

		// Verify the module exists
		module, err := w.registry.Get(step.Module)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		// Set validate flag when we find the target step
		if step.Name == workflowName {
			validate = true
			foundStep = true
		}

		// Validate module parameters
		if validate {
			// Create a new params map to avoid modifying the original
			params := make(map[string]interface{})
			for k, v := range step.Parameters {
				params[k] = v
			}

			// Handle input file for video-related modules
			if step.Module == "extractshorts" || step.Module == "addtext" {
				if w.Input != "" {
					params["videoFile"] = w.Input
					utils.LogDebug("Using CLI input as videoFile: %s", w.Input)
				}
			}

			// Always set the output parameter
			if w.Output != "" {
				params["output"] = w.Output
				utils.LogDebug("Using output directory: %s", w.Output)
			}

			// Handle ${output} variable substitution
			for k, v := range params {
				if str, ok := v.(string); ok {
					if strings.Contains(str, "${output}") {
						params[k] = strings.ReplaceAll(str, "${output}", w.Output)
					}
				}
			}

			if err := module.Validate(params); err != nil {
				return fmt.Errorf("invalid parameters for step %d (%s): %w", i+1, step.Module, err)
			}
		}
	}

	if !foundStep {
		return fmt.Errorf("no step found with name '%s'", workflowName)
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

	// Ensure output directory exists
	if err := os.MkdirAll(w.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	utils.LogDebug("Results will be stored in: %s", w.Output)

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
			utils.LogError("Failed to get module for step %s: %v", stepName, err)
			return fmt.Errorf("failed to get module for step %s: %v", stepName, err)
		}

		// Add input and output paths to parameters if not already specified
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			// Handle any special parameter substitutions
			if str, ok := v.(string); ok {
				// Handle variable substitution
				// Replace ${output} with the actual output directory
				if strings.Contains(str, "${output}") {
					v = strings.ReplaceAll(str, "${output}", w.Output)
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
			}
			params[k] = v
		}

		// For first step, always use the CLI input if provided
		if i == 0 {
			if w.Input != "" {
				params["input"] = w.Input
				utils.LogDebug("Using CLI input file: %s", w.Input)
			}
		}

		// Set videoFile parameter for modules that need it
		if step.Module == "extractshorts" || step.Module == "addtext" {
			// Only set videoFile from CLI input if it's not already specified in parameters
			if w.Input != "" {
				params["videoFile"] = w.Input
				utils.LogDebug("Using CLI input as videoFile: %s", w.Input)
			}
		}

		// Always set the output parameter
		params["output"] = w.Output

		// Execute the module
		if err := module.Execute(ctx, params); err != nil {
			utils.LogError("Failed to execute step %s: %v", stepName, err)
			return fmt.Errorf("failed to execute step %s: %v", stepName, err)
		}

		utils.LogSuccess("Completed %s", stepName)
	}

	utils.LogSuccess("Workflow completed: %s", w.Name)
	utils.LogDebug("Results stored in: %s", w.Output)
	return nil
}

// ExecuteRetry runs the workflow continuing from a previous failed execution
func (w *Workflow) ExecuteRetry(outputFolderPath, workflowName string) error {
	// Validate the workflow with retry-specific validation
	if err := w.ValidateForRetry(workflowName); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	utils.LogInfo("Retrying workflow: %s", w.Name)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	// Use the provided output folder
	utils.LogDebug("Using existing results directory: %s", w.Output)

	// Find the step that matches the specified workflowName
	startStep := 0
	stepFound := false

	for i, step := range w.Steps {
		if step.Name == workflowName {
			startStep = i
			stepFound = true
			break
		}
	}

	if !stepFound {
		return fmt.Errorf("no step found with name '%s'", workflowName)
	}

	utils.LogInfo("Resuming from step %d: %s", startStep+1, w.Steps[startStep].Name)

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
			utils.LogError("Failed to get module for step %s: %v", stepName, err)
			return fmt.Errorf("failed to get module for step %s: %v", stepName, err)
		}

		// Add input and output paths to parameters if not already specified
		params := make(map[string]interface{})
		for k, v := range step.Parameters {
			// Handle any special parameter substitutions
			if str, ok := v.(string); ok {
				// Handle variable substitution
				// Replace ${output} with the actual output directory
				if strings.Contains(str, "${output}") {
					v = strings.ReplaceAll(str, "${output}", w.Output)
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
				// Handle legacy path formats too
				if str == "./output" {
					v = w.Output
				} else if strings.HasPrefix(str, "./output/") {
					// Replace "./output/filename.ext" with "outputDir/filename.ext"
					v = filepath.Join(w.Output, strings.TrimPrefix(str, "./output/"))
					utils.LogDebug("Resolved path %s to %s", str, v)
				}
			}
			params[k] = v
		}

		// Handle input file based on module type and step position
		if i == startStep {
			// For first step in retry, use the original input file if available
			if w.Input != "" {
				params["input"] = w.Input
				utils.LogDebug("Using original input file: %s", w.Input)
			}
		} else {
			// For subsequent steps, check if we need to use a specific file from output
			if step.Module == "addtext" || step.Module == "extractshorts" {
				// Get the input filename from the workflow configuration
				if input, ok := step.Parameters["input"].(string); ok {
					// Extract just the filename from the input path
					inputFilename := filepath.Base(input)
					// Construct the full path in the output directory
					params["input"] = filepath.Join(w.Output, inputFilename)
					utils.LogDebug("Using input file from workflow: %s", params["input"])
				}
			}
		}

		// Set videoFile parameter for modules that need it
		if step.Module == "extractshorts" || step.Module == "addtext" {
			if w.Input != "" {
				params["videoFile"] = w.Input
				utils.LogDebug("Using original input as videoFile: %s", w.Input)
			}
		}

		// Always set the output parameter
		params["output"] = w.Output

		// Execute the module
		if err := module.Execute(ctx, params); err != nil {
			utils.LogError("Failed to execute step %s: %v", stepName, err)
			return fmt.Errorf("failed to execute step %s: %v", stepName, err)
		}

		utils.LogDebug("Completed %s", stepName)
	}

	utils.LogSuccess("Workflow completed after retry: %s", w.Name)
	utils.LogDebug("Results stored in: %s", w.Output)
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
	registry.Register(chatgpt.NewShorts())
	registry.Register(extractshorts.New())
	registry.Register(addtext.New())
	registry.Register(youtube.NewUploadYouTubeShorts())
}

// SetInputPath overrides the input path defined in the workflow
func (w *Workflow) SetInputPath(path string) {
	// Verify that the input is a file, not a directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		utils.LogError("Input file does not exist: %s", path)
		return
	}
	if fileInfo.IsDir() {
		utils.LogError("Input must be a file, not a directory: %s", path)
		return
	}
	w.Input = path
	utils.LogInfo("Using input file from CLI: %s", path)
}

// SetOutputPath overrides the output path defined in the workflow
func (w *Workflow) SetOutputPath(path string) {
	// Verify that the output is a directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		utils.LogError("Output directory does not exist: %s", path)
		return
	}
	if !fileInfo.IsDir() {
		utils.LogError("Output must be a directory, not a file: %s", path)
		return
	}
	w.Output = path
	utils.LogInfo("Using output directory from CLI: %s", path)
}

// ExecuteSingleModule executes a single module from the workflow
func (w *Workflow) ExecuteSingleModule(moduleName string) error {
	// Find the module in the workflow
	var targetStep *Step
	for _, step := range w.Steps {
		if step.Module == moduleName {
			targetStep = &step
			break
		}
	}

	if targetStep == nil {
		return fmt.Errorf("module '%s' not found in workflow", moduleName)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	// Get the module
	module, err := w.registry.Get(targetStep.Module)
	if err != nil {
		return fmt.Errorf("failed to get module: %w", err)
	}

	// Prepare parameters
	params := make(map[string]interface{})
	for k, v := range targetStep.Parameters {
		params[k] = v
	}

	// Handle input file for video-related modules
	if targetStep.Module == "extractshorts" || targetStep.Module == "addtext" {
		if w.Input != "" {
			params["videoFile"] = w.Input
			utils.LogDebug("Using CLI input as videoFile: %s", w.Input)
		}
	}

	// Set output directory
	if w.Output != "" {
		params["output"] = w.Output
		utils.LogDebug("Using output directory: %s", w.Output)
	}

	// Handle ${output} variable substitution
	for k, v := range params {
		if str, ok := v.(string); ok {
			if strings.Contains(str, "${output}") {
				params[k] = strings.ReplaceAll(str, "${output}", w.Output)
			}
		}
	}

	// Validate parameters
	if err := module.Validate(params); err != nil {
		return fmt.Errorf("invalid parameters: %w", err)
	}

	// Execute the module
	utils.LogInfo("Executing module: %s", moduleName)
	if err := module.Execute(ctx, params); err != nil {
		return fmt.Errorf("module execution failed: %w", err)
	}

	utils.LogSuccess("Module execution completed: %s", moduleName)
	return nil
}
