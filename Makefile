# Koneksi Drive Makefile

# Variables
BINARY_NAME := koneksi-drive
GO := go
GOFLAGS := -ldflags="-s -w"
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%d_%H:%M:%S")

# Platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
PLATFORM_TARGETS := $(addprefix build-, $(subst /,-,$(PLATFORMS)))

# Directories
BUILD_DIR := build
DIST_DIR := dist

# Default target
.DEFAULT_GOAL := build

# Help target
.PHONY: help
help:
	@echo "Koneksi Drive - Makefile targets:"
	@echo ""
	@echo "  make build          - Build for current platform"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make build-linux    - Build for Linux (amd64 and arm64)"
	@echo "  make build-darwin   - Build for macOS (amd64 and arm64)"
	@echo "  make install        - Build and install to /usr/local/bin"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make deps           - Download dependencies"
	@echo "  make tidy           - Tidy go.mod"
	@echo "  make release        - Create release archives"
	@echo "  make docker         - Build Docker image"
	@echo "  make run            - Run the application"
	@echo ""

# Build for current platform
.PHONY: build
build: deps
	@echo "Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for all platforms
.PHONY: build-all
build-all: $(PLATFORM_TARGETS)

# Platform-specific build targets
.PHONY: $(PLATFORM_TARGETS)
$(PLATFORM_TARGETS): build-%:
	$(eval GOOS := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	$(eval OUTPUT := $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH))
	@echo "Building for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(OUTPUT) .
	@echo "Built: $(OUTPUT)"

# Build for Linux
.PHONY: build-linux
build-linux: build-linux-amd64 build-linux-arm64

# Build for macOS
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

# Install to system
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete!"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f $(BINARY_NAME)
	@echo "Clean complete!"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v -race ./...

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod verify

# Tidy go.mod
.PHONY: tidy
tidy:
	@echo "Tidying go.mod..."
	$(GO) mod tidy

# Create release archives
.PHONY: release
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(subst /,-,$(PLATFORMS)); do \
		binary="$(BUILD_DIR)/$(BINARY_NAME)-$$platform"; \
		if [ -f "$$binary" ]; then \
			echo "Creating archive for $$platform..."; \
			tar -czf "$(DIST_DIR)/$(BINARY_NAME)-$$platform.tar.gz" -C $(BUILD_DIR) "$$(basename $$binary)" -C .. README.md LICENSE; \
		fi; \
	done
	@echo "Release archives created in $(DIST_DIR)/"

# Build Docker image
.PHONY: docker
docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

# Run the application
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Development targets
.PHONY: dev
dev:
	@echo "Running in development mode..."
	$(GO) run . $(ARGS)

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Check for security vulnerabilities
.PHONY: security
security:
	@echo "Checking for security vulnerabilities..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with:"; \
		echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

# Generate checksums for releases
.PHONY: checksums
checksums:
	@if [ -d "$(DIST_DIR)" ]; then \
		echo "Generating checksums..."; \
		cd $(DIST_DIR) && sha256sum *.tar.gz > SHA256SUMS.txt; \
		echo "Checksums saved to $(DIST_DIR)/SHA256SUMS.txt"; \
	else \
		echo "No dist directory found. Run 'make release' first."; \
	fi

# Version information
.PHONY: version
version:
	@echo "$(BINARY_NAME) version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"

# CI/CD helpers
.PHONY: ci-deps
ci-deps:
	@echo "Installing CI dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

.PHONY: ci
ci: ci-deps deps lint security test build-all

# Platform detection for development
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Linux)
	CURRENT_OS := linux
endif
ifeq ($(UNAME_S),Darwin)
	CURRENT_OS := darwin
endif

ifeq ($(UNAME_M),x86_64)
	CURRENT_ARCH := amd64
endif
ifeq ($(UNAME_M),arm64)
	CURRENT_ARCH := arm64
endif
ifeq ($(UNAME_M),aarch64)
	CURRENT_ARCH := arm64
endif

.PHONY: info
info:
	@echo "Current platform: $(CURRENT_OS)/$(CURRENT_ARCH)"
	@echo "Go version: $(shell $(GO) version)"
	@echo "Binary name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"