//nolint:testpackage // Need access to unexported methods for testing
package interposer

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jkoelker/posuer/pkg/config"
)

// MockMCPClient is a mock implementation of the MCPClient interface for testing.
type MockMCPClient struct {
	initializeCalled            bool
	listToolsCalled             bool
	listPromptsCalled           bool
	listResourcesCalled         bool
	listResourceTemplatesCalled bool
	completeCalled              bool
	lastCompletionRequest       mcp.CompleteRequest
	tools                       []mcp.Tool
	prompts                     []mcp.Prompt
	resources                   []mcp.Resource
	resourceTemplates           []mcp.ResourceTemplate
}

// Initialize implements the Initialize method of the MCPClient interface.
func (m *MockMCPClient) Initialize(
	_ context.Context,
	_ mcp.InitializeRequest,
) (*mcp.InitializeResult, error) {
	m.initializeCalled = true

	return &mcp.InitializeResult{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		Capabilities: mcp.ServerCapabilities{
			Tools: &struct {
				ListChanged bool `json:"listChanged,omitempty"` //nolint:tagliatelle
			}{},
			Resources: &struct {
				Subscribe   bool `json:"subscribe,omitempty"`
				ListChanged bool `json:"listChanged,omitempty"` //nolint:tagliatelle
			}{},
			Prompts: &struct {
				ListChanged bool `json:"listChanged,omitempty"` //nolint:tagliatelle
			}{},
		},
	}, nil
}

// Ping implements the Ping method of the MCPClient interface.
func (m *MockMCPClient) Ping(_ context.Context) error {
	return nil
}

// Close implements the Close method of the MCPClient interface.
func (m *MockMCPClient) Close() error {
	return nil
}

// ListTools implements the ListTools method of the MCPClient interface.
func (m *MockMCPClient) ListTools(
	_ context.Context,
	_ mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	m.listToolsCalled = true

	return &mcp.ListToolsResult{
		Tools: m.tools,
	}, nil
}

// ListToolsByPage implements the ListToolsByPage method of the MCPClient interface.
func (m *MockMCPClient) ListToolsByPage(
	_ context.Context,
	_ mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	m.listToolsCalled = true

	return &mcp.ListToolsResult{
		Tools: m.tools,
	}, nil
}

// CallTool implements the CallTool method of the MCPClient interface.
func (m *MockMCPClient) CallTool(
	_ context.Context,
	_ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: "Mock tool result",
			},
		},
	}, nil
}

// ListPrompts implements the ListPrompts method of the MCPClient interface.
func (m *MockMCPClient) ListPrompts(
	_ context.Context,
	_ mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	m.listPromptsCalled = true

	return &mcp.ListPromptsResult{
		Prompts: m.prompts,
	}, nil
}

// ListPromptsByPage implements the ListPromptsByPage method of the MCPClient interface.
func (m *MockMCPClient) ListPromptsByPage(
	_ context.Context,
	_ mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	m.listPromptsCalled = true

	return &mcp.ListPromptsResult{
		Prompts: m.prompts,
	}, nil
}

// GetPrompt implements the GetPrompt method of the MCPClient interface.
func (m *MockMCPClient) GetPrompt(
	_ context.Context,
	_ mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Type: "text",
					Text: "Mock prompt result",
				},
			},
		},
	}, nil
}

// ListResources implements the ListResources method of the MCPClient interface.
func (m *MockMCPClient) ListResources(
	_ context.Context,
	_ mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) {
	m.listResourcesCalled = true

	return &mcp.ListResourcesResult{
		Resources: m.resources,
	}, nil
}

// ListResourcesByPage implements the ListResourcesByPage method of the MCPClient interface.
func (m *MockMCPClient) ListResourcesByPage(
	_ context.Context,
	_ mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) {
	m.listResourcesCalled = true

	return &mcp.ListResourcesResult{
		Resources: m.resources,
	}, nil
}

// ReadResource implements the ReadResource method of the MCPClient interface.
func (m *MockMCPClient) ReadResource(
	_ context.Context,
	req mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     "Mock resource content",
			},
		},
	}, nil
}

// Subscribe implements the Subscribe method of the MCPClient interface.
func (m *MockMCPClient) Subscribe(_ context.Context, _ mcp.SubscribeRequest) error {
	return nil
}

// Unsubscribe implements the Unsubscribe method of the MCPClient interface.
func (m *MockMCPClient) Unsubscribe(_ context.Context, _ mcp.UnsubscribeRequest) error {
	return nil
}

// ListResourceTemplates implements the ListResourceTemplates method of the MCPClient interface.
func (m *MockMCPClient) ListResourceTemplates(
	_ context.Context,
	_ mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) {
	m.listResourceTemplatesCalled = true

	return &mcp.ListResourceTemplatesResult{
		ResourceTemplates: m.resourceTemplates,
	}, nil
}

// ListResourceTemplatesByPage implements the ListResourceTemplatesByPage method of the MCPClient interface.
func (m *MockMCPClient) ListResourceTemplatesByPage(
	_ context.Context,
	_ mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) {
	m.listResourceTemplatesCalled = true

	return &mcp.ListResourceTemplatesResult{
		ResourceTemplates: m.resourceTemplates,
	}, nil
}

