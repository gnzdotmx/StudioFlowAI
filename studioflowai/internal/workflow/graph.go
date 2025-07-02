// Package workflow provides functionality for managing video processing workflows
package workflow

import (
	"fmt"

	"github.com/google/uuid"
)

// NewWorkflowGraph creates a new workflow graph
func NewWorkflowGraph() *WorkflowGraph {
	return &WorkflowGraph{
		Nodes: make(map[string]*WorkflowNode),
		Edges: make(map[string][]string),
	}
}

// AddNode adds a new node to the graph in a thread-safe manner
func (g *WorkflowGraph) AddNode(step Step) *WorkflowNode {
	g.Lock()
	defer g.Unlock()

	node := &WorkflowNode{
		ID:       uuid.New().String(),
		Step:     step,
		Status:   NodeStatusPending,
		Inputs:   make(map[string]string),
		Outputs:  make(map[string]string),
		Metadata: make(map[string]interface{}),
	}
	g.Nodes[node.ID] = node
	return node
}

// AddEdge adds a directed edge between two nodes in a thread-safe manner
func (g *WorkflowGraph) AddEdge(fromID, toID string) error {
	g.Lock()
	defer g.Unlock()

	if _, exists := g.Nodes[fromID]; !exists {
		return fmt.Errorf("source node %s does not exist", fromID)
	}
	if _, exists := g.Nodes[toID]; !exists {
		return fmt.Errorf("destination node %s does not exist", toID)
	}

	if g.Edges[fromID] == nil {
		g.Edges[fromID] = make([]string, 0)
	}
	g.Edges[fromID] = append(g.Edges[fromID], toID)
	return nil
}

// TopologicalSort returns nodes in topological order
func (g *WorkflowGraph) TopologicalSort() ([]string, error) {
	g.RLock()
	defer g.RUnlock()

	visited := make(map[string]bool)
	temp := make(map[string]bool)
	order := make([]string, 0)

	var visit func(string) error
	visit = func(nodeID string) error {
		if temp[nodeID] {
			return fmt.Errorf("cycle detected in workflow graph")
		}
		if visited[nodeID] {
			return nil
		}
		temp[nodeID] = true

		for _, neighbor := range g.Edges[nodeID] {
			if err := visit(neighbor); err != nil {
				return err
			}
		}

		temp[nodeID] = false
		visited[nodeID] = true
		order = append([]string{nodeID}, order...)
		return nil
	}

	for nodeID := range g.Nodes {
		if !visited[nodeID] {
			if err := visit(nodeID); err != nil {
				return nil, err
			}
		}
	}

	return order, nil
}

// GetNodeDependencies returns all nodes that must complete before the given node
func (g *WorkflowGraph) GetNodeDependencies(nodeID string) []string {
	g.RLock()
	defer g.RUnlock()

	deps := make([]string, 0)
	for fromID, toNodes := range g.Edges {
		for _, toID := range toNodes {
			if toID == nodeID {
				deps = append(deps, fromID)
			}
		}
	}
	return deps
}

// CanExecuteNode checks if a node is ready to be executed
func (g *WorkflowGraph) CanExecuteNode(nodeID string) bool {
	g.RLock()
	defer g.RUnlock()

	deps := g.GetNodeDependencies(nodeID)
	for _, depID := range deps {
		if g.Nodes[depID].Status != NodeStatusComplete {
			return false
		}
	}
	return true
}

// GetReadyNodes returns all nodes that are ready to be executed
func (g *WorkflowGraph) GetReadyNodes() []*WorkflowNode {
	g.RLock()
	defer g.RUnlock()

	ready := make([]*WorkflowNode, 0)
	for nodeID, node := range g.Nodes {
		if node.Status == NodeStatusPending && g.CanExecuteNode(nodeID) {
			ready = append(ready, node)
		}
	}
	return ready
}
