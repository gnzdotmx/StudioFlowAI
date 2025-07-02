// Package workflow provides functionality for managing video processing workflows
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/config"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	cleantext "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/clean_text"
	correcttranscript "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/correct_transcript"
	extractaudio "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/extract_audio"
	extractshorts "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/extractshorts"
	settitle2shortvideo "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/settitle2shortvideo"
	suggestshorts "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/suggest_shorts"
	suggestsnscontent "github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/suggest_sns_content"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/tiktok"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/transcribe"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules/youtube"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Supported video extensions
var videoExtensions = []string{
	".mp4", ".mov", ".avi", ".mkv", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".3gp",
}

// isVideoFile checks if a file has a video extension
func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, videoExt := range videoExtensions {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// ExecuteWithState runs the workflow using the new graph-based execution engine
func (w *Workflow) ExecuteWithState() (*WorkflowState, error) {
	// Create new workflow state
	state := &WorkflowState{
		ID:           uuid.New().String(),
		Name:         w.Name,
		StartTime:    time.Now(),
		Status:       WorkflowStatusRunning,
		GlobalInputs: make(map[string]string),
		History:      make([]WorkflowEvent, 0),
	}

	// Build workflow graph
	graph := NewWorkflowGraph()
	state.Graph = graph

	// Add nodes for each step
	nodeMap := make(map[string]*WorkflowNode)
	for _, step := range w.Steps {
		node := graph.AddNode(step)
		nodeMap[step.Name] = node
	}

	// Add edges based on module dependencies
	if err := w.buildDependencyEdges(graph, nodeMap); err != nil {
		return state, err
	}

	// Get execution order
	order, err := graph.TopologicalSort()
	if err != nil {
		return state, fmt.Errorf("failed to determine execution order: %w", err)
	}

	// Keep track of module outputs
	moduleOutputs := make(map[string]map[string]string)

	// Execute nodes in order
	for i, nodeID := range order {
		node := graph.Nodes[nodeID]

		// Check for checkpoint
		if checkpoint := w.GetCheckpoint(nodeID); checkpoint != nil {
			// Restore state from checkpoint
			state = checkpoint.State
			node = state.Graph.Nodes[nodeID]
			utils.LogInfo("Restored checkpoint for node %s (retry %d)", nodeID, checkpoint.RetryCount)
		}

		// Update state
		state.CurrentNode = nodeID
		node.Status = NodeStatusRunning

		// Record event
		state.AddEvent(WorkflowEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			NodeID:    nodeID,
			Type:      "started",
			Message:   fmt.Sprintf("Started executing %s", node.Step.Name),
		})

		// Execute the module
		module, err := w.registry.Get(node.Step.Module)
		if err != nil {
			node.Status = NodeStatusFailed
			state.Status = WorkflowStatusFailed

			// Save checkpoint for retry
			w.SaveCheckpoint(nodeID, state)

			return state, fmt.Errorf("failed to get module %s: %w", node.Step.Module, err)
		}

		// Prepare parameters with input/output paths
		params := make(map[string]interface{})
		for k, v := range node.Step.Parameters {
			// Handle string parameters that might contain ${output}
			if strVal, ok := v.(string); ok {
				if strings.Contains(strVal, "${output}") {
					// Replace ${output} with actual output path
					resolvedPath := strings.ReplaceAll(strVal, "${output}", w.Output)
					params[k] = resolvedPath
				} else {
					// Only add ./ prefix for input/output paths, not for command names
					if k == "input" || k == "output" || strings.HasSuffix(k, "Path") || strings.HasSuffix(k, "File") || strings.HasSuffix(k, "Dir") {
						if !filepath.IsAbs(strVal) && !strings.HasPrefix(strVal, "./") {
							params[k] = "./" + strVal
						} else {
							params[k] = strVal
						}
					} else {
						params[k] = strVal
					}
				}
			} else {
				params[k] = v
			}
		}

		// Handle input parameter based on step position
		if i == 0 {
			// First step: use global input if provided, otherwise keep input from parameters
			if w.Input != "" {
				params["input"] = w.Input
				state.GlobalInputs["input"] = w.Input
			}
		} else if _, hasInput := params["input"]; hasInput {
			// Get the module's input requirements
			currentModule, err := w.registry.Get(node.Step.Module)
			if err != nil {
				continue
			}
			currentIO := currentModule.GetIO()

			// Find the input patterns we're looking for
			var expectedPatterns []string
			for _, input := range currentIO.RequiredInputs {
				if input.Name == "input" {
					expectedPatterns = input.Patterns
					break
				}
			}

			// Check if input is explicitly configured with ${output}
			if strInput, ok := params["input"].(string); ok {
				if strings.Contains(strInput, "${output}") {
					goto inputFound
				}
			}

			// Only try to find matching outputs if we have patterns to match against
			if len(expectedPatterns) > 0 {
				for j := i - 1; j >= 0; j-- {
					prevNode := graph.Nodes[order[j]]

					if outputs, ok := moduleOutputs[prevNode.ID]; ok {
						// Try to find a matching output based on file patterns
						for _, outputPath := range outputs {
							// Only use the output if it matches one of our expected patterns
							for _, expectedPattern := range expectedPatterns {
								if strings.HasSuffix(outputPath, expectedPattern) {
									utils.LogInfo("Step %s: Processing: %s", node.Step.Name, outputPath)
									params["input"] = outputPath
									goto inputFound
								}
							}
						}
					}
				}
			}
		inputFound:
		}

		// Set output directory
		params["output"] = w.Output

		// Execute the module
		result, err := module.Execute(context.Background(), params)
		if err != nil {
			node.Status = NodeStatusFailed
			state.Status = WorkflowStatusFailed

			// Save checkpoint for retry
			w.SaveCheckpoint(nodeID, state)

			// Record failure event
			state.AddEvent(WorkflowEvent{
				ID:        uuid.New().String(),
				Timestamp: time.Now(),
				NodeID:    nodeID,
				Type:      "failed",
				Message:   fmt.Sprintf("Failed executing %s: %v", node.Step.Name, err),
				Data: map[string]interface{}{
					"error": err.Error(),
				},
			})

			return state, fmt.Errorf("failed to execute module %s: %w", node.Step.Module, err)
		}

		// Store module outputs for dependency resolution
		moduleOutputs[nodeID] = result.Outputs

		// Update node with results
		node.Status = NodeStatusComplete
		node.Outputs = result.Outputs
		node.Metadata = result.Metadata

		// Clear checkpoint on success
		w.ClearCheckpoint(nodeID)

		// Record success event
		state.AddEvent(WorkflowEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			NodeID:    nodeID,
			Type:      "completed",
			Message:   fmt.Sprintf("Completed executing %s", node.Step.Name),
			Data:      result.Statistics,
		})
	}

	// Update final state
	state.Status = WorkflowStatusComplete
	state.EndTime = time.Now()

	return state, nil
}

