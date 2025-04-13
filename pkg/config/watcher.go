package config

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a configuration file for changes.
const (
	// DefaultDebounceInterval is the default time to wait for debouncing file change events.
	DefaultDebounceInterval = 500 * time.Millisecond
)

type Watcher struct {
	configPath string
	watcher    *fsnotify.Watcher
	callbacks  []func([]Server)
	mutex      sync.RWMutex
	// Debounce mechanism
	debounceInterval time.Duration
	debounceTimer    *time.Timer
}

// NewWatcher creates a new file watcher for the given config path.
func NewWatcher(configPath string) (*Watcher, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		configPath:       absPath,
		watcher:          watcher,
		callbacks:        make([]func([]Server), 0),
		debounceInterval: DefaultDebounceInterval,
	}, nil
}

// Start begins watching the config file for changes.
func (cw *Watcher) Start(ctx context.Context) error {
	// Add the config file to the watcher
	if err := cw.watcher.Add(cw.configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	// Also watch the directory containing the config file
	configDir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(configDir); err != nil {
		log.Printf("Warning: failed to watch config directory: %v", err)
	}

	// Start the watcher goroutine
	go cw.watchLoop(ctx)

	return nil
}

// reloadConfig reloads the configuration and notifies callbacks.
// OnChange registers a callback to be called when the config file changes.
func (cw *Watcher) OnChange(callback func([]Server)) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// SetDebounceInterval sets the debounce interval for file change events.
func (cw *Watcher) SetDebounceInterval(interval time.Duration) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()
	cw.debounceInterval = interval
}

// Close stops the watcher and releases resources.
func (cw *Watcher) Close() error {
	if err := cw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
		cw.debounceTimer = nil
	}

	return nil
}

// handleConfigChange handles a config file change event with debouncing.
func (cw *Watcher) handleConfigChange() {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	// Reset the timer if it exists
	if cw.debounceTimer != nil {
		cw.debounceTimer.Stop()
	}

	// Create a new timer for this event
	cw.debounceTimer = time.AfterFunc(cw.debounceInterval, func() {
		cw.reloadConfig()
	})
}

func (cw *Watcher) reloadConfig() {
	log.Printf("Reloading configuration from %s", cw.configPath)

	// Load the updated configuration
	serverConfigs, err := LoadConfig(cw.configPath)
	if err != nil {
		log.Printf("Error reloading configuration: %v", err)

		return
	}

	// Notify all registered callbacks
	cw.mutex.RLock()
	defer cw.mutex.RUnlock()

	for _, callback := range cw.callbacks {
		callback(serverConfigs)
	}
}

// watchLoop is the main event loop for the file watcher.
func (cw *Watcher) watchLoop(ctx context.Context) {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if this event is for our config file
			if filepath.Clean(event.Name) != filepath.Clean(cw.configPath) {
				continue
			}

			// We only care about write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce the events
			cw.handleConfigChange()

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}

			log.Printf("Error watching config file: %v", err)

		case <-ctx.Done():
			cw.Close()

			return
		}
	}
}
