package config

import (
	"encoding/json"
	"fmt"

	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

// CapabilityType represents the type of capability (tools, prompts, templates, etc.)
type CapabilityType string

const (
	// CapabilityTypeTool represents a tool capability.
	CapabilityTypeTool CapabilityType = "tools"

	// CapabilityTypePrompt represents a prompt capability.
	CapabilityTypePrompt CapabilityType = "prompts"

	// CapabilityTypeTemplate represents a template capability.
	CapabilityTypeTemplate CapabilityType = "templates"

	// CapabilityTypeResource represents a resource capability.
	CapabilityTypeResource CapabilityType = "resources"
)

// Capability handles specifying which capabilities to enable/disable.
type Capability struct {
	// All indicates whether all capabilities are enabled/disabled
	All bool `json:"all" yaml:"all"`

	// Capabilities is a map of capability types to lists of capabilities
	Capabilities map[CapabilityType][]string `json:"capabilities" yaml:"capabilities"`
}

// Clone creates a deep copy of the Capability instance.
func (c *Capability) Clone() *Capability {
	if c == nil {
		return nil
	}

	clone := &Capability{
		All:          c.All,
		Capabilities: make(map[CapabilityType][]string),
	}

	for key, value := range c.Capabilities {
		clone.Capabilities[key] = make([]string, len(value))
		copy(clone.Capabilities[key], value)
	}

	return clone
}

// HasCapability checks if a specific capability is in the list of capabilities.
func (c *Capability) HasCapability(capability CapabilityType, name string) bool {
	if c.All {
		return true
	}

	if capabilities, ok := c.Capabilities[capability]; ok {
		for _, capability := range capabilities {
			if capability == name {
				return true
			}
		}
	}

	return false
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Capability) UnmarshalJSON(data []byte) error {
	unmarshalFunc := func(data any, target any) error {
		bytes, ok := data.([]byte)
		if !ok {
			return fmt.Errorf("%w: expected []byte, got %T", ErrConfigInvalid, data)
		}

		return json.Unmarshal(bytes, target)
	}

	return c.unmarshal(unmarshalFunc, data)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Capability) UnmarshalYAML(value *yaml.Node) error {
	unmarshalFunc := func(data any, target any) error {
		node, ok := data.(*yaml.Node)
		if !ok {
			return fmt.Errorf("%w: expected *yaml.Node, got %T", ErrConfigInvalid, data)
		}

		return node.Decode(target)
	}

	return c.unmarshal(unmarshalFunc, value)
}

// unmarshalBool attempts to unmarshal data as a boolean.
// Returns true if successful, false otherwise.
func (c *Capability) unmarshalBool(
	unmarshalFunc func(data any, target any) error,
	data any,
) bool {
	var boolValue bool
	if err := unmarshalFunc(data, &boolValue); err == nil {
		c.All = boolValue
		c.Capabilities = nil

		return true
	}

	return false
}

// unmarshalToolList attempts to unmarshal data as a list of strings (tool names).
// Returns true if successful, false otherwise.
func (c *Capability) unmarshalToolList(
	unmarshalFunc func(data any, target any) error,
	data any,
) bool {
	var tools []string
	if err := unmarshalFunc(data, &tools); err == nil {
		c.All = false
		c.Capabilities = map[CapabilityType][]string{
			CapabilityTypeTool: tools,
		}

		return true
	}

	return false
}

// convertToStringArray converts an array of interface{} to an array of strings.
// Returns an error if any element is not a string.
func convertToStringArray(arr []any) ([]string, error) {
	var strArr []string

	for _, item := range arr {
		if str, ok := item.(string); ok {
			strArr = append(strArr, str)
		} else {
			return nil, fmt.Errorf("%w: capability values must be strings", ErrConfigInvalid)
		}
	}

	return strArr, nil
}

// processCapabilityValue processes a capability value which can be a string or array of strings.
// Returns the processed string array and an error if processing fails.
func processCapabilityValue(val any) ([]string, error) {
	// Handle single string value
	if strVal, ok := val.(string); ok {
		return []string{strVal}, nil
	}

	// Handle array of strings ([]any)
	if arr, ok := val.([]any); ok {
		return convertToStringArray(arr)
	}

	// Handle JSON specific type for arrays ([]interface{})
	if arr, ok := val.([]interface{}); ok {
		// Convert to []any for reuse
		anyArr := make([]any, len(arr))

		copy(anyArr, arr)

		return convertToStringArray(anyArr)
	}

	return nil, fmt.Errorf(
		"%w: capability values must be strings or arrays of strings",
		ErrConfigInvalid,
	)
}

// unmarshalCapMap attempts to unmarshal data as a capability map.
// Returns true if successful, false otherwise.
func (c *Capability) unmarshalCapMap(
	unmarshalFunc func(data any, target any) error,
	data any,
) error {
	var capMap map[string]any
	if err := unmarshalFunc(data, &capMap); err != nil {
		return fmt.Errorf(
			"%w: capability config must be a boolean, a list of names, or a map",
			ErrConfigInvalid,
		)
	}

	c.All = false
	c.Capabilities = make(map[CapabilityType][]string)

	for key, val := range capMap {
		capType := CapabilityType(key)

		strArr, err := processCapabilityValue(val)
		if err != nil {
			return err
		}

		c.Capabilities[capType] = strArr
	}

	return nil
}

// unmarshal is a helper function that handles the common unmarshaling logic.
func (c *Capability) unmarshal(
	unmarshalFunc func(data any, target any) error,
	data any,
) error {
	// Try to unmarshal as a boolean
	if c.unmarshalBool(unmarshalFunc, data) {
		return nil
	}

	// Try to unmarshal as a list of strings (tool names)
	if c.unmarshalToolList(unmarshalFunc, data) {
		return nil
	}

	// Try to unmarshal as a capability map
	return c.unmarshalCapMap(unmarshalFunc, data)
}

// CompareCapability compares two capability configurations to check if they are equivalent.
// Returns true if they are equivalent, false otherwise.
func CompareCapability(first, second *Capability) bool {
	// Handle nil cases and basic comparisons
	if areNilCapabilitiesEqual(first, second) {
		return true
	}

	if first == nil || second == nil || first.All != second.All {
		return false
	}

	// If All is true for both, they are equivalent regardless of Capabilities
	if first.All {
		return true
	}

	// Compare Capabilities maps
	return areCapabilityMapsEqual(first.Capabilities, second.Capabilities)
}

// areNilCapabilitiesEqual checks if two nil capabilities are equal.
func areNilCapabilitiesEqual(first, second *Capability) bool {
	return first == nil && second == nil
}

// areCapabilityMapsEqual compares two capability maps for equality.
func areCapabilityMapsEqual(firstMap, secondMap map[CapabilityType][]string) bool {
	if len(firstMap) != len(secondMap) {
		return false
	}

	for capType, firstList := range firstMap {
		secondList, exists := secondMap[capType]
		if !exists {
			return false
		}

		if !areCapabilityListsEqual(firstList, secondList) {
			return false
		}
	}

	return true
}

// areCapabilityListsEqual compares two capability lists for equality (order-insensitive).
func areCapabilityListsEqual(firstList, secondList []string) bool {
	if len(firstList) != len(secondList) {
		return false
	}

	// Convert lists to maps for order-insensitive comparison
	firstSet := makeStringSet(firstList)
	secondSet := makeStringSet(secondList)

	return areSetsEqual(firstSet, secondSet)
}

// makeStringSet converts a string slice to a set (map).
func makeStringSet(items []string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range items {
		result[item] = true
	}

	return result
}

// areSetsEqual checks if two sets have the same elements.
func areSetsEqual(firstSet, secondSet map[string]bool) bool {
	if len(firstSet) != len(secondSet) {
		return false
	}

	for item := range firstSet {
		if _, exists := secondSet[item]; !exists {
			return false
		}
	}

	return true
}
