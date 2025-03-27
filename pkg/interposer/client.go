package interposer

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ErrNoInitializationResult is returned when the initialization result is nil.
var ErrNoInitializationResult = errors.New("initialization result is nil")

// Initialize initializes an MCP client.
func Initialize(
	ctx context.Context,
	mcpClient client.MCPClient,
	info mcp.Implementation,
	name string,
) (*mcp.InitializeResult, error) {
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = info

	result, err := mcpClient.Initialize(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	// Check if the result is nil
	if result == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoInitializationResult, name)
	}

	return result, nil
}
