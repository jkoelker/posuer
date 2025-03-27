package interposer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jkoelker/posuer/pkg/interposer"
)

func TestCapabilityRegistry(t *testing.T) {
	t.Parallel()

	t.Run("add and retrieve capability", func(t *testing.T) {
		t.Parallel()

		registry := interposer.NewCapabilityRegistry()
		registry.AddCapability("backend1", "tool", "tool1")

		// Get by backend
		capabilities := registry.GetCapabilitiesForBackend("backend1")
		assert.Contains(t, capabilities, "tool")
		assert.Contains(t, capabilities["tool"], "tool1")

		// Get by type
		byType := registry.GetCapabilitiesByType("tool")
		assert.Contains(t, byType, "backend1")
		assert.Contains(t, byType["backend1"], "tool1")

		// Get backend for capability
		backend, exists := registry.GetBackendForCapability("tool", "tool1")
		assert.True(t, exists)
		assert.Equal(t, "backend1", backend)

		t.Run("has capabilities of type", func(t *testing.T) {
			t.Parallel()

			registry := interposer.NewCapabilityRegistry()
			registry.AddCapability("backend1", "tool", "tool1")
			registry.AddCapability("backend1", "prompt", "prompt1")

			assert.True(t, registry.HasCapabilitiesOfType("backend1", "tool"))
			assert.True(t, registry.HasCapabilitiesOfType("backend1", "prompt"))
			assert.False(t, registry.HasCapabilitiesOfType("backend1", "resource"))
			assert.False(t, registry.HasCapabilitiesOfType("backend2", "tool"))
		})

		t.Run("get capability types", func(t *testing.T) {
			t.Parallel()

			registry := interposer.NewCapabilityRegistry()
			registry.AddCapability("backend1", "tool", "tool1")
			registry.AddCapability("backend2", "prompt", "prompt1")
			registry.AddCapability("backend3", "resource", "resource1")

			types := registry.GetCapabilityTypes()
			assert.Len(t, types, 3)
			assert.True(t, types["tool"])
			assert.True(t, types["prompt"])
			assert.True(t, types["resource"])
		})

		t.Run("empty registry", func(t *testing.T) {
			t.Parallel()

			registry := interposer.NewCapabilityRegistry()

			// Check empty results
			backends := registry.GetAllBackends()
			assert.Empty(t, backends)

			types := registry.GetCapabilityTypes()
			assert.Empty(t, types)

			capabilities := registry.GetCapabilitiesForBackend("backend1")
			assert.Empty(t, capabilities)

			byType := registry.GetCapabilitiesByType("tool")
			assert.Empty(t, byType)

			removedByType := registry.RemoveBackendCapabilities("backend1")
			assert.Empty(t, removedByType)

			_, exists := registry.GetBackendForCapability("tool", "tool1")
			assert.False(t, exists)
		})
	})

	t.Run("remove capability", func(t *testing.T) {
		t.Parallel()

		registry := interposer.NewCapabilityRegistry()
		registry.AddCapability("backend1", "tool", "tool1")
		registry.AddCapability("backend1", "tool", "tool2")

		// Check that it was added
		capabilities := registry.GetCapabilitiesForBackend("backend1")
		assert.Len(t, capabilities["tool"], 2)

		// Remove one capability
		backend, removed := registry.RemoveCapability("tool", "tool1")
		assert.True(t, removed)
		assert.Equal(t, "backend1", backend)

		// Check that it was removed
		capabilities = registry.GetCapabilitiesForBackend("backend1")
		assert.Len(t, capabilities["tool"], 1)
		assert.Contains(t, capabilities["tool"], "tool2")

		// Verify backend for removed capability no longer exists
		backend, exists := registry.GetBackendForCapability("tool", "tool1")
		assert.False(t, exists)
		assert.Empty(t, backend)
	})

	t.Run("remove all backend capabilities", func(t *testing.T) {
		t.Parallel()

		registry := interposer.NewCapabilityRegistry()
		registry.AddCapability("backend1", "tool", "tool1")
		registry.AddCapability("backend1", "tool", "tool2")
		registry.AddCapability("backend1", "prompt", "prompt1")
		registry.AddCapability("backend2", "tool", "tool3")

		// Remove all capabilities for backend1
		removedByType := registry.RemoveBackendCapabilities("backend1")

		// Check return value
		assert.Len(t, removedByType, 2) // Two types: tool and prompt
		assert.Len(t, removedByType["tool"], 2)
		assert.Len(t, removedByType["prompt"], 1)
		assert.Contains(t, removedByType["tool"], "tool1")
		assert.Contains(t, removedByType["tool"], "tool2")
		assert.Contains(t, removedByType["prompt"], "prompt1")

		// Check that backend1 capabilities are gone
		capabilities := registry.GetCapabilitiesForBackend("backend1")
		assert.Empty(t, capabilities)

		// Check that backend2 capabilities are still there
		capabilities = registry.GetCapabilitiesForBackend("backend2")
		assert.Len(t, capabilities["tool"], 1)
		assert.Contains(t, capabilities["tool"], "tool3")
	})
}
