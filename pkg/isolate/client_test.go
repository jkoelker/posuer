package isolate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jkoelker/posuer/pkg/config"
	"github.com/jkoelker/posuer/pkg/isolate"
)

func TestClientNoContainer(t *testing.T) {
	t.Parallel()

	cfg := config.Server{
		Name:    "test",
		Command: "echo",
		Args:    []string{"hello"},
	}

	client, err := isolate.Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClientExplicitContainer(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	cfg := config.Server{
		Name:    "test",
		Command: "echo",
		Args:    []string{"hello"},
		Container: &config.Container{
			Image: "alpine:latest",
		},
	}

	client, err := isolate.Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClientDisabledContainer(t *testing.T) {
	t.Parallel()

	// Create a container that is explicitly disabled
	disabledContainer := &config.Container{}
	disabledContainer.Image = ""

	cfg := config.Server{
		Name:      "test",
		Command:   "npx", // Would normally trigger automatic container
		Args:      []string{"hello"},
		Container: disabledContainer,
	}

	client, err := isolate.Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClientAutomaticNPXContainer(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	cfg := config.Server{
		Name:    "test",
		Command: "npx",
		Args:    []string{"hello"},
	}

	client, err := isolate.Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClientAutomaticUVXContainer(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	cfg := config.Server{
		Name:    "test",
		Command: "uvx",
		Args:    []string{"hello"},
	}

	client, err := isolate.Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNPXContainerVolumes(t *testing.T) {
	t.Parallel()

	testContainerVolumes(t, "npx", "/root/.npm")
}

func TestUVXContainerVolumes(t *testing.T) {
	t.Parallel()

	testContainerVolumes(t, "uvx", "/root/.cache/uv")
}

func testContainerVolumes(t *testing.T, cmdName, expectedContainerPath string) {
	t.Helper()

	// Create a test configuration with specified command
	cfg := config.Server{
		Name:    "test",
		Command: cmdName,
		Args:    []string{"hello"},
	}

	// Create server config with container
	server, cacheDirPath := setupContainerConfig(t, cfg)

	// Verify the volumes
	require.NotNil(t, server.Container.Volumes)

	containerPath, exists := server.Container.Volumes[cacheDirPath]
	assert.True(t, exists, cmdName+" cache directory not mounted")
	assert.Equal(t, expectedContainerPath, containerPath)
}

func setupContainerConfig(t *testing.T, cfg config.Server) (config.Server, string) {
	t.Helper()

	// Clone the server config
	server := cfg.Clone()

	// Ensure Container is initialized
	if server.Container == nil {
		server.Container = &config.Container{}
	}

	// Set the default image
	server.Container.Image = isolate.DefaultImageForCommand(cfg.Command)

	// Ensure Env is initialized
	if server.Container.Env == nil {
		server.Container.Env = make(map[string]string)
	}

	// Get default volumes for the command
	volumes, err := isolate.DefaultVolumesForCommand(cfg.Command)
	require.NoError(t, err)

	// Add the volumes to the container configuration
	if len(volumes) > 0 {
		// Ensure Volumes is initialized
		if server.Container.Volumes == nil {
			server.Container.Volumes = make(map[string]string)
		}

		// Add each volume mapping
		for k, v := range volumes {
			server.Container.Volumes[k] = v
		}
	}

	// Get cache directory path
	cache, err := os.UserCacheDir()
	require.NoError(t, err)

	cacheDirPath := filepath.Join(cache, config.DefaultConfigDirName, cfg.Command)

	return server, cacheDirPath
}
