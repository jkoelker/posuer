package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "sigs.k8s.io/yaml/goyaml.v3"

	"github.com/jkoelker/posuer/pkg/config"
)

func TestLoadYAMLConfig(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a test config file
	mainConfigPath := filepath.Join(tempDir, "config.yaml")
	mainConfigContent := `
servers:
  - name: direct-server
    type: stdio
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - "/tmp"
  - included.yaml
`
	err := os.WriteFile(mainConfigPath, []byte(mainConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Create an included config file
	includedConfigPath := filepath.Join(tempDir, "included.yaml")
	includedConfigContent := `
servers:
  - name: included-server
    type: stdio
    command: python
    args:
      - -m
      - mcp_server_git
  - name: second-server
    type: sse
    url: http://localhost:8080/sse
`
	err = os.WriteFile(includedConfigPath, []byte(includedConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Load the config
	servers, err := config.LoadConfig(mainConfigPath)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 3)

	// Check the direct server
	assert.Equal(t, "direct-server", servers[0].Name)
	assert.Equal(t, config.ServerTypeStdio, servers[0].Type)
	assert.Equal(t, "npx", servers[0].Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, servers[0].Args)

	// Check the included servers
	assert.Equal(t, "included-server", servers[1].Name)
	assert.Equal(t, config.ServerTypeStdio, servers[1].Type)
	assert.Equal(t, "python", servers[1].Command)
	assert.Equal(t, []string{"-m", "mcp_server_git"}, servers[1].Args)

	assert.Equal(t, "second-server", servers[2].Name)
	assert.Equal(t, config.ServerTypeSSE, servers[2].Type)
	assert.Equal(t, "http://localhost:8080/sse", servers[2].URL)
}

func TestLoadClaudeConfig(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a Claude Desktop config file
	claudeConfigPath := filepath.Join(tempDir, "claude_config.json")
	claudeConfigContent := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    },
    "git": {
      "command": "python",
      "args": ["-m", "mcp_server_git"]
    }
  }
}`
	err := os.WriteFile(claudeConfigPath, []byte(claudeConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Load the config
	servers, err := config.LoadConfig(claudeConfigPath)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 2)

	// Check servers (order may vary since they come from a map)
	for _, server := range servers {
		switch server.Name {
		case "filesystem":
			assert.Equal(t, config.ServerTypeStdio, server.Type)
			assert.Equal(t, "npx", server.Command)
			assert.Equal(
				t,
				[]string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
				server.Args,
			)
		case "git":
			assert.Equal(t, config.ServerTypeStdio, server.Type)
			assert.Equal(t, "python", server.Command)
			assert.Equal(t, []string{"-m", "mcp_server_git"}, server.Args)
		default:
			t.Fatalf("Unexpected server name: %s", server.Name)
		}
	}
}

func TestNestedIncludes(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a main config file
	mainConfigPath := filepath.Join(tempDir, "config.yaml")
	mainConfigContent := `
servers:
  - name: main-server
    type: stdio
    command: npx
    args:
      - -y
      - server-main
  - level1.yaml
`
	err := os.WriteFile(mainConfigPath, []byte(mainConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Create a level 1 included file
	level1Path := filepath.Join(tempDir, "level1.yaml")
	level1Content := `
servers:
  - name: level1-server
    type: stdio
    command: npx
    args:
      - -y
      - server-level1
  - level2.yaml
`
	err = os.WriteFile(level1Path, []byte(level1Content), config.FilePermissions)
	require.NoError(t, err)

	// Create a level 2 included file
	level2Path := filepath.Join(tempDir, "level2.yaml")
	level2Content := `
servers:
  - name: level2-server
    type: stdio
    command: npx
    args:
      - -y
      - server-level2
`
	err = os.WriteFile(level2Path, []byte(level2Content), config.FilePermissions)
	require.NoError(t, err)

	// Load the config
	servers, err := config.LoadConfig(mainConfigPath)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 3)

	// Check servers
	assert.Equal(t, "main-server", servers[0].Name)
	assert.Equal(t, "level1-server", servers[1].Name)
	assert.Equal(t, "level2-server", servers[2].Name)
}

func TestMixedFormatIncludes(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a main YAML config file
	mainConfigPath := filepath.Join(tempDir, "config.yaml")
	mainConfigContent := `
servers:
  - name: yaml-server
    type: stdio
    command: npx
    args:
      - -y
      - server-yaml
  - claude.json
`
	err := os.WriteFile(mainConfigPath, []byte(mainConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Create a Claude Desktop JSON config file
	claudeConfigPath := filepath.Join(tempDir, "claude.json")
	claudeConfigContent := `{
  "mcpServers": {
    "json-server": {
      "command": "npx",
      "args": ["-y", "server-json"]
    }
  }
}`
	err = os.WriteFile(claudeConfigPath, []byte(claudeConfigContent), config.FilePermissions)
	require.NoError(t, err)

	// Load the config
	servers, err := config.LoadConfig(mainConfigPath)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 2)

	// Check servers
	assert.Equal(t, "yaml-server", servers[0].Name)
	assert.Equal(t, "json-server", servers[1].Name)
}

func TestLoadWithProvidedPath(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a test config file
	configPath := filepath.Join(tempDir, "custom-config.yaml")
	configContent := `
servers:
  - name: custom-server
    type: stdio
    command: custom-command
    args:
      - arg1
      - arg2
`
	err := os.WriteFile(configPath, []byte(configContent), config.FilePermissions)
	require.NoError(t, err)

	// Load the config with provided path
	servers, err := config.Load(configPath)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 1)
	assert.Equal(t, "custom-server", servers[0].Name)
	assert.Equal(t, config.ServerTypeStdio, servers[0].Type)
	assert.Equal(t, "custom-command", servers[0].Command)
	assert.Equal(t, []string{"arg1", "arg2"}, servers[0].Args)
}

func TestLoadWithNonexistentProvidedPath(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Try to load a non-existent config file
	_, err := config.Load(filepath.Join(tempDir, "nonexistent-config.yaml"))
	assert.ErrorIs(t, err, config.ErrConfigNotFound)
}

func TestLoadWithDefaultPath(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a test config file in the mocked user config dir
	configDir := filepath.Join(tempDir, config.DefaultConfigDirName)
	err := os.MkdirAll(configDir, config.DirectoryPermissions)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, config.DefaultConfigFileName)
	configContent := `
servers:
  - name: default-server
    type: stdio
    command: default-command
    args:
      - arg1
      - arg2
`
	err = os.WriteFile(configPath, []byte(configContent), config.FilePermissions)
	require.NoError(t, err)

	// Load the config without providing a path
	servers, err := config.Load(
		"",
		config.WithUserConfigDir(
			func() (string, error) {
				return tempDir, nil
			},
		),
	)
	require.NoError(t, err)

	// Verify the results
	assert.Len(t, servers, 1)
	assert.Equal(t, "default-server", servers[0].Name)
}

func TestCreateExampleConfig(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Load the config without providing a path, which should create an example
	servers, err := config.Load(
		"",
		config.WithUserConfigDir(
			func() (string, error) {
				return tempDir, nil
			},
		),
	)
	require.NoError(t, err)

	// Verify the example config was created
	configPath := filepath.Join(tempDir, config.DefaultConfigDirName, config.DefaultConfigFileName)
	assert.FileExists(t, configPath)

	// Verify the loaded servers match what's expected from the example
	assert.GreaterOrEqual(t, len(servers), 1)

	// Check if one of the servers is the filesystem server from the example
	found := false

	for _, server := range servers {
		if server.Name == "filesystem" {
			found = true

			assert.Equal(t, config.ServerTypeStdio, server.Type)
			assert.Equal(t, "npx", server.Command)

			break
		}
	}

	assert.True(t, found, "Example filesystem server not found in loaded config")
}

func TestServerDisabled(t *testing.T) {
	t.Parallel()

	t.Run("no config", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Disabled())
	})

	t.Run("enabled with boolean", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable: true"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Disabled())
	})

	t.Run("disabled with boolean", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\ndisable: true"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Disabled())
	})

	t.Run("specific capabilities", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  tools:\n    - tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Disabled())
	})
}

func TestServerEnabled(t *testing.T) {
	t.Parallel()

	t.Run("no config", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Default is enabled
		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("server disabled with boolean", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\ndisable: true"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("server enabled with boolean", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable: true"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("legacy list enabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  - tool1\n  - tool2"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool3"))
	})

	t.Run("legacy list disabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\ndisable:\n  - tool1\n  - tool2"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool3"))
	})

	t.Run("type list enabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  tools:\n    - tool1\n    - tool2"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool3"))
	})

	t.Run("type single item enabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  tools: tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool2"))
	})

	t.Run("different capability type", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  prompts:\n    - prompt1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Not explicitly enabled, so it's disabled
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("explicitly disabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\ndisable:\n  tools:\n    - tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool2"))
	})

	t.Run("disabled single item", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\ndisable:\n  tools: tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
		assert.True(t, server.Enabled(config.CapabilityTypeTool, "tool2"))
	})

	t.Run("enabled and disabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  tools:\n    - tool1\ndisable:\n  tools:\n    - tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Disable takes precedence
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("legacy enabled but disabled", func(t *testing.T) {
		t.Parallel()

		yamlStr := "name: test-server\ntype: stdio\nenable:\n  - tool1\ndisable:\n  tools:\n    - tool1"

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		// Disable takes precedence
		assert.False(t, server.Enabled(config.CapabilityTypeTool, "tool1"))
	})

	t.Run("resource type", func(t *testing.T) {
		t.Parallel()

		yamlStr := `
name: test-server
type: stdio
enable:
  resources:
    - resource1
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlStr), &server)
		require.NoError(t, err)

		assert.True(t, server.Enabled(config.CapabilityTypeResource, "resource1"))
		assert.False(t, server.Enabled(config.CapabilityTypeResource, "resource2"))
	})
}