// buildDependencyEdges adds edges to the graph based on module dependencies
func (w *Workflow) buildDependencyEdges(graph *WorkflowGraph, nodeMap map[string]*WorkflowNode) error {
	// First, add edges to enforce sequential order from YAML file
	for i := 1; i < len(w.Steps); i++ {
		prevStep := w.Steps[i-1]
		currStep := w.Steps[i]
		if err := graph.AddEdge(nodeMap[prevStep.Name].ID, nodeMap[currStep.Name].ID); err != nil {
			return fmt.Errorf("failed to add sequential edge: %w", err)
		}
	}

	// Then add edges based on module dependencies
	for i, step := range w.Steps {
		module, err := w.registry.Get(step.Module)
		if err != nil {
			return fmt.Errorf("failed to get module %s: %w", step.Module, err)
		}

		moduleIO := module.GetIO()

		// For each required input, find which previous step produces it
		for _, input := range moduleIO.RequiredInputs {
			// Skip if input is provided in parameters
			if _, hasParam := step.Parameters[input.Name]; hasParam {
				continue
			}

			// Skip if it's the first step and global input is provided
			if i == 0 && w.Input != "" {
				continue
			}

			// Look for a matching output from previous steps
			for _, prevStep := range w.Steps {
				if prevStep.Name == step.Name {
					break // Don't look at steps after current one
				}

				prevModule, err := w.registry.Get(prevStep.Module)
				if err != nil {
					continue
				}

				prevIO := prevModule.GetIO()
				for _, output := range prevIO.ProducedOutputs {
					if matchesIOPattern(input, output) {
						if err := graph.AddEdge(nodeMap[prevStep.Name].ID, nodeMap[step.Name].ID); err != nil {
							return fmt.Errorf("failed to add dependency edge: %w", err)
						}
						break
					}
				}
			}
		}
	}

	return nil
}

