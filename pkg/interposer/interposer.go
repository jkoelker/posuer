package interposer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jkoelker/posuer/pkg/config"
	"github.com/jkoelker/posuer/pkg/isolate"
)

// ErrBackendNotFound is returned when a backend is not found.
var ErrBackendNotFound = errors.New("backend not found")

// Interposer is the core component that bridges between MCP client and server.
type Interposer struct {
	name     string
	version  string
	server   *server.MCPServer
	clients  map[string]client.MCPClient
	mu       sync.RWMutex // protects clients
	registry *CapabilityRegistry
	factory  func(config.Server) (client.MCPClient, error)
}

// WithClientFactory sets a custom client factory for creating MCP clients.
func WithClientFactory(factory func(config.Server) (client.MCPClient, error)) func(*Interposer) error {
	return func(i *Interposer) error {
		i.factory = factory

		return nil
	}
}

// NewInterposer creates a new MCP interposer.
func NewInterposer(name, version string, opts ...func(*Interposer) error) (*Interposer, error) {
	mcpServer := server.NewMCPServer(
		name,
		version,
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
	)

	interposer := &Interposer{
		name:     name,
		version:  version,
		server:   mcpServer,
		clients:  make(map[string]client.MCPClient),
		registry: NewCapabilityRegistry(),
		factory:  isolate.Client,
	}

	for _, opt := range opts {
		if err := opt(interposer); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return interposer, nil
}

// Server returns the underlying MCP server.
func (i *Interposer) Server() *server.MCPServer {
	return i.server
}

// ImplementationInfo returns the implementation information of the interposer.
func (i *Interposer) ImplementationInfo() mcp.Implementation {
	return mcp.Implementation{
		Name:    i.name,
		Version: i.version,
	}
}

// AddBackend adds a backend server to the interposer.
func (i *Interposer) AddBackend(
	ctx context.Context,
	name string,
	cfg config.Server,
) error {
	mcpClient, err := i.factory(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize the client
	result, err := Initialize(ctx, mcpClient, i.ImplementationInfo(), name)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	// Register client's capabilities with our server
	i.addClientCapabilities(ctx, mcpClient, result, cfg)

	// Store the client
	i.mu.Lock()
	i.clients[name] = mcpClient
	i.mu.Unlock()

	return nil
}

// RegisterTool registers a tool and tracks its source.
func (i *Interposer) RegisterTool(
	backendName string,
	tool mcp.Tool,
	handler server.ToolHandlerFunc,
) {
	i.server.AddTool(tool, handler)
	i.registry.AddCapability(backendName, "tool", tool.Name)
}

// RegisterPrompt registers a prompt and tracks its source.
func (i *Interposer) RegisterPrompt(
	backendName string,
	prompt mcp.Prompt,
	handler server.PromptHandlerFunc,
) {
	i.server.AddPrompt(prompt, handler)
	i.registry.AddCapability(backendName, "prompt", prompt.Name)
}

// RegisterResource registers a resource and tracks its source.
func (i *Interposer) RegisterResource(
	backendName string,
	resource mcp.Resource,
	handler server.ResourceHandlerFunc,
) {
	i.server.AddResource(resource, handler)
	i.registry.AddCapability(backendName, "resource", resource.Name)
}

// RegisterResourceTemplate registers a resource template and tracks its source.
func (i *Interposer) RegisterResourceTemplate(
	backendName string,
	template mcp.ResourceTemplate,
	handler server.ResourceTemplateHandlerFunc,
) {
	i.server.AddResourceTemplate(template, handler)
	i.registry.AddCapability(backendName, "template", template.Name)
}

// RemoveTrackedCapabilities removes all capabilities that came from a specific backend
// and sends capability notifications to clients.
func (i *Interposer) RemoveTrackedCapabilities(ctx context.Context, backendName string) {
	// Remove capabilities from registry and get the removal details
	removedByType := i.registry.RemoveBackendCapabilities(backendName)

	if len(removedByType) == 0 {
		return
	}

	// Handle tools (which have a bulk deletion API)
	if toolNames, exists := removedByType["tool"]; exists && len(toolNames) > 0 {
		i.server.DeleteTools(toolNames...)
		i.sendChangeNotification(ctx, "toolsChange")
	}

	// Check if prompts were changed
	if _, promptsChanged := removedByType["prompt"]; promptsChanged {
		i.sendChangeNotification(ctx, "promptsChange")
	}

	// Check if resources or templates were changed
	_, resourcesChanged := removedByType["resource"]
	_, templatesChanged := removedByType["template"]

	if resourcesChanged || templatesChanged {
		i.sendChangeNotification(ctx, "resourcesChange")
	}
}

// extractRawCapabilityNames creates a map of capability names without backend prefixes.
func extractRawCapabilityNames(
	name string,
	capsByType map[string][]string,
	capType string,
) map[string]bool {
	result := make(map[string]bool)

	if capNames, exists := capsByType[capType]; exists {
		for _, capName := range capNames {
			// Strip backend name prefix for comparison
			rawName := capName[len(name)+1:] // +1 for the dot
			result[rawName] = true
		}
	}

	return result
}

// UpdateCapabilityConfig updates the capability configuration for a specific backend
// without restarting the connection. This is useful when only enable/disable lists change.
func (i *Interposer) UpdateCapabilityConfig(
	ctx context.Context,
	name string,
	oldConfig config.Server,
	newConfig config.Server,
) error {
	// Get the client
	i.mu.RLock()
	mcpClient, exists := i.clients[name]
	i.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrBackendNotFound, name)
	}

	// If the new config disables the entire server, remove it
	if newConfig.Disabled() {
		log.Printf("Server %s is now disabled, removing", name)
		i.removeBackend(ctx, name)

		return nil
	}

	// Get current capabilities from the registry
	capsByType := i.registry.GetCapabilitiesForBackend(name)

	// Track changes for notifications
	toolsChanged := i.processDisabledTools(name, capsByType, newConfig)
	promptsChanged := false
	resourcesChanged := false
	templatesChanged := false

	// For capabilities that are now enabled but weren't before, we need to
	// re-initialize the client to get the full list of capabilities
	needsReinit := !config.CompareCapability(oldConfig.Enable, newConfig.Enable) ||
		!config.CompareCapability(oldConfig.Disable, newConfig.Disable)

	// Re-initialize if needed
	if needsReinit {
		toolsChanged, promptsChanged, resourcesChanged, templatesChanged = i.handleReinitCapabilities(
			ctx, name, mcpClient, capsByType, newConfig, toolsChanged, promptsChanged, resourcesChanged, templatesChanged,
		)
	}

	// Send notifications for changes if needed
	if toolsChanged {
		log.Printf("Publishing tools change notification for %s", name)
		i.sendChangeNotification(ctx, "toolsChange")
	}

	if promptsChanged {
		log.Printf("Publishing prompts change notification for %s", name)
		i.sendChangeNotification(ctx, "promptsChange")
	}

	if resourcesChanged || templatesChanged {
		log.Printf("Publishing resources change notification for %s", name)
		i.sendChangeNotification(ctx, "resourcesChange")
	}

	return nil
}

