package isolate

import (
	"errors"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/client"

	"github.com/jkoelker/posuer/pkg/config"
)

// ErrUnsupportedServerType is returned when an unsupported server type is encountered.
var ErrUnsupportedServerType = errors.New("unsupported server type")

// Noop implements the Isolator interface without isolation.
type Noop struct{}

// NewNoop creates a new Noop.
func NewNoop() *Noop {
	return &Noop{}
}

// Isolate creates an MCP client without isolation.
func (n *Noop) Isolate(cfg config.Server) (client.MCPClient, error) {
	var err error

	var mcpClient client.MCPClient

	switch cfg.ServerType() {
	case config.ServerTypeStdio:
		// Convert env map to env slice
		envSlice := make([]string, 0, len(cfg.Env))
		for k, v := range cfg.Env {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}

		log.Printf(
			"Creating Stdio MCP client with command: %s, args: %v, env: %v",
			cfg.Command,
			cfg.Args,
			cfg.Env,
		)

		mcpClient, err = client.NewStdioMCPClient(cfg.Command, envSlice, cfg.Args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Stdio MCP client: %w", err)
		}
	case config.ServerTypeSSE:
		mcpClient, err = client.NewSSEMCPClient(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSE MCP client: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedServerType, cfg.ServerType())
	}

	return mcpClient, nil
}
