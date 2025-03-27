# Posuer: MCP Manager and Interposer

A Model Context Protocol (MCP) manager and interposer that enables Large
Language Models to seamlessly interact with multiple MCP servers.

## Project Vision

Posuer acts as a bridge between MCP clients (like Claude Desktop) and multiple
MCP servers. Instead of connecting each server individually to your LLM
client, Posuer lets you:

1. **Consolidate multiple servers** - Manage all your MCP servers through a single connection point
2. **Aggregate capabilities** - Combine tools, resources, and prompts from different servers
3. **Simplify configuration** - Use a single YAML file to define all your server connections
4. **Enhance reliability** - Monitor server health and handle failures gracefully

## How It Works

```
┌───────────────┐                  ┌──────────────────────────────────────┐
│               │                  │                                      │
│   LLM Client  │◄─── MCP/stdio ──►│               Posuer                 │
│ (Claude, etc) │                  │                                      │
│               │                  │                                      │
└───────────────┘                  └──┬─────────────┬─────────────┬───────┘
                                      │             │             │
                                  MCP/stdio     MCP/stdio     MCP/SSE
                                      │             │             │
                                      ▼             ▼             ▼
                                 ┌─────────┐   ┌─────────┐   ┌─────────┐
                                 │ ServerA │   │ ServerB │   │ ServerC │
                                 └─────────┘   └─────────┘   └─────────┘
```

Posuer implements both sides of the MCP protocol:
- It acts as an **MCP server** to clients like Claude Desktop
- It acts as an **MCP client** to backend servers
- It intelligently routes messages between them based on capabilities

## Key Features

- **Dynamic server management** - Configure and manage multiple MCP servers
- **Capability aggregation** - Combine resources, tools, and prompts from all servers
- **Smart routing** - Direct requests to the appropriate backend server
- **Multiple transport types** - Support for both stdio and SSE connections
- **Configuration inclusion** - Include server configurations from multiple files, including Claude Desktop configs
- **Error handling** - Graceful handling of server failures
- **Logging** - Detailed logs for debugging and monitoring

## Installation

### Prerequisites

- Go 1.23 or higher
- Access to MCP servers (e.g., filesystem, weather, search, etc.)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/jkoelker/posuer.git
cd posuer

# Build the binary
make

# The binary will be created in the build directory
ls -la build/posuer
```

## Usage

```bash
# Run with default configuration (config.yaml)
./build/posuer

# Run with a specific configuration file
./build/posuer -config /path/to/config.yaml

# Show version information
./build/posuer -version

# Run in stdio mode (default)
./build/posuer -stdio

# Run with config file watcher enabled
./build/posuer -config /path/to/config.yaml -watch
```

## Configuration

Posuer is configured using a YAML file. By default, it looks for `config.yaml` in the following locations (in order):
1. Path specified by the `-config` flag
2. User configuration directory (e.g., `~/.config/posuer/config.yaml`)

Example configuration:

```yaml
# Posuer configuration file
servers:
  # Direct server definition
  - name: filesystem
    type: stdio
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - "/tmp"

  # You can also include other config files
  - ~/other-servers.yaml
  - ~/.config/Claude/claude_desktop_config.json

  # Example of an SSE server (remote)
  - name: remote-server
    type: sse
    url: https://example.com/sse
