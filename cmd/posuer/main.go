package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/jkoelker/posuer/pkg/config"
	"github.com/jkoelker/posuer/pkg/interposer"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to the configuration file")
	stdioFlag := flag.Bool("stdio", false, "Run in stdio mode")
	versionFlag := flag.Bool("version", false, "Show version information")
	watchFlag := flag.Bool("watch", false, "Watch the config file for changes")
	flag.Parse()

	// Show version and exit if requested
	if *versionFlag {
		version, buildTime, revision := getVersionInfo()
		log.Printf("Posuer MCP Interposer %s\n", version)
		log.Printf("Build Date: %s\n", buildTime)
		log.Printf("Git Commit: %s\n", revision)

		return
	}

	// Load configuration
	serverConfigs, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create interposer
	version, _, _ := getVersionInfo()

	posuer, err := interposer.NewInterposer("Posuer", version)
	if err != nil {
		log.Fatalf("Failed to create interposer: %v", err)
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to backend servers
	for _, serverConfig := range serverConfigs {
		log.Printf("Connecting to backend server: %s", serverConfig.Name)

		if err := posuer.AddBackend(ctx, serverConfig.Name, serverConfig); err != nil {
			log.Printf("Warning: failed to connect to %s: %v", serverConfig.Name, err)
		}
	}

	// Set up config file watcher if requested
	setupConfigWatcher(ctx, *watchFlag, *configPath, posuer)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Run in stdio mode
	if *stdioFlag {
		log.Printf("Starting in stdio mode")

		if err := server.ServeStdio(posuer.Server()); err != nil {
			log.Printf("Stdio server error: %v", err)
		}

		return
	}

	// Default to stdio mode for now
	log.Printf("Starting in stdio mode (default)")

	if err := server.ServeStdio(posuer.Server()); err != nil {
		log.Printf("Stdio server error: %v", err)
	}
}

// getVersionInfo returns version information from runtime/debug.BuildInfo.
// Returns version, build time, and revision.
func getVersionInfo() (string, string, string) {
	// Default values
	version := "dev"
	buildTime := "unknown"
	revision := "unknown"

	// Try to get build info
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, buildTime, revision
	}

	// Check main module version
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	// Extract build info from settings (available in Go 1.18+)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			// Use first characters of the revision hash
			const shortHashLength = 12
			if len(setting.Value) > shortHashLength {
				revision = setting.Value[:shortHashLength]
			} else {
				revision = setting.Value
			}
		case "vcs.time":
			buildTime = setting.Value
		}
	}

	return version, buildTime, revision
}

// setupConfigWatcher sets up a watcher for the config file if enabled.
func setupConfigWatcher(
	ctx context.Context,
	watchEnabled bool,
	configPath string,
	posuer *interposer.Interposer,
) {
	// Skip if watching is disabled or no config path is provided
	if !watchEnabled || configPath == "" {
		return
	}

	log.Printf("Starting config file watcher for %s", configPath)

	watcher, err := config.NewWatcher(configPath)
	if err != nil {
		log.Printf("Warning: failed to create config watcher: %v", err)

		return
	}

	// Register callback for config changes
	watcher.OnChange(func(newConfigs []config.Server) {
		log.Printf("Config file changed, reconfiguring with %d backends", len(newConfigs))

		if err := posuer.Reconfigure(ctx, newConfigs); err != nil {
			log.Printf("Error reconfiguring: %v", err)
		}
	})

	// Start the watcher
	if err := watcher.Start(ctx); err != nil {
		log.Printf("Warning: failed to start config watcher: %v", err)

		return
	}

	log.Printf("Config file watcher started successfully")
	// Make sure to close the watcher when we're done
	defer watcher.Close()
}
