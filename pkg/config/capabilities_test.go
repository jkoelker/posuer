package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "sigs.k8s.io/yaml/goyaml.v3"

	"github.com/jkoelker/posuer/pkg/config"
)

const (
	True  = "true"
	False = "false"
)

func TestCapabilityUnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("boolean true", func(t *testing.T) {
		t.Parallel()

		var result config.Capability
		err := yaml.Unmarshal([]byte(True), &result)

		require.NoError(t, err)
		assert.True(t, result.All)
		assert.Nil(t, result.Capabilities)
	})

	t.Run("boolean false", func(t *testing.T) {
		t.Parallel()

		var result config.Capability
		err := yaml.Unmarshal([]byte(False), &result)

		require.NoError(t, err)
		assert.False(t, result.All)
		assert.Nil(t, result.Capabilities)
	})

	t.Run("list of tool names", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
- tool1
- tool2
- tool3
`

		var result config.Capability
		err := yaml.Unmarshal([]byte(yamlStr), &result)

		require.NoError(t, err)
		assert.False(t, result.All)
		require.NotNil(t, result.Capabilities)
		assert.ElementsMatch(
			t,
			[]string{"tool1", "tool2", "tool3"},
			result.Capabilities[config.CapabilityTypeTool],
		)
	})

	t.Run("map with single string", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
tools: tool1
prompts: prompt1
`

		var result config.Capability
		err := yaml.Unmarshal([]byte(yamlStr), &result)

		require.NoError(t, err)
		assert.False(t, result.All)
		require.NotNil(t, result.Capabilities)
		assert.ElementsMatch(t, []string{"tool1"}, result.Capabilities[config.CapabilityTypeTool])
		assert.ElementsMatch(t, []string{"prompt1"}, result.Capabilities[config.CapabilityTypePrompt])
	})

	t.Run("map with lists", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
tools:
  - tool1
  - tool2
prompts:
  - prompt1
templates:
  - template1
  - template2
`

		var result config.Capability
		err := yaml.Unmarshal([]byte(yamlStr), &result)

		require.NoError(t, err)
		assert.False(t, result.All)
		require.NotNil(t, result.Capabilities)
		assert.ElementsMatch(t, []string{"tool1", "tool2"}, result.Capabilities[config.CapabilityTypeTool])
		assert.ElementsMatch(t, []string{"prompt1"}, result.Capabilities[config.CapabilityTypePrompt])
		assert.ElementsMatch(t, []string{"template1", "template2"}, result.Capabilities[config.CapabilityTypeTemplate])
	})

	t.Run("mixed types in map values", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
tools: true
prompts:
  - prompt1
`

		var result config.Capability
		err := yaml.Unmarshal([]byte(yamlStr), &result)

		assert.Error(t, err)
	})
}

func TestCompareCapability(t *testing.T) {
	t.Parallel()

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()

		a := (*config.Capability)(nil)
		b := (*config.Capability)(nil)

		result := config.CompareCapability(a, b)
		assert.True(t, result)
	})

	t.Run("one nil", func(t *testing.T) {
		t.Parallel()

		a := &config.Capability{All: true}
		b := (*config.Capability)(nil)

		result := config.CompareCapability(a, b)
		assert.False(t, result)
	})

	t.Run("both all true", func(t *testing.T) {
		t.Parallel()

		a := &config.Capability{All: true}
		b := &config.Capability{All: true}

		result := config.CompareCapability(a, b)
		assert.True(t, result)
	})

	t.Run("all flag different", func(t *testing.T) {
		t.Parallel()

		a := &config.Capability{All: true}
		b := &config.Capability{All: false}

		result := config.CompareCapability(a, b)
		assert.False(t, result)
	})

	t.Run("empty capabilities", func(t *testing.T) {
		t.Parallel()

		a := &config.Capability{All: false, Capabilities: map[config.CapabilityType][]string{}}
		b := &config.Capability{All: false, Capabilities: map[config.CapabilityType][]string{}}

		result := config.CompareCapability(a, b)
		assert.True(t, result)
	})

	t.Run("same capabilities different order", func(t *testing.T) {
		t.Parallel()

		one := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool1", "tool2"},
			},
		}
		two := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool2", "tool1"},
			},
		}

		result := config.CompareCapability(one, two)
		assert.True(t, result)
	})

	t.Run("different capabilities", func(t *testing.T) {
		t.Parallel()

		one := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool1", "tool2"},
			},
		}
		two := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool1", "tool3"},
			},
		}

		result := config.CompareCapability(one, two)
		assert.False(t, result)
	})

	t.Run("different capability types", func(t *testing.T) {
		t.Parallel()

		one := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool1", "tool2"},
			},
		}
		two := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypePrompt: {"prompt1", "prompt2"},
			},
		}

		result := config.CompareCapability(one, two)
		assert.False(t, result)
	})

	t.Run("complex match", func(t *testing.T) {
		t.Parallel()

		one := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool:     {"tool1", "tool2"},
				config.CapabilityTypePrompt:   {"prompt1"},
				config.CapabilityTypeResource: {"resource1", "resource2"},
			},
		}
		two := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool:     {"tool2", "tool1"},
				config.CapabilityTypePrompt:   {"prompt1"},
				config.CapabilityTypeResource: {"resource2", "resource1"},
			},
		}

		result := config.CompareCapability(one, two)
		assert.True(t, result)
	})
}

func TestServerWithCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("complex configuration", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
command: test-command
args:
  - arg1
  - arg2
enable:
  tools:
    - tool1
    - tool2
  prompts: prompt1
disable:
  templates:
    - template1
`

		var serverConfig config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &serverConfig)
		require.NoError(t, err)

		assert.Equal(t, "test-server", serverConfig.Name)
		assert.Equal(t, config.ServerTypeStdio, serverConfig.Type)
		assert.Equal(t, "test-command", serverConfig.Command)
		assert.Equal(t, []string{"arg1", "arg2"}, serverConfig.Args)

		require.NotNil(t, serverConfig.Enable)
		require.NotNil(t, serverConfig.Enable.Capabilities)
		assert.ElementsMatch(t, []string{"tool1", "tool2"}, serverConfig.Enable.Capabilities[config.CapabilityTypeTool])
		assert.ElementsMatch(t, []string{"prompt1"}, serverConfig.Enable.Capabilities[config.CapabilityTypePrompt])

		require.NotNil(t, serverConfig.Disable)
		require.NotNil(t, serverConfig.Disable.Capabilities)
		assert.ElementsMatch(t, []string{"template1"}, serverConfig.Disable.Capabilities[config.CapabilityTypeTemplate])
	})
}

func TestCapability_HasCapability(t *testing.T) {
	t.Parallel()

	t.Run("all true", func(t *testing.T) {
		t.Parallel()

		capability := &config.Capability{
			All:          true,
			Capabilities: nil,
		}

		assert.True(t, capability.HasCapability(config.CapabilityTypeTool, "anything"))
	})

	t.Run("item in list", func(t *testing.T) {
		t.Parallel()

		capability := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"tool1", "tool2"},
			},
		}

		assert.True(t, capability.HasCapability(config.CapabilityTypeTool, "tool1"))
		assert.False(t, capability.HasCapability(config.CapabilityTypeTool, "tool3"))
	})

	t.Run("wrong capability type", func(t *testing.T) {
		t.Parallel()

		capability := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypePrompt: {"prompt1"},
			},
		}

		assert.False(t, capability.HasCapability(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("empty capability list", func(t *testing.T) {
		t.Parallel()

		capability := &config.Capability{
			All: false,
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {},
			},
		}

		assert.False(t, capability.HasCapability(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("nil capability map", func(t *testing.T) {
		t.Parallel()

		capability := &config.Capability{
			All:          false,
			Capabilities: nil,
		}

		assert.False(t, capability.HasCapability(config.CapabilityTypeTool, "tool1"))
	})
}

func TestEmptyCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("empty tools list", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable:
  tools: []
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Server with empty tools list should be considered disabled
		assert.True(t, server.Disabled())
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("empty enable map", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable: {}
disable:
  tools:
    - tool1
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Server should be considered disabled due to empty enable
		assert.True(t, server.Disabled())
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool2"))
	})
}

func TestMixedCapabilityFormats(t *testing.T) {
	t.Parallel()

	t.Run("multiple capability types", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable:
  tools:
    - tool1
  prompts: prompt1
disable:
  tools:
    - tool2
  templates:
    - template1
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Check enabled tools and prompts
		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.True(t, server.Enabled(config.CapabilityTypePrompt, "prompt1"))

		// Check disabled tools and templates
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool2"))
		assert.False(t, server.Enabled(config.CapabilityTypeTemplate, "template1"))
	})

	t.Run("enable true with disable false", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable: true
disable: false
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeTool, "anytool"))
	})

	t.Run("enable false with disable true", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable: false
disable: true
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Enabled(config.CapabilityTypeTool, "anytool"))
	})
}
