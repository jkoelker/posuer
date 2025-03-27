package isolate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/client"

	"github.com/jkoelker/posuer/pkg/config"
)

// Isolator defines an interface for creating isolated MCP clients.
type Isolator interface {
	// Isolate takes a server config and returns an MCP client
	Isolate(cfg config.Server) (client.MCPClient, error)
}

// IsolatorType represents the type of isolator to use.
type IsolatorType string

const (
	// TypeNoop represents no isolation.
	TypeNoop IsolatorType = "noop"

	// TypeContainer represents container-based isolation.
	TypeContainer IsolatorType = "container"

	// NPX represents the Node Package Executor.
	NPX = "npx"

	// NPXImage represents the default image for NPX.
	NPXImage = "docker.io/node:alpine"

	// UVX represents the Universal Executor.
	UVX = "uvx"

	// UVXImage represents the default image for UVX.
	UVXImage = "ghcr.io/astral-sh/uv:alpine"
)

// DefaultImageForCommand returns the default container image for a command.
func DefaultImageForCommand(command string) string {
	switch command {
	case NPX:
		return NPXImage
	case UVX:
		return UVXImage
	default:
		return ""
	}
}

// DefaultVolumesForCommand returns the default volume mappings for a command.
func DefaultVolumesForCommand(command string) (map[string]string, error) {
	volumes := make(map[string]string)

	// Get user's cache directory
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user cache dir: %w", err)
	}

	// Create our own cache directory for the specific command
	// Using the pattern cache/posuer/command
	defaultCacheDir := filepath.Join(cache, config.DefaultConfigDirName, command)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(defaultCacheDir, config.DirectoryPermissions); err != nil {
		return nil, fmt.Errorf("failed to create default cache dir: %w", err)
	}

	switch command {
	case NPX:
		// Mount our npm cache directory to /root/.npm in the container
		volumes[defaultCacheDir] = "/root/.npm"

	case UVX:
		// Mount our uv cache directory to /root/.cache/uv in the container
		volumes[defaultCacheDir] = "/root/.cache/uv"
	}

	return volumes, nil
}

func noopIsolator(cfg config.Server) (client.MCPClient, error) {
	// No isolation, return the original config
	client, err := NewNoop().Isolate(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	return client, nil
}

func containerIsolator(cfg config.Server) (client.MCPClient, error) {
	// Container isolation, return the container isolator
	isolator, err := NewContainer()
	if err != nil {
		return nil, fmt.Errorf("failed to create container isolator: %w", err)
	}

	client, err := isolator.Isolate(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	return client, nil
}

func defaultContainerIsolator(cfg config.Server) (client.MCPClient, error) {
	server := cfg.Clone()

	// Ensure Container is initialized
	if server.Container == nil {
		server.Container = &config.Container{}
	}

	// Set the default image
	server.Container.Image = DefaultImageForCommand(cfg.Command)

	// Ensure Env is initialized
	if server.Container.Env == nil {
		server.Container.Env = make(map[string]string)
	}

	// Get default volumes for the command
	volumes, err := DefaultVolumesForCommand(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to get default volumes for command %s: %w", cfg.Command, err)
	}

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

	if server.Container.WorkDir == "" {
		// Set the default working directory
		server.Container.WorkDir = config.DefaultContainerWorkDir

		// Check if the working directory is already in the volumes
		if _, ok := server.Container.Volumes[server.Container.WorkDir]; !ok {
			// Get the current working directory
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current working directory: %w", err)
			}

			// Add the current working directory to the volumes
			server.Container.Volumes[cwd] = server.Container.WorkDir
		}
	}

	return containerIsolator(server)
}

// Client creates an MCP client using the appropriate isolator for the config.
func Client(cfg config.Server) (client.MCPClient, error) {
	// Decide which isolation strategy to use
	switch {
	case cfg.Container != nil && cfg.Container.IsDisabled():
		return noopIsolator(cfg)

	case cfg.Container != nil && cfg.Container.IsConfigured():
		return containerIsolator(cfg)

	case DefaultImageForCommand(cfg.Command) != "":
		return defaultContainerIsolator(cfg)

	default:
		return noopIsolator(cfg)
	}
}
