package isolate_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jkoelker/posuer/pkg/config"
	"github.com/jkoelker/posuer/pkg/isolate"
)

func skipIfNoContainerRuntime(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath(isolate.PodmanRuntime); err != nil {
		if _, err := exec.LookPath(isolate.DockerRuntime); err != nil {
			t.Skip("no container runtime available")
		}
	}
}

func TestIsContainerCommand(t *testing.T) {
	t.Parallel()

	assert.True(t, isolate.IsContainerCommand("docker"))
	assert.True(t, isolate.IsContainerCommand("podman"))
	assert.True(t, isolate.IsContainerCommand("/usr/bin/docker"))
	assert.True(t, isolate.IsContainerCommand("/usr/bin/podman"))
	assert.False(t, isolate.IsContainerCommand("echo"))
	assert.False(t, isolate.IsContainerCommand("/bin/echo"))
}

func TestBasicContainerCommand(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image: "alpine:latest",
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "--interactive")
	assert.Contains(t, args, "alpine:latest")
	assert.Contains(t, args, "echo")
	assert.Contains(t, args, "hello")
}

func TestContainerWithVolumes(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image: "alpine:latest",
		Volumes: map[string]string{
			"/host": "/container",
		},
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assertFlagWithValue(t, args, "--volume", "/host:/container", "Volume argument not found")
}

func TestContainerWithEnv(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image: "alpine:latest",
		Env: map[string]string{
			"FOO": "bar",
		},
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assertFlagWithValue(t, args, "--env", "FOO=bar", "Environment argument not found")
}

func TestContainerWithNetwork(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image:   "alpine:latest",
		Network: "host",
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assertFlagWithValue(t, args, "--network", "host", "Network argument not found")
}

func TestContainerWithUser(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image: "alpine:latest",
		User:  "nobody",
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assertFlagWithValue(t, args, "--user", "nobody", "User argument not found")
}

func TestContainerWithWorkDir(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image:   "alpine:latest",
		WorkDir: "/app",
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assertFlagWithValue(t, args, "--workdir", "/app", "Working directory argument not found")
}

func TestContainerWithAdditionalArgs(t *testing.T) {
	t.Parallel()

	skipIfNoContainerRuntime(t)

	container := &config.Container{
		Image:          "alpine:latest",
		AdditionalArgs: []string{"--cap-add=SYS_ADMIN"},
	}

	args, err := isolate.ContainerCommand("echo", []string{"hello"}, container)
	require.NoError(t, err)
	assert.Contains(t, args, "--cap-add=SYS_ADMIN", "Additional argument not found")
}

func assertFlagWithValue(t *testing.T, args []string, flag, expectedValue, message string) {
	t.Helper()

	found := false

	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			if strings.Contains(args[i+1], expectedValue) {
				found = true

				break
			}
		}
	}

	assert.True(t, found, message)
}
