package interposer

import (
	"fmt"
	"log"
	"sync"
)

// CapabilityKey uniquely identifies a capability by its type and name.
type CapabilityKey struct {
	Type string
	Name string
}

// String returns a string representation of the capability key.
func (k CapabilityKey) String() string {
	return fmt.Sprintf("%s:%s", k.Type, k.Name)
}

// CapabilityRegistry provides a structured approach to tracking capabilities
// and their relationship to backends.
type CapabilityRegistry struct {
	// Maps capability to its providing backend
	capabilities map[CapabilityKey]string

	// Maps backend to the capabilities it provides
	backendCaps map[string]map[CapabilityKey]bool

	mu sync.RWMutex
}

// NewCapabilityRegistry creates a new capability registry.
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		capabilities: make(map[CapabilityKey]string),
		backendCaps:  make(map[string]map[CapabilityKey]bool),
	}
}

// AddCapability registers a capability as being provided by a backend.
func (r *CapabilityRegistry) AddCapability(backend, capType, capName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := CapabilityKey{Type: capType, Name: capName}

	// Add to capabilities map
	r.capabilities[key] = backend

	// Add to backend capabilities map
	if _, exists := r.backendCaps[backend]; !exists {
		r.backendCaps[backend] = make(map[CapabilityKey]bool)
	}

	r.backendCaps[backend][key] = true

	log.Printf("Added capability %s of type %s from backend %s", capName, capType, backend)
}

// RemoveCapability removes a specific capability.
func (r *CapabilityRegistry) RemoveCapability(capType, capName string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := CapabilityKey{Type: capType, Name: capName}

	// Get the backend providing this capability
	backend, exists := r.capabilities[key]
	if !exists {
		return "", false
	}

	// Remove from capabilities map
	delete(r.capabilities, key)

	// Remove from backend capabilities map
	if backendCaps, ok := r.backendCaps[backend]; ok {
		delete(backendCaps, key)
	}

	log.Printf("Removed capability %s of type %s from backend %s", capName, capType, backend)

	return backend, true
}

// RemoveBackendCapabilities removes all capabilities for a specific backend.
// Returns maps of removed capabilities grouped by type.
func (r *CapabilityRegistry) RemoveBackendCapabilities(backend string) map[string][]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	removedByType := make(map[string][]string)

	// Get capabilities for this backend
	backendCaps, exists := r.backendCaps[backend]
	if !exists {
		return removedByType
	}

	// Remove each capability
	for key := range backendCaps {
		// Track removals by type for notification purposes
		if _, exists := removedByType[key.Type]; !exists {
			removedByType[key.Type] = make([]string, 0)
		}

		removedByType[key.Type] = append(removedByType[key.Type], key.Name)

		// Remove from capabilities map
		delete(r.capabilities, key)
	}

	// Remove backend from backendCaps map
	delete(r.backendCaps, backend)

	log.Printf("Removed all capabilities for backend %s", backend)

	return removedByType
}

// GetBackendForCapability returns the backend providing a specific capability.
func (r *CapabilityRegistry) GetBackendForCapability(capType, capName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := CapabilityKey{Type: capType, Name: capName}
	backend, exists := r.capabilities[key]

	return backend, exists
}

// GetCapabilitiesForBackend returns all capabilities provided by a specific backend.
// Results are grouped by capability type.
func (r *CapabilityRegistry) GetCapabilitiesForBackend(backend string) map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)

	backendCaps, exists := r.backendCaps[backend]
	if !exists {
		return result
	}

	for key := range backendCaps {
		if _, exists := result[key.Type]; !exists {
			result[key.Type] = make([]string, 0)
		}

		result[key.Type] = append(result[key.Type], key.Name)
	}

	return result
}

// GetAllBackends returns a list of all backends in the registry.
func (r *CapabilityRegistry) GetAllBackends() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	backends := make([]string, 0, len(r.backendCaps))
	for backend := range r.backendCaps {
		backends = append(backends, backend)
	}

	return backends
}

// HasCapabilitiesOfType returns true if a backend has any capabilities of the specified type.
func (r *CapabilityRegistry) HasCapabilitiesOfType(backend, capType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	backendCaps, exists := r.backendCaps[backend]
	if !exists {
		return false
	}

	for key := range backendCaps {
		if key.Type == capType {
			return true
		}
	}

	return false
}

// GetCapabilitiesByType returns all capabilities of a specific type, grouped by backend.
func (r *CapabilityRegistry) GetCapabilitiesByType(capType string) map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)

	for key, backend := range r.capabilities {
		if key.Type == capType {
			if _, exists := result[backend]; !exists {
				result[backend] = make([]string, 0)
			}

			result[backend] = append(result[backend], key.Name)
		}
	}

	return result
}

// GetCapabilityTypes returns the types of capabilities currently registered.
func (r *CapabilityRegistry) GetCapabilityTypes() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make(map[string]bool)
	for key := range r.capabilities {
		types[key.Type] = true
	}

	return types
}