// matchesIOPattern checks if an input matches an output's pattern
func matchesIOPattern(input mod.ModuleInput, output mod.ModuleOutput) bool {
	// Check if types match
	if input.Type != output.Type {
		return false
	}

	// Check if any patterns match
	for _, inPattern := range input.Patterns {
		for _, outPattern := range output.Patterns {
			if inPattern == outPattern {
				return true
			}
		}
	}

	return false
}

// SaveWorkflowState saves the workflow state to a file
func (w *Workflow) SaveWorkflowState(state *WorkflowState, outputPath string) error {
	// Create state summary
	summary := map[string]interface{}{
		"id":          state.ID,
		"name":        state.Name,
		"status":      state.Status,
		"startTime":   state.StartTime,
		"endTime":     state.EndTime,
		"currentNode": state.CurrentNode,
		"nodes":       make(map[string]interface{}),
	}

	// Add node information
	for id, node := range state.Graph.Nodes {
		nodeSummary := map[string]interface{}{
			"name":     node.Step.Name,
			"module":   node.Step.Module,
			"status":   node.Status,
			"inputs":   node.Inputs,
			"outputs":  node.Outputs,
			"metadata": node.Metadata,
		}
		summary["nodes"].(map[string]interface{})[id] = nodeSummary
	}

	// Convert to YAML
	data, err := yaml.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow state: %w", err)
	}

	return nil
}

// LoadWorkflowState loads a workflow state from a file
func (w *Workflow) LoadWorkflowState(inputPath string) (*WorkflowState, error) {
	// Read file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow state: %w", err)
	}

	// Parse YAML
	var summary map[string]interface{}
	if err := yaml.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse workflow state: %w", err)
	}

	// Create new state
	state := &WorkflowState{
		ID:           summary["id"].(string),
		Name:         summary["name"].(string),
		Status:       WorkflowStatus(summary["status"].(string)),
		StartTime:    summary["startTime"].(time.Time),
		GlobalInputs: make(map[string]string),
		History:      make([]WorkflowEvent, 0),
	}

	if endTime, ok := summary["endTime"].(time.Time); ok {
		state.EndTime = endTime
	}

	// Create graph
	graph := NewWorkflowGraph()
	state.Graph = graph

	// Restore nodes
	if nodes, ok := summary["nodes"].(map[string]interface{}); ok {
		for id, nodeData := range nodes {
			nodeMap := nodeData.(map[string]interface{})

			step := Step{
				Name:       nodeMap["name"].(string),
				Module:     nodeMap["module"].(string),
				Parameters: make(map[string]interface{}),
			}

			node := &WorkflowNode{
				ID:       id,
				Step:     step,
				Status:   NodeStatus(nodeMap["status"].(string)),
				Inputs:   make(map[string]string),
				Outputs:  make(map[string]string),
				Metadata: make(map[string]interface{}),
			}

			if inputs, ok := nodeMap["inputs"].(map[string]interface{}); ok {
				for k, v := range inputs {
					node.Inputs[k] = v.(string)
				}
			}

			if outputs, ok := nodeMap["outputs"].(map[string]interface{}); ok {
				for k, v := range outputs {
					node.Outputs[k] = v.(string)
				}
			}

			if metadata, ok := nodeMap["metadata"].(map[string]interface{}); ok {
				node.Metadata = metadata
			}

			graph.Nodes[id] = node
		}
	}

	return state, nil
}