// checkCapabilityChanges checks if a backend has capabilities of specific types.
func checkCapabilityChanges(capsByType map[string][]string) (bool, bool, bool, bool) {
	toolsChanged := false
	promptsChanged := false
	resourcesChanged := false
	templatesChanged := false

	if len(capsByType) > 0 {
		if _, has := capsByType["tool"]; has {
			toolsChanged = true
		}

		if _, has := capsByType["prompt"]; has {
			promptsChanged = true
		}

		if _, has := capsByType["resource"]; has {
			resourcesChanged = true
		}

		if _, has := capsByType["template"]; has {
			templatesChanged = true
		}
	}

	return toolsChanged, promptsChanged, resourcesChanged, templatesChanged
}

// createDefaultOldConfig creates a default config for comparison purposes.
func createDefaultOldConfig(name string, newConfig config.Server) config.Server {
	return config.Server{
		Name:    name,
		Type:    newConfig.Type,
		Command: newConfig.Command,
		Args:    newConfig.Args,
		Env:     newConfig.Env,
		URL:     newConfig.URL,
		Enable:  &config.Capability{},
		Disable: &config.Capability{},
	}
}

// capabilityChanges tracks changes to different capability types.
type capabilityChanges struct {
	toolsChanged     bool
	promptsChanged   bool
	resourcesChanged bool
	templatesChanged bool
}

