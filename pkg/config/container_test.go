package config_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "sigs.k8s.io/yaml/goyaml.v3"

	"github.com/jkoelker/posuer/pkg/config"
)

func TestContainerClone(t *testing.T) {
	t.Parallel()

	t.Run("NilContainer", func(t *testing.T) {
		t.Parallel()

		var nilContainer *config.Container
		clone := nilContainer.Clone()
		assert.Nil(t, clone, "Clone of nil container should be nil")
	})

	t.Run("EmptyContainer", func(t *testing.T) {
		t.Parallel()

		emptyContainer := &config.Container{}
		clone := emptyContainer.Clone()
		assert.NotNil(t, clone, "Clone of empty container should not be nil")
		assert.Equal(t, emptyContainer, clone, "Clone of empty container should match original")
	})

	t.Run("PopulatedContainer", func(t *testing.T) {
		t.Parallel()

		container := &config.Container{
			Image: "alpine:latest",
			Volumes: map[string]string{
				"/host/path": "/container/path",
			},
			Env: map[string]string{
				"KEY": "VALUE",
			},
			Network:        "host",
			User:           "user",
			WorkDir:        "/work",
			AdditionalArgs: []string{"--no-cache"},
		}

		clone := container.Clone()

		// Check for deep equality
		assert.Equal(t, container, clone, "Clone should match original")

		// Check that it's actually a deep copy by modifying the clone
		clone.Image = "modified:image"
		clone.Volumes["/new/path"] = "/new/container/path"
		clone.Env["NEW_KEY"] = "NEW_VALUE"
		clone.AdditionalArgs[0] = "--modified"

		// Original should remain unchanged
		assert.NotEqual(t, container.Image, clone.Image, "Modifying clone should not affect original image")
		_, hasNewPath := container.Volumes["/new/path"]
		assert.False(t, hasNewPath, "Modifying clone volumes should not affect original")

		_, hasNewKey := container.Env["NEW_KEY"]
		assert.False(t, hasNewKey, "Modifying clone env should not affect original")

		assert.NotEqual(t, container.AdditionalArgs[0], clone.AdditionalArgs[0],
			"Modifying clone args should not affect original")
	})
}

func TestContainerUnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("BooleanFalse", func(t *testing.T) {
		t.Parallel()

		yamlData := `false`

		var container config.Container
		err := yaml.Unmarshal([]byte(yamlData), &container)

		require.NoError(t, err, "UnmarshalYAML() should not return an error")
		assert.Empty(t, container.Image, "Container should be marked as disabled with empty image")
		assert.True(t, container.IsDisabled(), "Container should be disabled")
		assert.False(t, container.IsConfigured(), "Container should not be configured")
	})

	t.Run("BooleanTrue", func(t *testing.T) {
		t.Parallel()

		yamlData := `true`

		var container config.Container
		err := yaml.Unmarshal([]byte(yamlData), &container)

		require.Error(t, err, "UnmarshalYAML() should return an error for boolean true")
	})

	t.Run("SimpleStringImage", func(t *testing.T) {
		t.Parallel()

		yamlData := `"alpine:latest"`
		want := &config.Container{
			Image: "alpine:latest",
		}

		var container config.Container
		err := yaml.Unmarshal([]byte(yamlData), &container)

		require.NoError(t, err, "UnmarshalYAML() should not return an error")
		assert.Equal(t, want, &container, "UnmarshalYAML() result should match expected")
	})

	t.Run("FullContainerConfig", func(t *testing.T) {
		t.Parallel()

		yamlData := `
image: nginx:alpine
volumes:
  /host/path: /container/path
  /another/path: /another/container/path
env:
  KEY1: VALUE1
  KEY2: VALUE2
network: host
user: nginx
workdir: /usr/share/nginx/html
args:
  - --no-cache
  - --rm
`
		want := &config.Container{
			Image: "nginx:alpine",
			Volumes: map[string]string{
				"/host/path":    "/container/path",
				"/another/path": "/another/container/path",
			},
			Env: map[string]string{
				"KEY1": "VALUE1",
				"KEY2": "VALUE2",
			},
			Network:        "host",
			User:           "nginx",
			WorkDir:        "/usr/share/nginx/html",
			AdditionalArgs: []string{"--no-cache", "--rm"},
		}

		var container config.Container
		err := yaml.Unmarshal([]byte(yamlData), &container)

		require.NoError(t, err, "UnmarshalYAML() should not return an error")
		assert.Equal(t, want, &container, "UnmarshalYAML() result should match expected")
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		t.Parallel()

		yamlData := `{image: "broken"`

		var container config.Container
		err := yaml.Unmarshal([]byte(yamlData), &container)

		require.Error(t, err, "UnmarshalYAML() should return an error for invalid YAML")
	})
}

func TestContainerUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("BooleanFalse", func(t *testing.T) {
		t.Parallel()

		jsonData := `false`

		var container config.Container
		err := json.Unmarshal([]byte(jsonData), &container)

		require.NoError(t, err, "UnmarshalJSON() should not return an error")
		assert.Empty(t, container.Image, "Container should be marked as disabled with empty image")
		assert.True(t, container.IsDisabled(), "Container should be disabled")
		assert.False(t, container.IsConfigured(), "Container should not be configured")
	})

	t.Run("BooleanTrue", func(t *testing.T) {
		t.Parallel()

		jsonData := `true`

		var container config.Container
		err := json.Unmarshal([]byte(jsonData), &container)

		require.Error(t, err, "UnmarshalJSON() should return an error for boolean true")
	})

	t.Run("SimpleStringImage", func(t *testing.T) {
		t.Parallel()

		jsonData := `"alpine:latest"`
		want := &config.Container{
			Image: "alpine:latest",
		}

		var container config.Container
		err := json.Unmarshal([]byte(jsonData), &container)

		require.NoError(t, err, "UnmarshalJSON() should not return an error")
		assert.Equal(t, want, &container, "UnmarshalJSON() result should match expected")
	})

	t.Run("FullContainerConfig", func(t *testing.T) {
		t.Parallel()

		jsonData := `{
  "image": "nginx:alpine",
  "volumes": {
    "/host/path": "/container/path",
    "/another/path": "/another/container/path"
  },
  "env": {
    "KEY1": "VALUE1",
    "KEY2": "VALUE2"
  },
  "network": "host",
  "user": "nginx",
  "workdir": "/usr/share/nginx/html",
  "args": ["--no-cache", "--rm"]
}`
		want := &config.Container{
			Image: "nginx:alpine",
			Volumes: map[string]string{
				"/host/path":    "/container/path",
				"/another/path": "/another/container/path",
			},
			Env: map[string]string{
				"KEY1": "VALUE1",
				"KEY2": "VALUE2",
			},
			Network:        "host",
			User:           "nginx",
			WorkDir:        "/usr/share/nginx/html",
			AdditionalArgs: []string{"--no-cache", "--rm"},
		}

		var container config.Container
		err := json.Unmarshal([]byte(jsonData), &container)

		require.NoError(t, err, "UnmarshalJSON() should not return an error")
		assert.Equal(t, want, &container, "UnmarshalJSON() result should match expected")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"image": "broken"`

		var container config.Container
		err := json.Unmarshal([]byte(jsonData), &container)

		require.Error(t, err, "UnmarshalJSON() should return an error for invalid JSON")
	})
}

func TestServerWithContainer(t *testing.T) {
	t.Parallel()

	t.Run("ServerWithDisabledContainer", func(t *testing.T) {
		t.Parallel()

		yamlData := `
name: test-server
type: stdio
command: npx
args:
  - hello
container: false
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlData), &server)

		require.NoError(t, err, "Unmarshal() should not return an error")
		require.NotNil(t, server.Container, "Server.Container should not be nil")
		assert.True(t, server.Container.IsDisabled(), "Container should be disabled")
	})

	t.Run("ServerWithSimpleContainerImage", func(t *testing.T) {
		t.Parallel()

		yamlData := `
name: test-server
type: stdio
command: echo
args:
  - hello
container: alpine:latest
`
		want := &config.Server{
			Name:    "test-server",
			Type:    config.ServerTypeStdio,
			Command: "echo",
			Args:    []string{"hello"},
			Container: &config.Container{
				Image: "alpine:latest",
			},
		}

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlData), &server)

		require.NoError(t, err, "Unmarshal() should not return an error")

		assert.Equal(t, want.Name, server.Name, "Server.Name should match")
		assert.Equal(t, want.Type, server.Type, "Server.Type should match")
		assert.Equal(t, want.Command, server.Command, "Server.Command should match")
		assert.Equal(t, want.Args, server.Args, "Server.Args should match")

		require.NotNil(t, server.Container, "Server.Container should not be nil")
		assert.Equal(t, want.Container.Image, server.Container.Image,
			"Server.Container.Image should match")
	})

	t.Run("ServerWithDetailedContainerConfig", func(t *testing.T) {
		t.Parallel()

		yamlData := `
name: test-server
type: stdio
command: echo
args:
  - hello
container:
  image: nginx:alpine
  volumes:
    /host/path: /container/path
  env:
    KEY: VALUE
  network: host
`
		want := &config.Server{
			Name:    "test-server",
			Type:    config.ServerTypeStdio,
			Command: "echo",
			Args:    []string{"hello"},
			Container: &config.Container{
				Image: "nginx:alpine",
				Volumes: map[string]string{
					"/host/path": "/container/path",
				},
				Env: map[string]string{
					"KEY": "VALUE",
				},
				Network: "host",
			},
		}

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlData), &server)

		require.NoError(t, err, "Unmarshal() should not return an error")

		assert.Equal(t, want.Name, server.Name, "Server.Name should match")
		assert.Equal(t, want.Type, server.Type, "Server.Type should match")
		assert.Equal(t, want.Command, server.Command, "Server.Command should match")
		assert.Equal(t, want.Args, server.Args, "Server.Args should match")

		require.NotNil(t, server.Container, "Server.Container should not be nil")
		assert.Equal(t, want.Container.Image, server.Container.Image,
			"Server.Container.Image should match")
		assert.Equal(t, want.Container.Volumes, server.Container.Volumes,
			"Server.Container.Volumes should match")
		assert.Equal(t, want.Container.Env, server.Container.Env,
			"Server.Container.Env should match")
		assert.Equal(t, want.Container.Network, server.Container.Network,
			"Server.Container.Network should match")
	})

	t.Run("ServerWithoutContainer", func(t *testing.T) {
		t.Parallel()

		yamlData := `
name: test-server
type: stdio
command: echo
args:
  - hello
`

		var server config.Server
		err := yaml.Unmarshal([]byte(yamlData), &server)

		require.NoError(t, err, "Unmarshal() should not return an error")
		assert.Nil(t, server.Container, "Server.Container should be nil")
	})
}

