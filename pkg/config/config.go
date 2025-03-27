package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

//go:embed config.yaml
var embeddedConfig embed.FS

var (
	// ErrConfigNotFound is returned when a configuration file is not found.
	ErrConfigNotFound = errors.New("config not found")

	// ErrConfigInvalid is returned when a configuration file is invalid.
	ErrConfigInvalid = errors.New("config invalid")
)

const (
	// DirectoryPermissions are set to 0755.
	DirectoryPermissions = 0o755

	// FilePermissions are set to 0644.
	FilePermissions = 0o644
)

// Config represents the main configuration structure.
type Config struct {
	Servers []any `json:"servers" yaml:"servers"`
}

// ClaudeConfig represents Claude Desktop's configuration structure.
type ClaudeConfig struct {
	MCPServers map[string]Server `json:"mcpServers"` //nolint:tagliatelle
}

// DefaultConfigFileName is the default name for the config file.
const DefaultConfigFileName = "config.yaml"

// DefaultConfigDirName is the default directory name for config files.
const DefaultConfigDirName = "posuer"

type loadOptions struct {
	getUserConfigDir func() (string, error)
}

// WithUserConfigDir sets the function to get the user's config directory.
func WithUserConfigDir(fn func() (string, error)) func(*loadOptions) {
	return func(opts *loadOptions) {
		opts.getUserConfigDir = fn
	}
}

// Load loads the configuration from the specified path or default location.
func Load(configPath string, opts ...func(*loadOptions)) ([]Server, error) {
	// If configPath is provided, use that
	if configPath != "" {
		if !fileExists(configPath) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, configPath)
		}

		return LoadConfig(configPath)
	}

	options := &loadOptions{
		getUserConfigDir: os.UserConfigDir,
	}

	for _, opt := range opts {
		opt(options)
	}
	// Try to find config file in standard location
	configDir, err := options.getUserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user config directory: %w", err)
	}

	defaultConfigPath := filepath.Join(configDir, DefaultConfigDirName, DefaultConfigFileName)
	if fileExists(defaultConfigPath) {
		return LoadConfig(defaultConfigPath)
	}

	// Create an example config file from the embedded config
	if err = createExampleConfig(defaultConfigPath); err != nil {
		return nil, fmt.Errorf("could not create example config file: %w", err)
	}

	return LoadConfig(defaultConfigPath)
}

// LoadConfig loads the configuration from the specified path.
func LoadConfig(configPath string) ([]Server, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Auto-detect the config format based on file extension
	var mainConfig Config

	ext := strings.ToLower(filepath.Ext(configPath))
	if ext == ".json" {
		// Try to parse as Claude Desktop format first
		var claudeConfig ClaudeConfig
		if err := json.Unmarshal(data, &claudeConfig); err == nil && len(claudeConfig.MCPServers) > 0 {
			// It's a Claude Desktop config
			return convertClaudeConfig(claudeConfig)
		}

		// Otherwise, try to parse as our own JSON format
		if err := json.Unmarshal(data, &mainConfig); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	} else {
		// Assume YAML format
		if err := yaml.Unmarshal(data, &mainConfig); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	}

	return processConfig(mainConfig, filepath.Dir(configPath))
}

// processConfig processes the configuration, handling includes.
func processConfig(cfg Config, baseDir string) ([]Server, error) {
	var servers []Server

	for _, entry := range cfg.Servers {
		switch value := entry.(type) {
		case string:
			// It's a file path, include servers from there
			included, err := includeServersFromFile(value, baseDir)
			if err != nil {
				return nil, err
			}

			servers = append(servers, included...)
		case map[string]any:
			// It's an inline server configuration
			serverBytes, err := yaml.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal server config: %w", err)
			}

			var server Server
			if err := yaml.Unmarshal(serverBytes, &server); err != nil {
				return nil, fmt.Errorf("failed to unmarshal server config: %w", err)
			}

			servers = append(servers, server)
		case Server:
			// It's already a proper Server
			servers = append(servers, value)
		default:
			return nil, fmt.Errorf("%w: unsupported server type: %T", ErrConfigInvalid, value)
		}
	}

	return servers, nil
}

// includeServersFromFile includes server configurations from the specified file.
func includeServersFromFile(filePath string, baseDir string) ([]Server, error) {
	// Resolve path that may contain ~ for home directory
	if strings.HasPrefix(filePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}

		filePath = filepath.Join(home, filePath[1:])
	}

	// Handle relative paths
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(baseDir, filePath)
	}

	// Load and parse the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read included file %s: %w", filePath, err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".json" {
		// First try Claude Desktop format
		var claudeConfig ClaudeConfig
		if err := json.Unmarshal(data, &claudeConfig); err == nil && len(claudeConfig.MCPServers) > 0 {
			// It's a Claude Desktop config
			return convertClaudeConfig(claudeConfig)
		}

		// Otherwise, try our own format with the servers array
		var ourConfig Config
		if err := json.Unmarshal(data, &ourConfig); err != nil {
			return nil, fmt.Errorf("failed to parse included JSON file %s: %w", filePath, err)
		}

		return processConfig(ourConfig, filepath.Dir(filePath))
	}

	// Assume YAML format
	var yamlConfig Config
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse included YAML file %s: %w", filePath, err)
	}

	return processConfig(yamlConfig, filepath.Dir(filePath))
}

// convertClaudeConfig converts a Claude Desktop config to our server config format.
func convertClaudeConfig(claudeConfig ClaudeConfig) ([]Server, error) {
	servers := make([]Server, 0, len(claudeConfig.MCPServers))

	for name, server := range claudeConfig.MCPServers {
		// If name is not set in the config, use the key as the name
		if server.Name == "" {
			server.Name = name
		}
		// Default to stdio for Claude configs
		if server.Type == "" {
			server.Type = ServerTypeStdio
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}

		log.Printf("Warning: error checking file %s: %v", path, err)

		return false
	}

	return !info.IsDir()
}

// createExampleConfig creates an example configuration file.
func createExampleConfig(configPath string) error {
	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read the embedded example config
	exampleConfigData, err := embeddedConfig.ReadFile("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to read embedded config: %w", err)
	}

	// Write example config from embedded file
	if err := os.WriteFile(configPath, exampleConfigData, FilePermissions); err != nil {
		return fmt.Errorf("failed to write example config file: %w", err)
	}

	return nil
}