// updateChanges updates capability change flags based on new changes.
func (c *capabilityChanges) updateChanges(tc, pc, rc, tec bool) {
	c.toolsChanged = c.toolsChanged || tc
	c.promptsChanged = c.promptsChanged || pc
	c.resourcesChanged = c.resourcesChanged || rc
	c.templatesChanged = c.templatesChanged || tec
}

// Reconfigure updates the interposer with a new set of server configurations.
func (i *Interposer) Reconfigure(ctx context.Context, serverConfigs []config.Server) error {
	// Collect current backends and prepare new config map
	currentBackends := i.getCurrentBackends()
	newConfigMap := prepareNewConfigMap(serverConfigs)

	// Track changes for notifications
	changes := &capabilityChanges{}

	// Process backends in three phases: removed, updated, and new
	i.processRemovedBackends(ctx, currentBackends, newConfigMap, changes)
	i.processUpdatedBackends(ctx, currentBackends, newConfigMap, changes)
	i.processNewBackends(ctx, newConfigMap, changes)

	// Send notifications for all changes
	i.sendNotifications(ctx,
		changes.toolsChanged,
		changes.promptsChanged,
		changes.resourcesChanged,
		changes.templatesChanged)

	return nil
}

// Close closes all connections.
func (i *Interposer) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	for name, client := range i.clients {
		if err := client.Close(); err != nil {
			log.Printf("Error closing client %s: %v", name, err)
		}
	}

	return nil
}

// addClientCapabilities registers all capabilities from the client with our server.
func (i *Interposer) addClientCapabilities(
	ctx context.Context,
	mcpClient client.MCPClient,
	result *mcp.InitializeResult,
	cfg config.Server,
) {
	// Check if the entire server is disabled
	if cfg.Disabled() {
		log.Printf("Server %s is disabled by configuration", cfg.Name)

		return
	}

	// Add tools if supported and not disabled
	if result.Capabilities.Tools != nil {
		if err := i.addClientTools(ctx, mcpClient, cfg); err != nil {
			log.Printf("Warning: failed to add tools from %s: %v", cfg.Name, err)
		}
	}

	// Add prompts if supported
	if result.Capabilities.Prompts != nil {
		if err := i.addClientPrompts(ctx, mcpClient, cfg); err != nil {
			log.Printf("Warning: failed to add prompts from %s: %v", cfg.Name, err)
		}
	}

	// Add resources if supported
	if result.Capabilities.Resources != nil {
		if err := i.addClientResources(ctx, mcpClient, cfg); err != nil {
			log.Printf("Warning: failed to add resources from %s: %v", cfg.Name, err)
		}

		if err := i.addClientResourceTemplates(ctx, mcpClient, cfg); err != nil {
			log.Printf("Warning: failed to add resource templates from %s: %v", cfg.Name, err)
		}
	}
}

// getCurrentBackends returns a map of current backend names.
func (i *Interposer) getCurrentBackends() map[string]bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	currentBackends := make(map[string]bool)
	for name := range i.clients {
		currentBackends[name] = true
	}

	return currentBackends
}

