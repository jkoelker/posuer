package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jkoelker/posuer/pkg/config"
)

func TestWatcher(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for test files
	tempDir := t.TempDir() // Using t.TempDir() instead of os.MkdirTemp

	// Create a test config file
	testConfigPath := filepath.Join(tempDir, "test-config.yaml")
	testConfig := `servers:
  - name: test-server
    type: stdio
    command: echo
    args: ["hello"]
`

	if err := os.WriteFile(testConfigPath, []byte(testConfig), 0o600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create the watcher
	watcher, err := config.NewWatcher(testConfigPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Set a shorter debounce interval for testing
	watcher.SetDebounceInterval(100 * time.Millisecond)

	// Create a channel to receive config change notifications
	configChan := make(chan []config.Server, 1)

	watcher.OnChange(func(configs []config.Server) {
		configChan <- configs
	})

	// Start the watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Modify the config file
	updatedConfig := `servers:
  - name: test-server
    type: stdio
    command: echo
    args: ["updated"]
  - name: second-server
    type: stdio
    command: cat
`
	// Wait a bit before modifying to ensure watcher is ready
	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(testConfigPath, []byte(updatedConfig), 0o600); err != nil {
		t.Fatalf("Failed to update test config: %v", err)
	}

	// Wait for the notification with a timeout
	select {
	case configs := <-configChan:
		if len(configs) != 2 {
			t.Errorf("Expected 2 server configs, got %d", len(configs))
		}

		if configs[0].Name != "test-server" || configs[1].Name != "second-server" {
			t.Errorf("Unexpected server names: %s, %s", configs[0].Name, configs[1].Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for config change notification")
	}
}
