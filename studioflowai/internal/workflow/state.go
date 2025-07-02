// Package workflow provides functionality for managing video processing workflows
package workflow

import (
	"time"
)

// AddEvent adds an event to the workflow history in a thread-safe manner
func (s *WorkflowState) AddEvent(event WorkflowEvent) {
	s.Lock()
	defer s.Unlock()
	s.History = append(s.History, event)
}

// UpdateNodeStatus updates a node's status in a thread-safe manner
func (s *WorkflowState) UpdateNodeStatus(nodeID string, status NodeStatus) {
	s.Lock()
	defer s.Unlock()
	if node, exists := s.Graph.Nodes[nodeID]; exists {
		node.Status = status
	}
}

// UpdateNodeOutputs updates a node's outputs in a thread-safe manner
func (s *WorkflowState) UpdateNodeOutputs(nodeID string, outputs map[string]string) {
	s.Lock()
	defer s.Unlock()
	if node, exists := s.Graph.Nodes[nodeID]; exists {
		node.Outputs = outputs
	}
}

// UpdateNodeMetadata updates a node's metadata in a thread-safe manner
func (s *WorkflowState) UpdateNodeMetadata(nodeID string, metadata map[string]interface{}) {
	s.Lock()
	defer s.Unlock()
	if node, exists := s.Graph.Nodes[nodeID]; exists {
		node.Metadata = metadata
	}
}

// GetNodeStatus gets a node's status in a thread-safe manner
func (s *WorkflowState) GetNodeStatus(nodeID string) NodeStatus {
	s.RLock()
	defer s.RUnlock()
	if node, exists := s.Graph.Nodes[nodeID]; exists {
		return node.Status
	}
	return NodeStatusPending
}

// SaveCheckpoint saves the current state as a checkpoint
func (w *Workflow) SaveCheckpoint(nodeID string, state *WorkflowState) {
	w.checkpointMutex.Lock()
	defer w.checkpointMutex.Unlock()

	if w.checkpoints == nil {
		w.checkpoints = make(map[string]*WorkflowCheckpoint)
	}

	checkpoint := &WorkflowCheckpoint{
		NodeID:     nodeID,
		State:      state,
		Timestamp:  time.Now(),
		RetryCount: 0,
	}

	// If checkpoint exists, increment retry count
	if existing, exists := w.checkpoints[nodeID]; exists {
		checkpoint.RetryCount = existing.RetryCount + 1
	}

	w.checkpoints[nodeID] = checkpoint
}

// GetCheckpoint retrieves a checkpoint for a given node
func (w *Workflow) GetCheckpoint(nodeID string) *WorkflowCheckpoint {
	w.checkpointMutex.RLock()
	defer w.checkpointMutex.RUnlock()

	if w.checkpoints == nil {
		return nil
	}

	return w.checkpoints[nodeID]
}

// ClearCheckpoint removes a checkpoint for a given node
func (w *Workflow) ClearCheckpoint(nodeID string) {
	w.checkpointMutex.Lock()
	defer w.checkpointMutex.Unlock()

	if w.checkpoints != nil {
		delete(w.checkpoints, nodeID)
	}
}

// ClearAllCheckpoints removes all checkpoints
func (w *Workflow) ClearAllCheckpoints() {
	w.checkpointMutex.Lock()
	defer w.checkpointMutex.Unlock()

	w.checkpoints = make(map[string]*WorkflowCheckpoint)
}