// handleReinitCapabilities handles the re-initialization of capabilities for a backend.
func (i *Interposer) handleReinitCapabilities(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
	toolsChanged, promptsChanged, resourcesChanged, templatesChanged bool,
) (bool, bool, bool, bool) {
	log.Printf("Capability configuration changed for backend %s, re-initializing", name)

	// Initialize the client to get capability info
	initResult, err := Initialize(ctx, mcpClient, i.ImplementationInfo(), name)
	if err != nil {
		log.Printf("Warning: failed to re-initialize client: %v", err)

		return toolsChanged, promptsChanged, resourcesChanged, templatesChanged
	}

	// Process tools if supported
	if initResult.Capabilities.Tools != nil {
		toolsChanged = i.handleToolReinit(ctx, name, mcpClient, capsByType, newConfig, toolsChanged)
	}

	// Process prompts if supported
	if initResult.Capabilities.Prompts != nil {
		promptsChanged = i.handlePromptReinit(ctx, name, mcpClient, capsByType, newConfig, promptsChanged)
	}

	// Process resources if supported
	if initResult.Capabilities.Resources != nil {
		resourcesChanged, templatesChanged = i.handleResourceReinit(
			ctx, name, mcpClient, capsByType, newConfig, resourcesChanged, templatesChanged,
		)
	}

	return toolsChanged, promptsChanged, resourcesChanged, templatesChanged
}

// handleToolReinit processes tools during re-initialization.
func (i *Interposer) handleToolReinit(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
	toolsChanged bool,
) bool {
	currentTools := extractRawCapabilityNames(name, capsByType, "tool")

	changed, err := i.processNewTools(ctx, name, mcpClient, currentTools, newConfig)
	if err != nil {
		log.Printf("Warning: error processing tools: %v", err)
	}

	return toolsChanged || changed
}

// handlePromptReinit processes prompts during re-initialization.
func (i *Interposer) handlePromptReinit(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
	promptsChanged bool,
) bool {
	changed, err := i.processNewPrompts(ctx, name, mcpClient, capsByType, newConfig)
	if err != nil {
		log.Printf("Warning: error processing prompts: %v", err)
	}

	return promptsChanged || changed
}

// handleResourceReinit processes resources and templates during re-initialization.
func (i *Interposer) handleResourceReinit(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
	resourcesChanged, templatesChanged bool,
) (bool, bool) {
	// Process resources
	resChanged, err := i.processNewResources(ctx, name, mcpClient, capsByType, newConfig)
	if err != nil {
		log.Printf("Warning: error processing resources: %v", err)
	}

	// Process resource templates
	tempChanged, err := i.processNewTemplates(ctx, name, mcpClient, capsByType, newConfig)
	if err != nil {
		log.Printf("Warning: error processing resource templates: %v", err)
	}

	return resourcesChanged || resChanged, templatesChanged || tempChanged
}

// prepareNewConfigMap creates a map of non-disabled configurations by name.
func prepareNewConfigMap(serverConfigs []config.Server) map[string]config.Server {
	newConfigMap := make(map[string]config.Server)

	for _, cfg := range serverConfigs {
		if !cfg.Disabled() {
			newConfigMap[cfg.Name] = cfg
		}
	}

	return newConfigMap
}

// processRemovedBackends handles backends that no longer exist in the config.
func (i *Interposer) processRemovedBackends(
	ctx context.Context,
	currentBackends map[string]bool,
	newConfigMap map[string]config.Server,
	changes *capabilityChanges,
) {
	// Find backends that exist in current but not in new config
	for name, exists := range currentBackends {
		if !exists {
			continue // Skip if already processed
		}

		// If backend doesn't exist in new config, remove it
		if _, hasNewConfig := newConfigMap[name]; !hasNewConfig {
			tc, pc, rc, tec := i.processRemovedBackend(ctx, name)
			changes.updateChanges(tc, pc, rc, tec)

			// Mark as processed
			currentBackends[name] = false
		}
	}
}

