.PHONY: build clean deps fmt lint test run

# Project name
PROJECT := posuer

# Build directory
BUILD_DIR := build

# Go source files in cmd and pkg directories
GO_FILES := $(shell find ./cmd ./pkg -name "*.go" -not -path "./vendor/*")

# Default target
all: $(BUILD_DIR)/$(PROJECT)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build the binary
$(BUILD_DIR)/$(PROJECT): pkg/config/config.yaml
$(BUILD_DIR)/$(PROJECT): $(GO_FILES)
$(BUILD_DIR)/$(PROJECT): $(BUILD_DIR)
	@echo "Building $(PROJECT)..."
	go build -trimpath -o $(BUILD_DIR)/$(PROJECT) ./cmd/$(PROJECT)
	@echo "Build complete: $(BUILD_DIR)/$(PROJECT)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Get dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies downloaded"

# Format Go code
fmt:
	@echo "Formatting code..."
	gofmt -w $(shell find . -name "*.go" -not -path "./vendor/*")
	@echo "Formatting complete"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Linting complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests complete"

# Run tests with coverage
test-coverage: $(BUILD_DIR)
	@echo "Running tests with coverage..."
	go test -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the application
run: $(BUILD_DIR)/$(PROJECT)
	@echo "Running $(PROJECT)..."
	@$(BUILD_DIR)/$(PROJECT)

# Show version
version:
	@echo "$(PROJECT) version $(VERSION)"
	@echo "Build date: $(BUILD_DATE)"
	@echo "Git commit: $(GIT_COMMIT)"
