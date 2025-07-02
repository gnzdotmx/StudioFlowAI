// Package mod provides the core module functionality for the workflow system
package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// Module defines the interface that all modules must implement
type Module interface {
	// Name returns the module's unique identifier
	Name() string

	// GetIO returns the module's input/output specification
	GetIO() ModuleIO

	// Validate checks if the parameters are valid
	Validate(params map[string]interface{}) error

	// Execute runs the module with the given parameters
	Execute(ctx context.Context, params map[string]interface{}) (ModuleResult, error)
}

// ModuleIO defines the expected inputs and outputs for a module
type ModuleIO struct {
	// Required input files/data from previous modules
	RequiredInputs []ModuleInput
	// Generated outputs that can be used by subsequent modules
	ProducedOutputs []ModuleOutput
	// Optional inputs that can enhance module functionality
	OptionalInputs []ModuleInput
}

// ModuleInput defines an input requirement for a module
type ModuleInput struct {
	Name        string   // Logical name of the input (e.g., "videoFile", "transcription")
	Description string   // Description of what this input is used for
	Patterns    []string // File patterns that match this input
	Type        string   // Type of input (e.g., "file", "directory", "data")
}

// ModuleOutput defines an output produced by a module
type ModuleOutput struct {
	Name        string   // Logical name of the output
	Description string   // Description of what this output contains
	Patterns    []string // File patterns that match this output
	Type        string   // Type of output (e.g., "file", "directory", "data")
}

// ModuleResult contains the results of a module execution
type ModuleResult struct {
	Outputs     map[string]string      // Map of output name to file/directory path
	Metadata    map[string]interface{} // Additional metadata about the execution
	Statistics  map[string]interface{} // Performance and other statistics
	NextModules []string               // Suggested next modules in workflow
}

// InputType defines the valid types of module inputs
type InputType string

const (
	InputTypeFile      InputType = "file"
	InputTypeDirectory InputType = "directory"
	InputTypeData      InputType = "data"
)

// OutputType defines the valid types of module outputs
type OutputType string

const (
	OutputTypeFile      OutputType = "file"
	OutputTypeDirectory OutputType = "directory"
	OutputTypeData      OutputType = "data"
)

// ModuleRegistry stores all available modules
type ModuleRegistry struct {
	modules      map[string]Module
	sync.RWMutex // Add thread safety
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
	}
}

// ValidateIO validates a module's I/O specification
func ValidateIO(io ModuleIO) error {
	// Validate required inputs
	for i, input := range io.RequiredInputs {
		if input.Name == "" {
			return fmt.Errorf("required input %d has empty name", i)
		}
		if input.Type == "" {
			return fmt.Errorf("required input %s has empty type", input.Name)
		}
		if !isValidInputType(input.Type) {
			return fmt.Errorf("required input %s has invalid type: %s", input.Name, input.Type)
		}
	}

	// Validate optional inputs
	for i, input := range io.OptionalInputs {
		if input.Name == "" {
			return fmt.Errorf("optional input %d has empty name", i)
		}
		if input.Type == "" {
			return fmt.Errorf("optional input %s has empty type", input.Name)
		}
		if !isValidInputType(input.Type) {
			return fmt.Errorf("optional input %s has invalid type: %s", input.Name, input.Type)
		}
	}

	// Validate produced outputs
	for i, output := range io.ProducedOutputs {
		if output.Name == "" {
			return fmt.Errorf("output %d has empty name", i)
		}
		if output.Type == "" {
			return fmt.Errorf("output %s has empty type", output.Name)
		}
		if !isValidOutputType(output.Type) {
			return fmt.Errorf("output %s has invalid type: %s", output.Name, output.Type)
		}
		if len(output.Patterns) == 0 {
			return fmt.Errorf("output %s has no patterns defined", output.Name)
		}
	}

	return nil
}

// isValidInputType checks if the input type is valid
func isValidInputType(t string) bool {
	switch InputType(t) {
	case InputTypeFile, InputTypeDirectory, InputTypeData:
		return true
	default:
		return false
	}
}

// isValidOutputType checks if the output type is valid
func isValidOutputType(t string) bool {
	switch OutputType(t) {
	case OutputTypeFile, OutputTypeDirectory, OutputTypeData:
		return true
	default:
		return false
	}
}

// Register adds a module to the registry
func (r *ModuleRegistry) Register(m Module) error {
	if m == nil {
		return fmt.Errorf("cannot register nil module")
	}

	name := m.Name()
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	// Validate module I/O specification
	if err := ValidateIO(m.GetIO()); err != nil {
		return fmt.Errorf("invalid I/O specification for module %s: %w", name, err)
	}

	r.Lock()
	defer r.Unlock()

	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("module %s is already registered", name)
	}

	r.modules[name] = m
	return nil
}

// Get retrieves a module by name
func (r *ModuleRegistry) Get(name string) (Module, error) {
	if name == "" {
		return nil, fmt.Errorf("module name cannot be empty")
	}

	r.RLock()
	defer r.RUnlock()

	module, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("module %s not found", name)
	}
	return module, nil
}

// ParseParams converts generic parameter map to a specific struct for each module
func ParseParams(params map[string]interface{}, target interface{}) error {
	if params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	// Validate that target is a pointer
	if reflect.ValueOf(target).Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	// Validate that target points to a struct
	if reflect.ValueOf(target).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshaling params: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("error unmarshaling params: %w", err)
	}

	return nil
}

// ListModules returns a slice of all registered modules
func (r *ModuleRegistry) ListModules() []Module {
	r.RLock()
	defer r.RUnlock()

	modules := make([]Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}