// processUpdatedBackends handles backends that exist in both current and new configs.
func (i *Interposer) processUpdatedBackends(
	ctx context.Context,
	currentBackends map[string]bool,
	newConfigMap map[string]config.Server,
	changes *capabilityChanges,
) {
	for name, exists := range currentBackends {
		if !exists {
			continue // Skip if already processed
		}

		newConfig, hasNewConfig := newConfigMap[name]
		if !hasNewConfig {
			continue // Skip if not in new config (should be handled by processRemovedBackends)
		}

		// Verify client still exists
		clientExists := i.verifyClientExists(name)
		if !clientExists {
			log.Printf("Client %s not found in clients map, recreating", name)
			// Mark for recreation as a new backend
			currentBackends[name] = false

			continue
		}

		// Handle existing backend update
		i.updateExistingBackend(ctx, name, newConfig, changes)

		// Mark as processed
		currentBackends[name] = false

		delete(newConfigMap, name)
	}
}

// sendChangeNotification sends a capability change notification to clients.
func (i *Interposer) sendChangeNotification(ctx context.Context, notificationType string) {
	params := map[string]any{}
	if err := i.server.SendNotificationToClient(ctx, notificationType, params); err != nil {
		log.Printf("Warning: failed to send %s notification: %v", notificationType, err)
	}
}

// verifyClientExists checks if a client exists in the clients map.
func (i *Interposer) verifyClientExists(name string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	_, exists := i.clients[name]

	return exists
}

// updateExistingBackend updates an existing backend with new configuration.
func (i *Interposer) updateExistingBackend(
	ctx context.Context,
	name string,
	newConfig config.Server,
	changes *capabilityChanges,
) {
	// For now, we always do a full restart since we can't easily
	// compare all the client parameters
	oldConfig := createDefaultOldConfig(name, newConfig)
	needsFullRestart := true

	if needsFullRestart {
		tc, pc, rc, tec := i.processBackendRestart(ctx, name, newConfig)
		changes.updateChanges(tc, pc, rc, tec)
	} else {
		// Only capability settings have changed
		tc, pc, rc, tec := i.processCapabilityUpdate(ctx, name, oldConfig, newConfig)
		changes.updateChanges(tc, pc, rc, tec)
	}
}

// processNewBackends handles backends that only exist in the new config.
func (i *Interposer) processNewBackends(
	ctx context.Context,
	newConfigMap map[string]config.Server,
	changes *capabilityChanges,
) {
	for name, config := range newConfigMap {
		tc, pc, rc, tec := i.processNewBackend(ctx, name, config)
		changes.updateChanges(tc, pc, rc, tec)
	}
}

// processDisabledTools handles tools that need to be disabled.
func (i *Interposer) processDisabledTools(
	name string,
	capsByType map[string][]string,
	newConfig config.Server,
) bool {
	toolsChanged := false
	currentTools := extractRawCapabilityNames(name, capsByType, "tool")

	var toolsToRemove []string

	for toolName := range currentTools {
		if !newConfig.Enabled(config.CapabilityTypeTool, toolName) {
			log.Printf("Tool %s is now disabled", toolName)
			fullName := fmt.Sprintf("%s.%s", name, toolName)
			toolsToRemove = append(toolsToRemove, fullName)
			toolsChanged = true

			// Remove from registry
			i.registry.RemoveCapability("tool", fullName)
		}
	}

	// Remove now-disabled tools
	if len(toolsToRemove) > 0 {
		i.server.DeleteTools(toolsToRemove...)
	}

	return toolsChanged
}

// processNewTools adds newly enabled tools.
func (i *Interposer) processNewTools(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	currentTools map[string]bool,
	newConfig config.Server,
) (bool, error) {
	toolsChanged := false

	// Get tools from client
	toolReq := mcp.ListToolsRequest{}

	result, err := mcpClient.ListTools(ctx, toolReq)
	if err != nil {
		return false, fmt.Errorf("failed to list tools: %w", err)
	}

	// Add newly enabled tools
	for _, tool := range result.Tools {
		if newConfig.Enabled(config.CapabilityTypeTool, tool.Name) && !currentTools[tool.Name] {
			// This is a newly enabled tool
			log.Printf("Adding newly enabled tool: %s", tool.Name)

			toolsChanged = true

			// Register the tool
			i.RegisterTool(name, transform(name, tool), handleTool(tool, mcpClient))
		}
	}

	return toolsChanged, nil
}

