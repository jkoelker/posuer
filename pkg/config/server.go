package config

// ServerType represents the type of MCP server connection.
type ServerType string

const (
	// ServerTypeStdio represents a stdio-based MCP server.
	ServerTypeStdio ServerType = "stdio"
	// ServerTypeSSE represents an SSE-based MCP server.
	ServerTypeSSE ServerType = "sse"
)

// Server represents a single MCP server configuration.
type Server struct {
	Name      string            `json:"name"      yaml:"name"`
	Type      ServerType        `json:"type"      yaml:"type"`
	Command   string            `json:"command"   yaml:"command"`
	Args      []string          `json:"args"      yaml:"args"`
	Env       map[string]string `json:"env"       yaml:"env"`
	URL       string            `json:"url"       yaml:"url"`
	Enable    *Capability       `json:"enable"    yaml:"enable"`
	Disable   *Capability       `json:"disable"   yaml:"disable"`
	Container *Container        `json:"container" yaml:"container"`
}

// Clone creates a deep copy of the Server.
func (s *Server) Clone() Server {
	if s == nil {
		return Server{}
	}

	server := *s

	if s.Args != nil {
		server.Args = make([]string, len(s.Args))
		copy(server.Args, s.Args)
	}

	if s.Env != nil {
		server.Env = make(map[string]string, len(s.Env))
		for k, v := range s.Env {
			server.Env[k] = v
		}
	}

	if s.Enable != nil {
		server.Enable = s.Enable.Clone()
	}

	if s.Disable != nil {
		server.Disable = s.Disable.Clone()
	}

	if s.Container != nil {
		server.Container = s.Container.Clone()
	}

	return server
}

// ServerType return the type of the server.
func (s *Server) ServerType() ServerType {
	if s.Type != "" {
		return s.Type
	}

	if s.URL != "" {
		return ServerTypeSSE
	}

	return ServerTypeStdio
}

// Disabled returns true if the server is disabled.
func (s *Server) Disabled() bool {
	return s.isExplicitlyDisabled() || s.hasEmptyEnabledCapabilities() || s.hasAllEmptyCapabilityLists()
}

// Enabled returns true if the item is enabled for the server.
func (s *Server) Enabled(capability CapabilityType, name string) bool {
	if s.Disabled() {
		return false
	}

	if s.disabled(capability, name) {
		return false
	}

	if s.Enable == nil {
		// If no enable configuration exists, default to enabled
		return true
	}

	if s.Enable.All {
		return true
	}

	// If we have enable.Capabilities map
	if s.Enable.Capabilities != nil {
		// Check if this specific capability+name is in the enabled list
		if enabled, ok := s.Enable.Capabilities[capability]; ok {
			for _, enabledName := range enabled {
				if enabledName == name {
					return true
				}
			}
			// The capability type exists in Enable.Capabilities but this
			// specific name is not in the list
			return false
		}
	}

	// If enable exists but doesn't contain this capability type,
	// we're using a whitelist approach, so return false
	if len(s.Enable.Capabilities) > 0 {
		return false
	}

	// Empty enable configuration means nothing is explicitly enabled,
	// which means everything is enabled by default
	return true
}

// isExplicitlyDisabled checks if the server is explicitly disabled.
func (s *Server) isExplicitlyDisabled() bool {
	return s.Disable != nil && s.Disable.All
}

// hasEmptyEnabledCapabilities checks if Enable exists but has empty capabilities.
func (s *Server) hasEmptyEnabledCapabilities() bool {
	return s.Enable != nil &&
		!s.Enable.All &&
		s.Enable.Capabilities != nil &&
		len(s.Enable.Capabilities) == 0
}

// hasAllEmptyCapabilityLists checks if Enable has only empty capability lists.
func (s *Server) hasAllEmptyCapabilityLists() bool {
	if s.Enable == nil || s.Enable.All || s.Enable.Capabilities == nil || len(s.Enable.Capabilities) == 0 {
		return false
	}

	// Check if all capability lists are empty
	for _, capList := range s.Enable.Capabilities {
		if len(capList) > 0 {
			return false
		}
	}

	return true
}

// disabled returns true if the item is disabled for the server.
func (s *Server) disabled(capability CapabilityType, name string) bool {
	if s.Disabled() {
		return true
	}

	if s.Disable != nil {
		if s.Disable.All {
			return true
		}

		if disabled, ok := s.Disable.Capabilities[capability]; ok {
			for _, disabledName := range disabled {
				if disabledName == name {
					return true
				}
			}
		}
	}

	return false
}
