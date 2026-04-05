.PHONY: help build clean install test run deps fmt lint dev docker-build \
         test-coverage build-all run-all docs generate-workspaces perf-test perf-clean \
         start stop reset

# ============================================================================
# Configuration
# ============================================================================

BINARY_NAME=kwot
VERSION?=1.0.9
BUILD_DIR=bin
DOCS_DIR=docs
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
# Optimize binary: strip debug symbols (-s), strip DWARF info (-w)
# With -trimpath: reduces binary from ~10.5MB to ~8MB, no impact on runtime performance
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

# Performance testing defaults
NUM_WORKSPACES?=50
CONFIG_DIR?=config-perf-test
CLEANUP?=true

# ============================================================================
# Default target
# ============================================================================

all: deps fmt lint test build


# ============================================================================
# Build targets
# ============================================================================

## build: Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) -v

## build-release: Build optimized release binary (stripped debug symbols)
build-release:
	@echo "Building optimized release binary..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-release -v
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-release

## build-minimal: Build ultra-minimal binary (CGO disabled, no cgo)
build-minimal:
	@echo "Building minimal binary (no CGO, static linking)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-minimal main.go
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-minimal
	@echo "Size reduction: $$(stat -f%z $(BUILD_DIR)/$(BINARY_NAME)-minimal | numfmt --to=iec-i --suffix=B 2>/dev/null || du -h $(BUILD_DIR)/$(BINARY_NAME)-minimal | cut -f1)"

## build-all: Build for multiple platforms (darwin, linux, windows)
build-all: clean deps
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@echo "✓ Build complete!"
	@echo ""
	@echo "Binary sizes:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-* | awk '{print $$9, $$5}'

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## size: Analyze binary size breakdown
size:
	@echo "📊 Binary Size Analysis"
	@echo "======================="
	@echo ""
	@echo "Current builds:"
	@if [ -d $(BUILD_DIR) ]; then ls -lh $(BUILD_DIR)/$(BINARY_NAME)* 2>/dev/null | awk '{printf "  %-30s %6s\n", $$9, $$5}' || echo "  (no binaries found)"; else echo "  (no binaries found - run 'make build' first)"; fi
	@echo ""
	@echo "Build options comparison:"
	@echo "  Standard:    $(GOBUILD) $(LDFLAGS) -o bin/kwot main.go"
	@echo "  Minimal:     CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o bin/kwot main.go"
	@echo "  No-strip:    $(GOBUILD) -o bin/kwot main.go"
	@echo ""
	@echo "💡 To reduce size further:"
	@echo "  1. make build-minimal  - Static binary without cgo (usually 6-7MB)"
	@echo "  2. Check imported packages: go list -m all"
	@echo "  3. Unused imports: go list ./... | xargs go list -f='{{.Imports}}'"

## install: Install the binary to GOPATH/bin
install: build
	@echo "Installing to $(GOPATH)/bin..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# ============================================================================
# Testing targets
# ============================================================================

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# ============================================================================
# Code quality targets
# ============================================================================

## deps: Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✓ Dependencies up to date"

## fmt: Format all Go code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "✓ Code formatted"

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "✗ golangci-lint not installed. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run
	@echo "✓ Linting passed"

# ============================================================================
# Documentation targets
# ============================================================================

## docs: Generate and display command documentation
docs:
	@echo "Generating command documentation..."
	@mkdir -p $(DOCS_DIR)
	@./bin/kwot --help > $(DOCS_DIR)/commands.txt 2>&1 || echo "Build binary first with: make build"
	@echo "Available documentation:"
	@echo "  - README.md                  - Project overview"
	@echo "  - COMMANDS_CHEATSHEET.md     - Complete command reference with examples"
	@echo "  - CHANGELOG.md               - Version history and changes"
	@echo "  - MIGRATION.md               - Node.js to Go migration guide"
	@echo "  - $(DOCS_DIR)/commands.txt   - Auto-generated CLI help"
	@echo ""
	@echo "Quick start: ./bin/kwot --help"

# ============================================================================
# Runtime targets
# ============================================================================

## run: Build and run the application (shows help)
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

## run-all: Apply all configuration (workspaces, roles, groups)
run-all: build
	@echo "Applying all configuration..."
	./$(BUILD_DIR)/$(BINARY_NAME) all

# ============================================================================
# Development targets
# ============================================================================

## dev: Run in development mode with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "✓ Docker image built: $(BINARY_NAME):$(VERSION)"

# ============================================================================
# Performance Testing targets
# ============================================================================