// processNewPrompts adds newly enabled prompts.
func (i *Interposer) processNewPrompts(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
) (bool, error) {
	promptsChanged := false
	currentPrompts := extractRawCapabilityNames(name, capsByType, "prompt")

	// Get prompts from client
	promptReq := mcp.ListPromptsRequest{}

	result, err := mcpClient.ListPrompts(ctx, promptReq)
	if err != nil {
		return false, fmt.Errorf("failed to list prompts: %w", err)
	}

	// Add newly enabled prompts
	for _, prompt := range result.Prompts {
		if newConfig.Enabled(config.CapabilityTypePrompt, prompt.Name) && !currentPrompts[prompt.Name] {
			// This is a newly enabled prompt
			log.Printf("Adding newly enabled prompt: %s", prompt.Name)

			promptsChanged = true

			// Register the prompt
			transformedPrompt := transform(name, prompt)
			i.RegisterPrompt(name, transformedPrompt, handlePrompt(prompt, mcpClient))
		}
	}

	return promptsChanged, nil
}

// processNewResources adds newly enabled resources.
func (i *Interposer) processNewResources(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
) (bool, error) {
	resourcesChanged := false
	currentResources := extractRawCapabilityNames(name, capsByType, "resource")

	// Get resources from client
	resourceReq := mcp.ListResourcesRequest{}

	result, err := mcpClient.ListResources(ctx, resourceReq)
	if err != nil {
		return false, fmt.Errorf("failed to list resources: %w", err)
	}

	// Add newly enabled resources
	for _, resource := range result.Resources {
		if newConfig.Enabled(config.CapabilityTypeResource, resource.Name) && !currentResources[resource.Name] {
			// This is a newly enabled resource
			log.Printf("Adding newly enabled resource: %s", resource.Name)

			resourcesChanged = true

			// Register the resource
			i.RegisterResource(
				name,
				transform(name, resource),
				handleResource(name, resource, mcpClient),
			)
		}
	}

	return resourcesChanged, nil
}

// processNewTemplates adds newly enabled resource templates.
func (i *Interposer) processNewTemplates(
	ctx context.Context,
	name string,
	mcpClient client.MCPClient,
	capsByType map[string][]string,
	newConfig config.Server,
) (bool, error) {
	templatesChanged := false
	currentTemplates := extractRawCapabilityNames(name, capsByType, "template")

	// Get templates from client
	templateReq := mcp.ListResourceTemplatesRequest{}

	result, err := mcpClient.ListResourceTemplates(ctx, templateReq)
	if err != nil {
		return false, fmt.Errorf("failed to list resource templates: %w", err)
	}

	// Add newly enabled templates
	for _, template := range result.ResourceTemplates {
		if newConfig.Enabled(config.CapabilityTypeTemplate, template.Name) && !currentTemplates[template.Name] {
			// This is a newly enabled template
			log.Printf("Adding newly enabled resource template: %s", template.Name)

			templatesChanged = true

			// Register the template
			transformedTemplate := transform(name, template)
			i.RegisterResourceTemplate(
				name,
				transformedTemplate,
				handleResource(name, template, mcpClient),
			)
		}
	}

	return templatesChanged, nil
}

// sendNotifications sends capability change notifications based on what changed.
func (i *Interposer) sendNotifications(
	ctx context.Context,
	toolsChanged,
	promptsChanged,
	resourcesChanged,
	templatesChanged bool,
) {
	if toolsChanged {
		log.Printf("Publishing tools change notification")
		i.sendChangeNotification(ctx, "toolsChange")
	}

	if promptsChanged {
		log.Printf("Publishing prompts change notification")
		i.sendChangeNotification(ctx, "promptsChange")
	}

	if resourcesChanged || templatesChanged {
		log.Printf("Publishing resources change notification")
		i.sendChangeNotification(ctx, "resourcesChange")
	}
}

