package modules

import (
	"context"
	"encoding/json"
	"fmt"
)

// Module represents a processing step in a workflow
type Module interface {
	// Name returns the unique identifier of the module
	Name() string

	// Execute performs the module's operation
	Execute(ctx context.Context, params map[string]interface{}) error

	// Validate checks if the provided parameters are valid for this module
	Validate(params map[string]interface{}) error
}

// ModuleRegistry stores all available modules
type ModuleRegistry struct {
	modules map[string]Module
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
	}
}

// Register adds a module to the registry
func (r *ModuleRegistry) Register(m Module) {
	r.modules[m.Name()] = m
}

// Get retrieves a module by name
func (r *ModuleRegistry) Get(name string) (Module, error) {
	module, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("module %s not found", name)
	}
	return module, nil
}

// ParseParams converts generic parameter map to a specific struct for each module
func ParseParams(params map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshaling params: %w", err)
	}

	return json.Unmarshal(data, target)
}