## generate-workspaces: Generate workspace configuration for performance testing
## Usage: make generate-workspaces NUM_WORKSPACES=50 CONFIG_DIR=config-perf-test
generate-workspaces:
	@echo "Generating workspaces..."
	@bash perf-test/generate-workspaces.sh $(NUM_WORKSPACES) $(CONFIG_DIR)

## perf-test: Run performance test with generated workspaces
## Usage: make perf-test (uses CONFIG_DIR from configuration)
## To skip cleanup: make perf-test CLEANUP=false
perf-test: build
	@echo "Running performance test..."
	@CONFIG_DIR=$(CONFIG_DIR) bash perf-test/perf-test.sh $(CLEANUP)

## perf-quick: Quick performance test (starts Kong, runs apply, then stops)
## Usage: make perf-quick NUM_WORKSPACES=50
perf-quick: build generate-workspaces
	@echo "Running quick performance test with generated workspaces..."
	@CONFIG_DIR=$(CONFIG_DIR) bash perf-test/perf-test.sh $(CLEANUP)

## perf-clean: Clean up performance test configuration and results
perf-clean:
	@echo "Cleaning up performance test files..."
	@rm -rf config-perf-test
	@rm -rf perf-test/test-results
	@echo "✓ Cleaned up performance test files"

## perf-reset: Reset Kong database and ensure control plane is ready (for fresh start)
## Usage: make perf-reset
perf-reset:
	@echo "Starting Kong (if not running)..."
	@cd perf-test && docker-compose -f docker-compose.yaml up -d 2>/dev/null || true
	@echo "Waiting for Kong to be ready..."
	@sleep 5
	@for i in {1..30}; do \
		if docker exec kong-gateway-cp curl -sf -H "kong-admin-token: password" http://localhost:8001 > /dev/null 2>&1; then \
			echo "✓ Kong is ready"; \
			break; \
		fi; \
		echo "Attempt $$i/30..."; \
		sleep 2; \
	done
	@echo "Resetting Kong DB (this will erase all data)..."
	docker exec kong-gateway-cp kong migrations reset -y
	docker exec kong-gateway-cp kong migrations bootstrap -y
	@echo "Restarting kong-gateway-cp container..."
	docker restart kong-gateway-cp
	@echo "Waiting for Kong to be ready..."
	@sleep 10
	@for i in {1..30}; do \
		if curl -sf -H "kong-admin-token: password" http://localhost:8001 > /dev/null 2>&1; then \
			echo "✓ Kong control plane is ready!"; \
			exit 0; \
		fi; \
		echo "Attempt $$i/30..."; \
		sleep 2; \
	done; \
	echo "✗ Kong failed to become ready"; \
	exit 1

## perf-full: Full performance test workflow (generate + run)
## Usage: make perf-full NUM_WORKSPACES=50
## Note: This cleans up config files but does NOT reset Kong data
perf-full: perf-clean perf-quick
	@echo "✓ Performance test complete"

## perf-full-clean: Full test with Kong data reset (fresh start)
## Usage: make perf-full-clean NUM_WORKSPACES=50
## WARNING: This will erase all Kong data
perf-full-clean: perf-reset perf-full
	@echo "✓ Full performance test with clean Kong database complete"

## start: Start Kong using docker compose
start:
	cd perf-test && docker compose up -d

## stop: Stop Kong using docker compose
stop:
	cd perf-test && docker compose down

## reset: Reset Kong database (erase all data)
reset:
	@echo "Resetting Kong DB (this will erase all data)..."
	docker exec -it kong-gateway-cp kong migrations reset -y
	docker exec -it kong-gateway-cp kong migrations bootstrap -y
	@echo "Restarting kong-gateway-cp container..."
	docker restart kong-gateway-cp

## help: Show this help message
help:
	@echo "$(BINARY_NAME) - Kong Onboarding Control Tool"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'
	@echo ""
	@echo "Examples:"
	@echo "  make build                          # Build binary"
	@echo "  make test                           # Run tests"
	@echo "  make build-all                      # Build for all platforms"
	@echo "  make test-coverage                  # Run tests with coverage"
	@echo "  make lint fmt                       # Lint and format code"
	@echo ""
	@echo "Performance Testing:"
	@echo "  make perf-quick NUM_WORKSPACES=50   # Quick perf test (dry-run + apply)"
	@echo "  make perf-full NUM_WORKSPACES=50    # Full test (cleans config, keeps Kong data)"
	@echo "  make perf-full-clean NUM_WORKSPACES=50  # Clean test (resets Kong DB, fresh start)"
	@echo "  make perf-reset                     # Just reset Kong DB"
	@echo "  make perf-clean                     # Just clean config files"

.DEFAULT_GOAL := help