func TestServerCloneWithContainer(t *testing.T) {
	t.Parallel()

	t.Run("CloneServerWithContainer", func(t *testing.T) {
		t.Parallel()

		// Setup original server with container
		original := config.Server{
			Name:    "test-server",
			Type:    config.ServerTypeStdio,
			Command: "echo",
			Args:    []string{"hello"},
			Env: map[string]string{
				"KEY": "VALUE",
			},
			Container: &config.Container{
				Image: "alpine:latest",
				Volumes: map[string]string{
					"/host/path": "/container/path",
				},
				Env: map[string]string{
					"CONTAINER_KEY": "CONTAINER_VALUE",
				},
				Network: "host",
			},
		}

		// Clone the server
		clone := original.Clone()

		// Test that clone was successful
		assert.Equal(t, original, clone, "Clone should match original")

		// Test deep copy by modifying clone
		clone.Container.Image = "modified:image"
		clone.Container.Volumes["/new/path"] = "/new/container/path"
		clone.Container.Env["NEW_KEY"] = "NEW_VALUE"

		// Original should remain unchanged
		assert.NotEqual(t, original.Container.Image, clone.Container.Image,
			"Modifying clone container should not affect original image")

		_, hasNewPath := original.Container.Volumes["/new/path"]
		assert.False(t, hasNewPath, "Modifying clone container volumes should not affect original")

		_, hasNewKey := original.Container.Env["NEW_KEY"]
		assert.False(t, hasNewKey, "Modifying clone container env should not affect original")
	})
}
