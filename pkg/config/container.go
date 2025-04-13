package config

import (
	"encoding/json"
	"fmt"

	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

// DefaultContainerWorkdir is the default working directory for the container.
const DefaultContainerWorkDir = "/code"

// Container represents configuration for a container.
// It can be nil to indicate no container configuration,
// set to false to explicitly disable automatic container detection,
// or configured with specific container settings.
type Container struct {
	// Image is the container image to use.
	Image string `json:"image" yaml:"image"`

	// Volumes maps host paths to container paths.
	Volumes map[string]string `json:"volumes" yaml:"volumes"`

	// EnvVars contains environment variables to pass to the container.
	Env map[string]string `json:"env" yaml:"env"`

	// Network specifies the network mode (host, bridge, etc.).
	Network string `json:"network" yaml:"network"`

	// User specifies the user to run as in the container.
	User string `json:"user" yaml:"user"`

	// WorkDir specifies the working directory in the container.
	WorkDir string `json:"workdir" yaml:"workdir"`

	// AdditionalArgs contains any additional arguments to pass to the container runtime.
	AdditionalArgs []string `json:"args" yaml:"args"`
}

// Clone creates a deep copy of the Container configuration.
func (c *Container) Clone() *Container {
	if c == nil {
		return nil
	}

	clone := *c

	if c.Volumes != nil {
		clone.Volumes = make(map[string]string, len(c.Volumes))
		for k, v := range c.Volumes {
			clone.Volumes[k] = v
		}
	}

	if c.Env != nil {
		clone.Env = make(map[string]string, len(c.Env))
		for k, v := range c.Env {
			clone.Env[k] = v
		}
	}

	if c.AdditionalArgs != nil {
		clone.AdditionalArgs = make([]string, len(c.AdditionalArgs))
		copy(clone.AdditionalArgs, c.AdditionalArgs)
	}

	return &clone
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Container) UnmarshalYAML(value *yaml.Node) error {
	unmarshalFunc := func(data any, target any) error {
		node, ok := data.(*yaml.Node)
		if !ok {
			return fmt.Errorf("%w: expected *yaml.Node, got %T", ErrConfigInvalid, data)
		}

		return node.Decode(target)
	}

	return c.unmarshal(unmarshalFunc, value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Container) UnmarshalJSON(data []byte) error {
	unmarshalFunc := func(data any, target any) error {
		bytes, ok := data.([]byte)
		if !ok {
			return fmt.Errorf("%w: expected []byte, got %T", ErrConfigInvalid, data)
		}

		return json.Unmarshal(bytes, target)
	}

	return c.unmarshal(unmarshalFunc, data)
}

// IsDisabled returns true if container isolation is explicitly disabled.
func (c *Container) IsDisabled() bool {
	return c != nil && c.Image == "" && c.Volumes == nil &&
		c.Env == nil && c.Network == "" && c.User == "" &&
		c.WorkDir == "" && c.AdditionalArgs == nil
}

// IsConfigured returns true if the container has a valid configuration.
func (c *Container) IsConfigured() bool {
	return c != nil && c.Image != "" && !c.IsDisabled()
}

// unmarshal is a helper function to unmarshal the configuration.
func (c *Container) unmarshal(unmarshalFunc func(data any, target any) error, data any) error {
	// Try to unmarshal as a boolean
	var boolValue bool
	if err := unmarshalFunc(data, &boolValue); err == nil {
		// If false, mark as explicitly disabled with empty values
		if !boolValue {
			// Set empty image to mark as disabled
			c.Image = ""
			c.Volumes = nil
			c.Env = nil
			c.Network = ""
			c.User = ""
			c.WorkDir = ""
			c.AdditionalArgs = nil

			return nil
		}
		// If true, this doesn't make sense in our context, so return an error
		return fmt.Errorf("%w: boolean true is not a valid container configuration", ErrConfigInvalid)
	}

	// Try to unmarshal as a string (image name)
	var image string
	if err := unmarshalFunc(data, &image); err == nil {
		c.Image = image

		return nil
	}

	// Try to unmarshal as a full container configuration
	type ContainerAlias Container

	var container ContainerAlias
	if err := unmarshalFunc(data, &container); err != nil {
		return fmt.Errorf("%w: %w", ErrConfigInvalid, err)
	}

	c.Image = container.Image
	c.Volumes = container.Volumes
	c.Env = container.Env
	c.Network = container.Network
	c.User = container.User
	c.WorkDir = container.WorkDir
	c.AdditionalArgs = container.AdditionalArgs

	return nil
}
