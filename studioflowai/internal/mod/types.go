// Package mod provides the core module functionality for the workflow system
package mod

// ModuleVersion represents semantic versioning for modules
type ModuleVersion struct {
	Major    int
	Minor    int
	Patch    int
	Metadata string
}

// ModuleDependency represents a dependency between modules
type ModuleDependency struct {
	Name            string
	RequiredVersion ModuleVersion
	Optional        bool
}

// ValidationResult represents the result of module validation
type ValidationResult struct {
	IsValid     bool
	Errors      []error
	Warnings    []string
	Suggestions []string
}

// IOSchema represents the schema for module IO validation
type IOSchema struct {
	Type       string
	Properties map[string]interface{}
	Required   bool
	Validator  func(interface{}) error
}
