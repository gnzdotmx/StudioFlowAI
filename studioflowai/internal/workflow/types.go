// Package workflow provides functionality for managing video processing workflows
package workflow

import (
	"sync"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/config"
	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
)

// Core workflow types

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

	// Checkpoint management
	checkpoints     map[string]*WorkflowCheckpoint
	checkpointMutex sync.RWMutex
}

// Step represents a single processing step in a workflow
type Step struct {
	Name       string                 `yaml:"name"`
	Module     string                 `yaml:"module"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// Graph-related types

// WorkflowGraph represents the directed acyclic graph of workflow steps
type WorkflowGraph struct {
	sync.RWMutex // Protects all fields below
	Nodes        map[string]*WorkflowNode
	Edges        map[string][]string
}

// WorkflowNode represents a single node in the workflow graph
type WorkflowNode struct {
	ID       string
	Step     Step
	Status   NodeStatus
	Inputs   map[string]string
	Outputs  map[string]string
	Metadata map[string]interface{}
}

// State-related types

// WorkflowState represents the current state of a workflow execution
type WorkflowState struct {
	sync.RWMutex // Protects all fields below

	ID            string
	Name          string
	Graph         *WorkflowGraph
	GlobalInputs  map[string]string
	GlobalOutputs map[string]string
	StartTime     time.Time
	EndTime       time.Time
	Status        WorkflowStatus
	CurrentNode   string
	History       []WorkflowEvent
}

// WorkflowEvent represents an event that occurred during workflow execution
type WorkflowEvent struct {
	ID        string
	Timestamp time.Time
	NodeID    string
	Type      string
	Message   string
	Data      map[string]interface{}
}

// WorkflowCheckpoint represents a saved state of workflow execution
type WorkflowCheckpoint struct {
	NodeID     string
	State      *WorkflowState
	Timestamp  time.Time
	RetryCount int
}

// Status types

// NodeStatus represents the current status of a workflow node
type NodeStatus string

const (
	NodeStatusPending  NodeStatus = "pending"
	NodeStatusRunning  NodeStatus = "running"
	NodeStatusComplete NodeStatus = "complete"
	NodeStatusFailed   NodeStatus = "failed"
	NodeStatusSkipped  NodeStatus = "skipped"
)

// WorkflowStatus represents the current status of the workflow
type WorkflowStatus string

const (
	WorkflowStatusPending  WorkflowStatus = "pending"
	WorkflowStatusRunning  WorkflowStatus = "running"
	WorkflowStatusComplete WorkflowStatus = "complete"
	WorkflowStatusFailed   WorkflowStatus = "failed"
)

// Execution types

// ExecutionStrategy defines how workflow execution should be handled
type ExecutionStrategy struct {
	MaxConcurrent    int
	TimeoutPerModule time.Duration
	RetryStrategy    RetryStrategy
}

// RetryStrategy defines how retries should be handled
type RetryStrategy struct {
	MaxAttempts     int
	BackoffDuration time.Duration
	OnRetry         func(error) bool
}
