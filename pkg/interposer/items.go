package interposer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yosida95/uritemplate/v3"

	"github.com/jkoelker/posuer/pkg/config"
)

// ErrInvalidHandler is returned when a handler is not valid for the item type.
var ErrInvalidHandler = errors.New("invalid handler for item type")

type Resource interface {
	mcp.Resource | mcp.ResourceTemplate
}

type Items interface {
	mcp.Tool | mcp.Prompt | Resource
}

func itemName[Item Items](item Item) string {
	switch value := any(item).(type) {
	case mcp.Tool:
		return value.Name

	case mcp.Prompt:
		return value.Name

	case mcp.Resource:
		return value.Name

	case mcp.ResourceTemplate:
		return value.Name

	default:
		log.Printf("Unknown item type: %T", value)
	}

	return "unknown"
}

type Handlers interface {
	server.ToolHandlerFunc |
		server.PromptHandlerFunc |
		server.ResourceHandlerFunc |
		server.ResourceTemplateHandlerFunc
}

func register[Item Items, Handler Handlers](
	interposer *Interposer,
	name string,
	item Item,
	handler Handler,
) error {
	switch value := any(item).(type) {
	case mcp.Tool:
		handlerFunc, ok := any(handler).(server.ToolHandlerFunc)
		if !ok {
			return fmt.Errorf("%w: tool %s: have %T", ErrInvalidHandler, name, handler)
		}

		interposer.RegisterTool(name, value, handlerFunc)
	case mcp.Prompt:
		handlerFunc, ok := any(handler).(server.PromptHandlerFunc)
		if !ok {
			return fmt.Errorf("%w: prompt %s: have %T", ErrInvalidHandler, name, handler)
		}

		interposer.RegisterPrompt(name, value, handlerFunc)
	case mcp.Resource:
		handlerFunc, ok := any(handler).(server.ResourceHandlerFunc)
		if !ok {
			return fmt.Errorf("%w: resource %s: have %T", ErrInvalidHandler, name, handler)
		}

		interposer.RegisterResource(name, value, handlerFunc)
	case mcp.ResourceTemplate:
		handlerFunc, ok := any(handler).(server.ResourceTemplateHandlerFunc)
		if !ok {
			return fmt.Errorf("%w: resource template %s: have %T", ErrInvalidHandler, name, handler)
		}

		interposer.RegisterResourceTemplate(name, value, handlerFunc)
	default:
		return fmt.Errorf("%w: %s: have %T", ErrInvalidHandler, name, handler)
	}

	return nil
}

func addItems[Item Items, Handler Handlers](
	ctx context.Context,
	interposer *Interposer,
	name string,
	list func(ctx context.Context, cursor string) ([]Item, string, error),
	handler func(item Item) Handler,
) error {
	var cursor string

	for {
		items, next, err := list(ctx, cursor)
		if err != nil {
			return fmt.Errorf("failed to list items: %w", err)
		}

		for _, item := range items {
			transformed := transform(name, item)
			if err := register(interposer, name, transformed, handler(item)); err != nil {
				log.Printf("Failed to register item %s: %v", itemName(item), err)

				continue
			}
		}

		if next == "" {
			break
		}

		cursor = next
	}

	return nil
}

func transform[Item Items](name string, item Item) Item {
	switch value := any(item).(type) {
	case mcp.Tool:
		value.Name = fmt.Sprintf("%s-%s", name, value.Name)

		if i, ok := any(value).(Item); ok {
			return i
		}

		log.Printf("Failed to cast tool to Item: %T", value)

	case mcp.Prompt:
		value.Name = fmt.Sprintf("%s.%s", name, value.Name)

		if i, ok := any(value).(Item); ok {
			return i
		}

		log.Printf("Failed to cast prompt to Item: %T", value)

	case mcp.Resource:
		value.Name = fmt.Sprintf("%s-%s", name, value.Name)
		value.URI = fmt.Sprintf("%s+%s", name, value.URI)

		if i, ok := any(value).(Item); ok {
			return i
		}

		log.Printf("Failed to cast resource to Item: %T", value)

	case mcp.ResourceTemplate:
		value.Name = fmt.Sprintf("%s-%s", name, value.Name)

		template := fmt.Sprintf("%s+%s", name, value.URITemplate.Raw())
		value.URITemplate = &mcp.URITemplate{
			Template: uritemplate.MustNew(template),
		}

		if i, ok := any(value).(Item); ok {
			return i
		}

		log.Printf("Failed to cast resource template to Item: %T", value)

	default:
		log.Printf("Unknown item type: %T", value)

		return item
	}

	return item
}

// addClientItems is a helper function to add the client's items of any type to our server.
func addClientItems[Item Items, Handler Handlers](
	ctx context.Context,
	interposer *Interposer,
	cfg config.Server,
	capability config.CapabilityType,
	request func(ctx context.Context, cursor string) ([]Item, string, error),
	create func(Item) Handler,
) error {
	list := func(ctx context.Context, cursor string) ([]Item, string, error) {
		items, next, err := request(ctx, cursor)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list %s: %w", capability, err)
		}

		// Filter items based on configuration
		var filtered []Item

		for _, item := range items {
			if cfg.Enabled(capability, itemName(item)) {
				filtered = append(filtered, item)
			} else {
				log.Printf(
					"%s %s from %s is disabled by configuration",
					capability,
					itemName(item),
					cfg.Name,
				)
			}
		}

		return filtered, next, nil
	}

	// Create the handler function
	return addItems(ctx, interposer, cfg.Name, list, create)
}