// processRemovedBackend handles a backend that's been removed from config.
func (i *Interposer) processRemovedBackend(ctx context.Context, name string) (bool, bool, bool, bool) {
	log.Printf("Removing backend that's no longer in config: %s", name)

	// Check which capability types this backend had
	capsByType := i.registry.GetCapabilitiesForBackend(name)
	toolsChanged, promptsChanged, resourcesChanged, templatesChanged := checkCapabilityChanges(capsByType)

	i.removeBackend(ctx, name)

	return toolsChanged, promptsChanged, resourcesChanged, templatesChanged
}

// processBackendRestart fully restarts a backend.
func (i *Interposer) processBackendRestart(
	ctx context.Context,
	name string,
	newConfig config.Server,
) (bool, bool, bool, bool) {
	log.Printf("Full restart needed for backend: %s", name)

	// Check which capability types this backend had
	capsByType := i.registry.GetCapabilitiesForBackend(name)
	toolsChanged, promptsChanged, resourcesChanged, templatesChanged := checkCapabilityChanges(capsByType)

	i.removeBackend(ctx, name)

	// Add the backend with the new configuration
	if err := i.AddBackend(ctx, name, newConfig); err != nil {
		log.Printf("Warning: failed to update backend %s: %v", name, err)
	} else {
		// Check which capability types were added
		capsByType = i.registry.GetCapabilitiesForBackend(name)
		newToolsChanged, newPromptsChanged, newResourcesChanged, newTemplatesChanged := checkCapabilityChanges(capsByType)

		// Combine the change flags
		toolsChanged = toolsChanged || newToolsChanged
		promptsChanged = promptsChanged || newPromptsChanged
		resourcesChanged = resourcesChanged || newResourcesChanged
		templatesChanged = templatesChanged || newTemplatesChanged
	}

	return toolsChanged, promptsChanged, resourcesChanged, templatesChanged
}

// processCapabilityUpdate updates only the capability configuration for a backend.
func (i *Interposer) processCapabilityUpdate(
	ctx context.Context,
	name string,
	oldConfig,
	newConfig config.Server,
) (bool, bool, bool, bool) {
	log.Printf("Updating capability configuration for backend: %s", name)

	// Update capability configuration without restarting
	if err := i.UpdateCapabilityConfig(ctx, name, oldConfig, newConfig); err != nil {
		log.Printf("Warning: failed to update capability configuration for %s: %v", name, err)

		// Fall back to full restart
		return i.processBackendRestart(ctx, name, newConfig)
	}

	// Check what got updated in the registry
	capsByType := i.registry.GetCapabilitiesForBackend(name)

	return checkCapabilityChanges(capsByType)
}

// processNewBackend adds a new backend that wasn't previously configured.
func (i *Interposer) processNewBackend(
	ctx context.Context,
	name string,
	config config.Server,
) (bool, bool, bool, bool) {
	log.Printf("Adding new backend: %s", name)

	if err := i.AddBackend(ctx, name, config); err != nil {
		log.Printf("Warning: failed to add backend %s: %v", name, err)

		return false, false, false, false
	}

	// Check which capability types were added
	capsByType := i.registry.GetCapabilitiesForBackend(name)

	return checkCapabilityChanges(capsByType)
}

// removeBackend removes a backend client from the interposer.
func (i *Interposer) removeBackend(ctx context.Context, name string) {
	// Get and close the client under lock
	var clientExists bool

	i.mu.Lock()

	client, clientExists := i.clients[name]
	if clientExists {
		// Close the client
		if err := client.Close(); err != nil {
			log.Printf("Error closing client %s: %v", name, err)
		}

		// Remove it from our map
		delete(i.clients, name)
	}

	i.mu.Unlock()

	if !clientExists {
		return
	}

	// Remove capabilities outside the lock
	i.RemoveTrackedCapabilities(ctx, name)
}