// LoadFromFile loads a workflow from a YAML file
func LoadFromFile(inputConfig *config.InputConfig) (*Workflow, error) {
	// Read workflow file
	data, err := os.ReadFile(inputConfig.WorkflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Parse YAML
	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow file: %w", err)
	}

	// Initialize workflow
	workflow.inputConfig = inputConfig
	workflow.registry = mod.NewModuleRegistry()
	workflow.checkpoints = make(map[string]*WorkflowCheckpoint)

	// Register available modules
	if err := registerModules(workflow.registry); err != nil {
		return nil, fmt.Errorf("failed to register modules: %w", err)
	}

	// Map of module parameters that require video input
	videoInputParams := map[string][]string{
		"extractaudio":             {"input"},
		"extract_shorts":           {"videoFile"},
		"set_title_to_short_video": {"videoFile"},
	}

	// Set input path - prefer command line flag over workflow file
	inputPath := inputConfig.InputPath
	if inputPath != "" {
		// If the input path is not absolute, add ./ prefix
		if !filepath.IsAbs(inputPath) && !strings.HasPrefix(inputPath, "./") {
			inputPath = "./" + inputPath
		}

		// Only apply to video parameters if the input is a video file
		if isVideoFile(inputPath) {
			// Update all steps that require video input
			for i, step := range workflow.Steps {
				if paramNames, requiresVideo := videoInputParams[step.Module]; requiresVideo {
					// Initialize parameters map if nil
					if workflow.Steps[i].Parameters == nil {
						workflow.Steps[i].Parameters = make(map[string]interface{})
					}

					// Set video path for each required parameter
					for _, paramName := range paramNames {
						workflow.Steps[i].Parameters[paramName] = inputPath
						utils.LogVerbose("Setting %s.%s to %s", step.Module, paramName, inputPath)
					}
				}
			}
		} else {
			utils.LogVerbose("Input file %s is not a video - video parameters will not be updated", inputPath)
		}

		workflow.Input = inputPath
	} else if len(workflow.Steps) > 0 {
		// If no command line input, try to get it from the first step's parameters
		if inputParam, ok := workflow.Steps[0].Parameters["input"].(string); ok {
			// If the input path is absolute, use it as is
			if filepath.IsAbs(inputParam) {
				workflow.Input = inputParam
			} else {
				// For relative paths, add ./ prefix if not present
				if !strings.HasPrefix(inputParam, "./") {
					workflow.Input = "./" + inputParam
				} else {
					workflow.Input = inputParam
				}
			}
		}
	}

	// Set output path
	workflow.Output = inputConfig.OutputPath

	return &workflow, nil
}

// registerModules registers all available modules with the registry
func registerModules(registry *mod.ModuleRegistry) error {
	// Upload modules (these implement the correct interface)
	if err := registry.Register(extractaudio.New()); err != nil {
		utils.LogError("Failed to register extractaudio module: %v", err)
	}
	if err := registry.Register(transcribe.New()); err != nil {
		utils.LogError("Failed to register transcribe module: %v", err)
	}
	if err := registry.Register(cleantext.New()); err != nil {
		utils.LogError("Failed to register cleantext module: %v", err)
	}
	if err := registry.Register(correcttranscript.New()); err != nil {
		utils.LogError("Failed to register correcttranscript module: %v", err)
	}
	if err := registry.Register(suggestsnscontent.New()); err != nil {
		utils.LogError("Failed to register suggestsnscontent module: %v", err)
	}
	if err := registry.Register(extractshorts.New()); err != nil {
		utils.LogError("Failed to register extractshorts module: %v", err)
	}
	if err := registry.Register(suggestshorts.New()); err != nil {
		utils.LogError("Failed to register suggestshorts module: %v", err)
	}
	if err := registry.Register(settitle2shortvideo.New()); err != nil {
		utils.LogError("Failed to register settitle2shortvideo module: %v", err)
	}
	if err := registry.Register(youtube.New()); err != nil {
		utils.LogError("Failed to register youtube module: %v", err)
	}
	if err := registry.Register(tiktok.NewUploadTikTokShorts()); err != nil {
		utils.LogError("Failed to register tiktok module: %v", err)
	}

	return nil
}

