package isolate

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/client"

	"github.com/jkoelker/posuer/pkg/config"
)

const (
	// DockerRuntime is the name of the Docker container runtime.
	DockerRuntime = "docker"

	// PodmanRuntime is the name of the Podman container runtime.
	PodmanRuntime = "podman"
)

// ErrNoContainerRuntime indicates that no container runtime was found.
var ErrNoContainerRuntime = errors.New("no container runtime found")

// Container implements the Isolator interface using containers.
type Container struct {
	runtime string
	once    sync.Once
}

// WithContainerRuntime specifies the container runtime to use.
func WithContainerRuntime(runtime string) func(*Container) {
	return func(isolator *Container) {
		isolator.runtime = runtime
	}
}

// NewContainer creates a new Container.
func NewContainer(options ...func(*Container)) (*Container, error) {
	isolator := &Container{}

	for _, option := range options {
		option(isolator)
	}

	if err := isolator.detectRuntime(); err != nil {
		return nil, fmt.Errorf("failed to detect container runtime: %w", err)
	}

	return isolator, nil
}

// Isolate creates an MCP client isolated in a container.
func (c *Container) Isolate(cfg config.Server) (client.MCPClient, error) {
	// Skip if already a container command or if Container is nil or explicitly disabled
	if IsContainerCommand(cfg.Command) ||
		cfg.Container == nil ||
		cfg.Container.IsDisabled() ||
		!cfg.Container.IsConfigured() {
		return NewNoop().Isolate(cfg)
	}

	server := cfg.Clone()

	// Make sure Container.Env is initialized
	if server.Container.Env == nil {
		server.Container.Env = make(map[string]string)
	}

	// Copy environment variables from the server config to the container config
	for key, value := range cfg.Env {
		server.Container.Env[key] = value
	}

	// Build container command and args
	args, err := ContainerCommand(
		cfg.Command,
		cfg.Args,
		server.Container,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build container command for %s: %w", cfg.Name, err)
	}

	// Replace the original command with the container command
	server.Command = c.runtime
	server.Args = args
	server.Container = nil

	return NewNoop().Isolate(server)
}

// detectRuntime returns the available container runtime and its path.
func (c *Container) detectRuntime() error {
	var err error

	c.once.Do(func() {
		if c.runtime != "" {
			return
		}

		var path string

		// Check for podman
		if path, err = exec.LookPath(PodmanRuntime); err == nil {
			c.runtime = path

			return
		}

		// Check for docker
		if path, err = exec.LookPath(DockerRuntime); err == nil {
			c.runtime = path

			return
		}

		// No container runtime found
		err = ErrNoContainerRuntime
	})

	return err
}

// IsContainerCommand checks if the command is already a container command.
func IsContainerCommand(command string) bool {
	return command == DockerRuntime || command == PodmanRuntime ||
		strings.HasSuffix(command, "/"+DockerRuntime) ||
		strings.HasSuffix(command, "/"+PodmanRuntime)
}

// ContainerCommand takes a command, arguments, and container config and returns
// a container-wrapped command and arguments.
func ContainerCommand(
	command string,
	args []string,
	config *config.Container,
) ([]string, error) {
	// Start building the container arguments
	containerArgs := []string{"run", "--rm", "--interactive"}

	// Add volumes
	for host, container := range config.Volumes {
		containerArgs = append(containerArgs, "--volume", fmt.Sprintf("%s:%s", host, container))
	}

	// Add environment variables
	for key, value := range config.Env {
		containerArgs = append(containerArgs, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	// Add network mode if specified
	if config.Network != "" {
		containerArgs = append(containerArgs, "--network", config.Network)
	}

	// Add user if specified
	if config.User != "" {
		containerArgs = append(containerArgs, "--user", config.User)
	}

	// Add working directory if specified
	if config.WorkDir != "" {
		containerArgs = append(containerArgs, "--workdir", config.WorkDir)
	}

	// Add any additional arguments
	containerArgs = append(containerArgs, config.AdditionalArgs...)

	// Add the image
	containerArgs = append(containerArgs, config.Image)

	// Add the command and args
	containerArgs = append(containerArgs, command)
	containerArgs = append(containerArgs, args...)

	return containerArgs, nil
}