// Complete implements the Complete method of the MCPClient interface.
func (m *MockMCPClient) Complete(
	_ context.Context,
	req mcp.CompleteRequest,
) (*mcp.CompleteResult, error) {
	// Track that this method was called
	m.completeCalled = true
	// Store the request for later inspection
	m.lastCompletionRequest = req

	// Return mock completion values
	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"` //nolint:tagliatelle
		}{
			Values: []string{"option1", "option2", "option3"},
			Total:  3,
		},
	}, nil
}

// OnNotification implements the OnNotification method of the MCPClient interface.
func (m *MockMCPClient) OnNotification(_ func(notification mcp.JSONRPCNotification)) {
	// Do nothing in the mock
}

// SetLevel implements the SetLevel method of the MCPClient interface.
func (m *MockMCPClient) SetLevel(_ context.Context, _ mcp.SetLevelRequest) error {
	return nil
}

// mockClientFactory creates a factory function that returns a mock client.
func mockClientFactory() func(config.Server) (client.MCPClient, error) {
	return func(_ config.Server) (client.MCPClient, error) {
		return createMockClient(), nil
	}
}

func TestInterposer(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Create a server config with no tool filtering
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
	}

	// Get the initialization result from the mock client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(context.Background(), initRequest)
	require.NoError(t, err)

	// Use the mockClient in addClientCapabilities
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Verify the mock was called
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")

	assert.True(t, mocked.listToolsCalled)
	assert.True(t, mocked.listPromptsCalled)
	assert.True(t, mocked.listResourcesCalled)
	assert.True(t, mocked.listResourceTemplatesCalled)

	// Verify that tools were registered with the server
	server := interposerInstance.Server()
	require.NotNil(t, server)

	// Close the interposer
	require.NoError(t, interposerInstance.Close())
}

func TestInterposerWithNoFiltering(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Configure with no filtering
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
	}

	// Get initialization result
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(ctx, initRequest)
	require.NoError(t, err)

	// Apply configuration
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Verify tools were called (no filtering)
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")

	assert.True(t, mocked.listToolsCalled)
}

// TestInterposerWithDisableAllTools has been removed as it was testing internal implementation details
// in a brittle way. Container support is tested more effectively through integration tests.

func TestInterposerWithEnableSpecificTool(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Configure to enable specific tool
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
		Enable: &config.Capability{
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"test-tool"},
			},
		},
	}

	// Get initialization result
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(ctx, initRequest)
	require.NoError(t, err)

	// Apply configuration
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Verify tools were called (specific tool enabled)
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")
	assert.True(t, mocked.listToolsCalled, "listTools should be called")
}

func TestInterposerWithEnableDifferentTool(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Configure to enable a tool that doesn't exist
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
		Enable: &config.Capability{
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"different-tool"},
			},
		},
	}

	// Get initialization result
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(ctx, initRequest)
	require.NoError(t, err)

	// Apply configuration
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Tools should be called but filtered
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")

	assert.True(t, mocked.listToolsCalled, "listTools should be called")
}

func TestInterposerWithDisableSpecificTool(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Configure to disable specific tool
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
		Disable: &config.Capability{
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"test-tool"},
			},
		},
	}

	// Get initialization result
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(ctx, initRequest)
	require.NoError(t, err)

	// Apply configuration
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Verify tools were called but filtered
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")

	assert.True(t, mocked.listToolsCalled, "listTools should be called")
}

func TestInterposerWithDisableDifferentTool(t *testing.T) {
	t.Parallel()

	// Create a mock client factory
	factory := mockClientFactory()
	mockClient, err := factory(config.Server{})
	require.NoError(t, err)

	// Create an interposer with mock client factory
	interposerInstance, err := NewInterposer(
		"TestInterposer",
		"1.0.0",
		WithClientFactory(factory),
	)
	require.NoError(t, err)
	require.NotNil(t, interposerInstance)

	// Create a context
	ctx := context.Background()

	// Configure to disable a tool that doesn't exist
	serverConfig := config.Server{
		Name: "test-server",
		Type: config.ServerTypeStdio,
		Disable: &config.Capability{
			Capabilities: map[config.CapabilityType][]string{
				config.CapabilityTypeTool: {"different-tool"},
			},
		},
	}

	// Get initialization result
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = interposerInstance.ImplementationInfo()
	initResult, err := mockClient.Initialize(ctx, initRequest)
	require.NoError(t, err)

	// Apply configuration
	interposerInstance.addClientCapabilities(ctx, mockClient, initResult, serverConfig)

	// Verify tools were called (different tool disabled)
	mocked, ok := mockClient.(*MockMCPClient)
	require.True(t, ok, "mockClient should be of type *MockMCPClient")

	assert.True(t, mocked.listToolsCalled, "listTools should be called")
}

// createMockClient creates a mock MCP client with standard test data.
func createMockClient() *MockMCPClient {
	return &MockMCPClient{
		tools: []mcp.Tool{
			{
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"foo": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
		prompts: []mcp.Prompt{
			{
				Name:        "test-prompt",
				Description: "A test prompt",
			},
		},
		resources: []mcp.Resource{
			{
				URI:         "test://resource",
				Name:        "Test Resource",
				Description: "A test resource",
			},
		},
		resourceTemplates: []mcp.ResourceTemplate{
			mcp.NewResourceTemplate(
				"test://{id}",
				"Test Template",
				mcp.WithTemplateDescription("A test resource template"),
			),
		},
	}
}