```

### Capability Configuration Options

Posuer provides flexible capability configuration with three formats:

1. **Boolean (Enable/Disable entire server)**
   ```yaml
   servers:
     - name: all-enabled
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-memory"
       enable: true  # Enable all capabilities

     - name: all-disabled
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-memory"
       enable: false  # Disable all capabilities (or use disable: true)
   ```

2. **List (Quick format for tools)**
   ```yaml
   servers:
     - name: specific-tools
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-memory"
       enable:
         - create_entities  # Only enable these specific tools
         - read_graph

     - name: blocked-tools
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-filesystem"
         - "/tmp"
       disable:
         - write_file  # Disable these specific tools
         - move_file
   ```

3. **Map (Advanced capability filtering)**
   ```yaml
   servers:
     - name: advanced-filtering
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-memory"
       enable:
         tools:  # Enable specific tools
           - create_entities
           - read_graph
         prompts: system_prompt  # Enable a single prompt
         templates:  # Enable specific templates
           - entity_template
           - graph_template
       disable:
         tools:  # Disable specific tools (even if in enable list)
           - delete_entities
   ```

The map format supports filtering by different capability types:
- `tools`: Tool functions the LLM can call
- `prompts`: System prompts the LLM can use
- `templates`: Resource templates for dynamic content
- `resources`: Static resources (files, etc.)

See the default config `pkg/config/config.yaml` for more detailed
configuration examples.

### Dynamic Configuration Reloading

Posuer supports dynamic configuration reloading, allowing you to modify your
configuration file while the service is running. Changes are detected
automatically and applied without requiring a restart.

To enable this feature, use the `-watch` flag:

```bash
# Run with configuration file watching enabled
./posuer -config /path/to/config.yaml -watch
```

When the configuration file is modified:
1. Posuer detects the change automatically
2. The new configuration is loaded and validated
3. New servers are added, existing servers are updated, and removed servers are shut down
4. All changes are applied without disrupting active connections

This feature is useful for:
- Adding new MCP servers on the fly
- Removing or disabling servers without restarting Posuer
- Testing different configurations during development
- Rotating API keys or updating endpoints in production

The file watcher includes debouncing to prevent excessive reloads during rapid edits.

### Configuration Options

- `servers`: Array of server configurations or file paths to include
  - For direct server definitions:
    - `name`: Server name (used for namespacing capabilities)
    - `type`: Server connection type ("stdio" or "sse")
    - `command`: The command to run (for stdio)
    - `args`: Command line arguments
    - `env`: Environment variables
    - `url`: Server URL (for sse type)
    - `enable`: Enable specific capabilities (see format options below)
    - `disable`: Disable specific capabilities (see format options below)
    - `container`: Container configuration (see container options below)
  - For file inclusions, simply provide the file path as a string

### Container Configuration

Posuer supports running MCP servers in containers for improved isolation and dependency management. Container configuration can be specified in three formats:

1. **Simple format**: Just specify the image name as a string
   ```yaml
   servers:
     - name: filesystem
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-filesystem"
         - "/tmp"
       container: "node:18-alpine"
   ```

2. **Full configuration**: Specify a map with all container options
   ```yaml
   servers:
     - name: database
       type: stdio
       command: npx
       args:
         - -y
         - "@modelcontextprotocol/server-sqlite"
       container:
         image: node:18-alpine
         volumes:
           "/data": "/app/data"
         env:
           DB_PATH: "/app/data/database.sqlite"
         network: host
         workdir: "/app"
         args:
           - "--cap-add=SYS_ADMIN"
   ```

3. **Explicitly disabled**: Set to false to disable container isolation including automatic detection
   ```yaml
   servers:
     - name: no-container
       type: stdio
       command: npx  # Would normally use a container automatically
       args:
         - -y
         - "@modelcontextprotocol/server-filesystem"
         - "/tmp"
       container: false  # Explicitly disable container usage
   ```

Container configuration options:
- `image`: The container image to use (required)
- `volumes`: Map of host paths to container paths
- `env`: Environment variables to pass to the container
- `network`: Network mode (host, bridge, etc.)
- `user`: User to run as in the container
- `workdir`: Working directory in the container
- `args`: Additional arguments to pass to the container runtime

### Automatic Container Detection

Posuer can automatically detect certain commands and run them in appropriate containers:

- `npx` commands are automatically run in `docker.io/node:alpine`
- `uvx` commands are automatically run in `ghcr.io/astral-sh/uv:alpine`

This automatic detection simplifies configuration when working with common Node.js and Python package managers. For example:

```yaml
servers:
  # This will automatically run in a Node.js container
  - name: filesystem
    type: stdio
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - "/tmp"
    # No container configuration needed, it will be auto-detected

  # To explicitly disable container detection
  - name: local-npm
    type: stdio
    command: npx
    args:
      - -y
      - some-local-package
    container: false  # Explicitly disable container detection
```

Posuer will automatically detect if a container runtime (podman or docker) is available on the system and prefer podman for better rootless container support.

## Integration with Claude Desktop

To use Posuer with Claude Desktop:

1. Build Posuer as described above
2. Create a configuration file with your desired MCP servers
3. Update your Claude Desktop configuration to use Posuer as an MCP server:

```json
{
  "mcpServers": {
    "posuer": {
      "command": "/path/to/posuer",
      "args": ["-watch"]
    }
  }
}
```

4. Restart Claude Desktop

## Development

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Clean build artifacts
make clean
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
