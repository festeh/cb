package main

import (
	"sync"
)

// FlowStateManager provides thread-safe access to flow state.
// All access to flow state should go through this manager.
type FlowStateManager struct {
	mu    sync.RWMutex
	state *FlowState
}

var flowManager = &FlowStateManager{}

// Snapshot returns a deep copy of the current state, safe for concurrent use.
// Returns nil if no flow is active.
func (m *FlowStateManager) Snapshot() *FlowState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil {
		return nil
	}

	return m.state.deepCopy()
}

// SnapshotForModel returns formatted state for a specific model/tab.
func (m *FlowStateManager) SnapshotForModel(model string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil {
		return map[string]interface{}{"running": false}
	}

	// Deep copy the content map
	content := make(map[int]string)
	if steps, ok := m.state.Responses[model]; ok {
		for step, text := range steps {
			content[step] = text
		}
	}

	return map[string]interface{}{
		"flow_id": m.state.FlowID,
		"running": m.state.Running,
		"step":    m.state.Step,
		"model":   model,
		"content": content,
	}
}

// GetModelForTab returns the model name for a given tab index.
func (m *FlowStateManager) GetModelForTab(tab int) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil || tab < 0 || tab >= len(m.state.Models) {
		return ""
	}
	return m.state.Models[tab]
}

// IsActive returns true if a flow is currently active.
func (m *FlowStateManager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state != nil
}

// IsRunning returns true if a flow is currently running.
func (m *FlowStateManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state != nil && m.state.Running
}

// HasContent returns true if there's any content in the flow.
func (m *FlowStateManager) HasContent() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil {
		return false
	}
	for _, steps := range m.state.Responses {
		if len(steps) > 0 {
			return true
		}
	}
	return false
}

// Clear resets the flow state to nil.
func (m *FlowStateManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = nil
}

// Initialize creates a new flow state with the given parameters.
func (m *FlowStateManager) Initialize(flowID string, models []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = &FlowState{
		FlowID:    flowID,
		Running:   true,
		Step:      0,
		Models:    models,
		Responses: make(map[string]map[int]string),
	}
	for _, model := range models {
		m.state.Responses[model] = make(map[int]string)
	}
}

// SetRunning updates the running status.
func (m *FlowStateManager) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil {
		m.state.Running = running
	}
}

// SetStep updates the current step.
func (m *FlowStateManager) SetStep(step int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil {
		m.state.Step = step
	}
}

// SetResponse sets the response text for a model at a given step.
func (m *FlowStateManager) SetResponse(model string, step int, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && m.state.Responses[model] != nil {
		m.state.Responses[model][step] = text
	}
}

// deepCopy creates a deep copy of FlowState.
func (f *FlowState) deepCopy() *FlowState {
	if f == nil {
		return nil
	}

	copy := &FlowState{
		FlowID:    f.FlowID,
		Running:   f.Running,
		Step:      f.Step,
		Models:    make([]string, len(f.Models)),
		Responses: make(map[string]map[int]string),
	}

	for i, m := range f.Models {
		copy.Models[i] = m
	}

	for model, steps := range f.Responses {
		copy.Responses[model] = make(map[int]string)
		for step, text := range steps {
			copy.Responses[model][step] = text
		}
	}

	return copy
}