// ExecuteRetry resumes a failed workflow execution from the last checkpoint
func (w *Workflow) ExecuteRetry(outputPath, workflowName string) error {
	// Find the specified step in the workflow
	var startStepIndex = -1
	for i, step := range w.Steps {
		if step.Name == workflowName {
			startStepIndex = i
			break
		}
	}

	if startStepIndex == -1 {
		return fmt.Errorf("workflow step '%s' not found in workflow", workflowName)
	}

	// Process all paths in workflow steps
	for i := range w.Steps {
		for k, v := range w.Steps[i].Parameters {
			if strVal, ok := v.(string); ok {
				// Handle ${output} placeholder and escaped spaces
				if strings.Contains(strVal, "${output}") {
					strVal = strings.ReplaceAll(strVal, "${output}", outputPath)
				}
				if strings.Contains(strVal, "\\ ") {
					strVal = strings.ReplaceAll(strVal, "\\ ", " ")
				}
				w.Steps[i].Parameters[k] = strVal
			}
		}
	}

	// Create a subset of steps starting from the specified step
	w.Steps = w.Steps[startStepIndex:]

	// Sanitize workflow name for file system
	sanitizedName := strings.ReplaceAll(w.Name, " ", "_")

	// Try different possible state file paths
	possiblePaths := []string{
		filepath.Join(outputPath, sanitizedName+".state.yaml"),
		filepath.Join(outputPath, w.Name+".state.yaml"),
		filepath.Join(outputPath, "Complete_Video_Processing_Workflow.state.yaml"), // Default workflow name
	}

	var prevState *WorkflowState
	var loadErr error

	// Try each possible path
	for _, statePath := range possiblePaths {
		prevState, loadErr = w.LoadWorkflowState(statePath)
		if loadErr == nil {
			break
		}
	}

	// If no state file found, create a new one starting from the specified step
	if loadErr != nil {
		utils.LogInfo("No previous state found. Creating new workflow state starting from step: %s", workflowName)

		// Create new workflow state
		prevState = &WorkflowState{
			ID:           uuid.New().String(),
			Name:         w.Name,
			StartTime:    time.Now(),
			Status:       WorkflowStatusRunning,
			GlobalInputs: make(map[string]string),
			History:      make([]WorkflowEvent, 0),
		}

		// Build workflow graph
		graph := NewWorkflowGraph()
		prevState.Graph = graph

		// Add nodes for each step
		nodeMap := make(map[string]*WorkflowNode)
		for _, step := range w.Steps {
			// Create a copy of the step
			nodeCopy := Step{
				Name:       step.Name,
				Module:     step.Module,
				Parameters: make(map[string]interface{}),
			}

			// Copy and process parameters
			for k, v := range step.Parameters {
				nodeCopy.Parameters[k] = v
			}

			node := graph.AddNode(nodeCopy)
			nodeMap[step.Name] = node
		}

		// Add edges based on module dependencies
		if err := w.buildDependencyEdges(graph, nodeMap); err != nil {
			return fmt.Errorf("failed to build workflow graph: %w", err)
		}

		// For the starting step, try to use its configured input if no override is provided
		if w.Input == "" {
			if inputParam, ok := w.Steps[0].Parameters["input"].(string); ok {
				w.Input = inputParam
				utils.LogInfo("Using configured input from step: %s", w.Input)
			}
		}

		// Set input file as global input if provided
		if w.Input != "" {
			prevState.GlobalInputs["input"] = w.Input
		}
	}

	// Restore workflow state
	w.Name = prevState.Name
	w.checkpoints = make(map[string]*WorkflowCheckpoint)

	// If we created a new state, mark nodes as pending
	if loadErr != nil {
		for id, node := range prevState.Graph.Nodes {
			node.Status = NodeStatusPending
			w.SaveCheckpoint(id, prevState)
		}
	} else {
		// For existing state, only checkpoint failed nodes
		for id, node := range prevState.Graph.Nodes {
			if node.Status == NodeStatusFailed {
				w.SaveCheckpoint(id, prevState)
				break
			}
		}
	}

	// Execute from specified step or last failed node
	newState, err := w.ExecuteWithState()
	if err != nil {
		return err
	}

	// Save final state
	if err := w.SaveWorkflowState(newState, filepath.Join(outputPath, sanitizedName+".state.yaml")); err != nil {
		return fmt.Errorf("failed to save workflow state: %w", err)
	}

	return nil
}

// Execute runs the workflow and returns any error
func (w *Workflow) Execute() error {
	state, err := w.ExecuteWithState()
	if err != nil {
		return err
	}

	// Sanitize workflow name for file system
	sanitizedName := strings.ReplaceAll(w.Name, " ", "_")

	// Save final state
	statePath := filepath.Join(w.Output, sanitizedName+".state.yaml")
	if err := w.SaveWorkflowState(state, statePath); err != nil {
		return fmt.Errorf("failed to save workflow state: %w", err)
	}

	return nil
}