// addClientTools adds the client's tools to our server.
func (i *Interposer) addClientTools(
	ctx context.Context,
	mcpClient client.MCPClient,
	cfg config.Server,
) error {
	request := func(ctx context.Context, cursor string) ([]mcp.Tool, string, error) {
		req := mcp.ListToolsRequest{}
		if cursor != "" {
			req.Params.Cursor = mcp.Cursor(cursor)
		}

		result, err := mcpClient.ListTools(ctx, req)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list tools: %w", err)
		}

		return result.Tools, string(result.NextCursor), nil
	}

	create := func(tool mcp.Tool) server.ToolHandlerFunc {
		// NOTE until upstream merges the `omitempty` fix.
		if tool.InputSchema.Properties == nil {
			tool.InputSchema.Properties = make(map[string]any)
		}

		return handleTool(tool, mcpClient)
	}

	return addClientItems(
		ctx,
		i,
		cfg,
		config.CapabilityTypeTool,
		request,
		create,
	)
}

// addClientPrompts adds the client's prompts to our server.
func (i *Interposer) addClientPrompts(
	ctx context.Context,
	mcpClient client.MCPClient,
	cfg config.Server,
) error {
	request := func(ctx context.Context, cursor string) ([]mcp.Prompt, string, error) {
		req := mcp.ListPromptsRequest{}
		if cursor != "" {
			req.Params.Cursor = mcp.Cursor(cursor)
		}

		result, err := mcpClient.ListPrompts(ctx, req)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list prompts: %w", err)
		}

		return result.Prompts, string(result.NextCursor), nil
	}

	create := func(prompt mcp.Prompt) server.PromptHandlerFunc {
		return handlePrompt(prompt, mcpClient)
	}

	return addClientItems(
		ctx,
		i,
		cfg,
		config.CapabilityTypePrompt,
		request,
		create,
	)
}

// addClientResources adds the client's resources to our server.
func (i *Interposer) addClientResources(
	ctx context.Context,
	mcpClient client.MCPClient,
	cfg config.Server,
) error {
	request := func(ctx context.Context, cursor string) ([]mcp.Resource, string, error) {
		req := mcp.ListResourcesRequest{}
		if cursor != "" {
			req.Params.Cursor = mcp.Cursor(cursor)
		}

		result, err := mcpClient.ListResources(ctx, req)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list resources: %w", err)
		}

		return result.Resources, string(result.NextCursor), nil
	}

	create := func(resource mcp.Resource) server.ResourceHandlerFunc {
		return handleResource(cfg.Name, resource, mcpClient)
	}

	return addClientItems(
		ctx,
		i,
		cfg,
		config.CapabilityTypeResource,
		request,
		create,
	)
}

// addClientResourceTemplates adds the client's resource templates to our server.
func (i *Interposer) addClientResourceTemplates(
	ctx context.Context,
	mcpClient client.MCPClient,
	cfg config.Server,
) error {
	request := func(ctx context.Context, cursor string) ([]mcp.ResourceTemplate, string, error) {
		req := mcp.ListResourceTemplatesRequest{}
		if cursor != "" {
			req.Params.Cursor = mcp.Cursor(cursor)
		}

		result, err := mcpClient.ListResourceTemplates(ctx, req)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list resource templates: %w", err)
		}

		return result.ResourceTemplates, string(result.NextCursor), nil
	}

	create := func(template mcp.ResourceTemplate) server.ResourceTemplateHandlerFunc {
		return handleResource(cfg.Name, template, mcpClient)
	}

	return addClientItems(
		ctx,
		i,
		cfg,
		config.CapabilityTypeTemplate,
		request,
		create,
	)
}

func handleTool(
	tool mcp.Tool,
	mcpClient client.MCPClient,
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(
		ctx context.Context,
		request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		request.Params.Name = tool.Name

		result, err := mcpClient.CallTool(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to call tool: %w", err)
		}

		return result, nil
	}
}

func handlePrompt(
	prompt mcp.Prompt,
	mcpClient client.MCPClient,
) func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return func(
		ctx context.Context,
		request mcp.GetPromptRequest,
	) (*mcp.GetPromptResult, error) {
		request.Params.Name = prompt.Name

		result, err := mcpClient.GetPrompt(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to read prompt: %w", err)
		}

		return result, nil
	}
}

func handleResource[Item Resource](
	name string,
	resource Item,
	mcpClient client.MCPClient,
) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(
		ctx context.Context,
		request mcp.ReadResourceRequest,
	) ([]mcp.ResourceContents, error) {
		switch value := any(resource).(type) {
		case mcp.Resource:
			request.Params.URI = value.URI
		case mcp.ResourceTemplate:
			request.Params.URI = strings.TrimPrefix(request.Params.URI, name+"+")
		}

		result, err := mcpClient.ReadResource(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to read resource: %w", err)
		}

		return result.Contents, nil
	}
}
