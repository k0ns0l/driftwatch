# DriftWatch Makefile

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Binary name
BINARY_NAME = driftwatch
BINARY_UNIX = $(BINARY_NAME)_unix
BINARY_WINDOWS = $(BINARY_NAME).exe

# Build flags
LDFLAGS = -ldflags "-X github.com/k0ns0l/driftwatch/internal/version.Version=$(VERSION) \
                   -X github.com/k0ns0l/driftwatch/internal/version.GitCommit=$(GIT_COMMIT) \
                   -X github.com/k0ns0l/driftwatch/internal/version.BuildDate=$(BUILD_DATE)"

.PHONY: all build clean test coverage deps help

# Default target
all: test build

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v

# Build for production with optimizations
build-prod:
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_NAME) .

# Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_UNIX) .

# Build for Windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_WINDOWS) .

# Build for multiple platforms
build-all: build-linux build-windows

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(BINARY_WINDOWS)

# Run tests
test:
	$(GOTEST) -v ./... -count=1

# Run tests with coverage
coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests for specific packages
test-errors:
	$(GOTEST) -v ./internal/errors -count=1

test-logging:
	$(GOTEST) -v ./internal/logging -count=1

test-recovery:
	$(GOTEST) -v ./internal/recovery -count=1

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

deps-update:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Development tools installed successfully!"

# Check if tools are installed
check-tools:
	@echo "Checking development tools..."
	@which golangci-lint > /dev/null && echo "✓ golangci-lint is installed" || echo "✗ golangci-lint is missing (run 'make install-tools')"
	@which gosec > /dev/null && echo "✓ gosec is installed" || echo "✗ gosec is missing (run 'make install-tools')"

run:
	$(GOCMD) run . $(ARGS)

install: build
	cp $(BINARY_NAME) /usr/local/bin/

# Uninstall the binary
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

fmt:
	$(GOCMD) fmt ./...

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

security:
	@which gosec > /dev/null || (echo "gosec not found. Install it with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest" && exit 1)
	gosec ./...

# Generate comprehensive documentation
docs:
	@echo "🔧 Generating DriftWatch Documentation..."
	@mkdir -p docs
	@echo "# DriftWatch CLI Documentation" > docs/CLI.md
	@echo "" >> docs/CLI.md
	@echo "Generated on: $$(date)" >> docs/CLI.md
	@echo "" >> docs/CLI.md
	@echo "## Main Command" >> docs/CLI.md
	@echo "\`\`\`" >> docs/CLI.md
	@./$(BINARY_NAME) --help >> docs/CLI.md 2>/dev/null || (echo "Binary not found. Building..." && $(MAKE) build && ./$(BINARY_NAME) --help >> docs/CLI.md)
	@echo "\`\`\`" >> docs/CLI.md
	@echo "" >> docs/CLI.md
	@echo "## Subcommands" >> docs/CLI.md
	@echo "" >> docs/CLI.md
	@for cmd in init add list remove update monitor check health status report alert baseline ci config export validate-baseline version; do \
		echo "### driftwatch $$cmd" >> docs/CLI.md; \
		echo "\`\`\`" >> docs/CLI.md; \
		./$(BINARY_NAME) $$cmd --help >> docs/CLI.md 2>/dev/null || echo "Command not available" >> docs/CLI.md; \
		echo "\`\`\`" >> docs/CLI.md; \
		echo "" >> docs/CLI.md; \
	done
	@echo "✅ CLI documentation generated: docs/CLI.md"
	@echo ""
	@echo "📚 Available documentation:"
	@echo "  - README.md (User guide and examples)"
	@echo "  - CONTRIBUTING.md (Development guide)"
	@echo "  - CHANGELOG.md (Version history)"
	@echo "  - docs/CLI.md (Complete CLI reference)"
	@echo ""
	@echo "🌐 For code documentation, use: make docs-code"

# # Generate code documentation in a readable format
docs-code:
	@echo "📖 Generating code documentation..."
	@mkdir -p docs
	@echo "# DriftWatch Code Documentation" > docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "Generated on: $$(date)" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "## Project Structure" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "\`\`\`" >> docs/CODE.md
	@tree -I 'node_modules|.git|bin|*.exe|*.db|coverage.*' -L 3 . >> docs/CODE.md 2>/dev/null || find . -type d -name ".git" -prune -o -type f -print | head -20 >> docs/CODE.md
	@echo "\`\`\`" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "## Package Overview" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "### Main Packages" >> docs/CODE.md
	@echo "- **cmd/**: CLI commands and user interface" >> docs/CODE.md
	@echo "- **internal/config/**: Configuration management" >> docs/CODE.md
	@echo "- **internal/monitor/**: Core monitoring logic" >> docs/CODE.md
	@echo "- **internal/validator/**: OpenAPI validation" >> docs/CODE.md
	@echo "- **internal/storage/**: Data persistence" >> docs/CODE.md
	@echo "- **internal/alerting/**: Notification system" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "### Key Types and Functions" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "\`\`\`go" >> docs/CODE.md
	@$(GOCMD) doc -short ./internal/config | head -20 >> docs/CODE.md 2>/dev/null || echo "// Config package types" >> docs/CODE.md
	@echo "\`\`\`" >> docs/CODE.md
	@echo "" >> docs/CODE.md
	@echo "✅ Code documentation generated: docs/CODE.md"

# Clean documentation
docs-clean:
	@echo "🧹 Cleaning documentation..."
	@rm -rf docs/
	@echo "✅ Documentation cleaned"

# Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Development build with race detection
dev:
	$(GOBUILD) $(LDFLAGS) -race -o $(BINARY_NAME) -v

# Run example
example:
	$(GOCMD) run examples/error_handling_demo.go

# Release management targets
release-notes:
	@echo "📝 Generating release notes..."
	@./scripts/generate-release-notes.sh

release-notes-version:
	@echo "📝 Generating release notes for specific version..."
	@read -p "Enter version (e.g., 1.2.0): " version; \
	./scripts/generate-release-notes.sh $$version

release-check:
	@echo "🔍 Checking release readiness..."
	@echo "Running tests..."
	@$(MAKE) test
	@echo "Running security scan..."
	@$(MAKE) security
	@echo "Checking for deprecations..."
	@./$(BINARY_NAME) migrate check || echo "No deprecation check available (build first)"
	@echo "✅ Release checks completed"

release-prepare:
	@echo "🚀 Preparing release..."
	@$(MAKE) clean
	@$(MAKE) deps
	@$(MAKE) test
	@$(MAKE) security
	@$(MAKE) build-all
	@echo "✅ Release preparation completed"

# Deprecation management
deprecation-check:
	@echo "🔍 Checking for deprecated feature usage..."
	@./$(BINARY_NAME) migrate check || (echo "Binary not found. Building..." && $(MAKE) build && ./$(BINARY_NAME) migrate check)

deprecation-status:
	@echo "📋 Showing deprecation status..."
	@./$(BINARY_NAME) migrate status || (echo "Binary not found. Building..." && $(MAKE) build && ./$(BINARY_NAME) migrate status)

migrate-config:
	@echo "🔄 Migrating configuration..."
	@./$(BINARY_NAME) migrate config --dry-run || (echo "Binary not found. Building..." && $(MAKE) build && ./$(BINARY_NAME) migrate config --dry-run)

# Security and vulnerability management
security-full:
	@echo "🔒 Running comprehensive security scan..."
	@$(MAKE) security
	@echo "Checking for vulnerabilities..."
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...
	@echo "Generating SBOM..."
	@which syft > /dev/null || go install github.com/anchore/syft/cmd/syft@latest
	@syft scan . -o spdx-json=sbom.spdx.json
	@echo "✅ Security scan completed"

vulnerability-check:
	@echo "🛡️  Checking for known vulnerabilities..."
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...

sbom:
	@echo "📋 Generating Software Bill of Materials..."
	@which syft > /dev/null || go install github.com/anchore/syft/cmd/syft@latest
	@syft scan . -o spdx-json=sbom.spdx.json
	@syft scan . -o cyclonedx-json=sbom.cyclonedx.json
	@echo "✅ SBOM generated: sbom.spdx.json, sbom.cyclonedx.json"

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build         - Build the binary"
	@echo "  build-prod    - Build optimized production binary"
	@echo "  build-linux   - Build for Linux"
	@echo "  build-windows - Build for Windows"
	@echo "  build-all     - Build for all platforms"
	@echo "  clean         - Clean build artifacts"
	@echo ""
	@echo "Test targets:"
	@echo "  test          - Run all tests"
	@echo "  coverage      - Run tests with coverage report"
	@echo "  test-errors   - Run error package tests"
	@echo "  test-logging  - Run logging package tests"
	@echo "  test-recovery - Run recovery package tests"
	@echo ""
	@echo "Development targets:"
	@echo "  deps          - Download dependencies"
	@echo "  deps-update   - Update dependencies"
	@echo "  install-tools - Install development tools (golangci-lint, gosec)"
	@echo "  check-tools   - Check if development tools are installed"
	@echo "  run           - Run the application (use ARGS=... for arguments)"
	@echo "  dev           - Development build with race detection"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo ""
	@echo "Security targets:"
	@echo "  security      - Run security scan"
	@echo "  security-full - Run comprehensive security scan"
	@echo "  vulnerability-check - Check for known vulnerabilities"
	@echo "  sbom          - Generate Software Bill of Materials"
	@echo ""
	@echo "Release targets:"
	@echo "  release-notes - Generate release notes for latest version"
	@echo "  release-notes-version - Generate release notes for specific version"
	@echo "  release-check - Check release readiness"
	@echo "  release-prepare - Prepare release (clean, test, build)"
	@echo ""
	@echo "Migration targets:"
	@echo "  deprecation-check - Check for deprecated feature usage"
	@echo "  deprecation-status - Show deprecation status"
	@echo "  migrate-config - Preview configuration migration"
	@echo ""
	@echo "Documentation targets:"
	@echo "  docs          - Generate comprehensive CLI documentation"
	@echo "  docs-code     - Generate code documentation"
	@echo "  docs-clean    - Clean generated documentation"
	@echo ""
	@echo "Installation targets:"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  uninstall     - Remove binary from /usr/local/bin"
	@echo ""
	@echo "Utility targets:"
	@echo "  version       - Show version information"
	@echo "  example       - Run error handling demo"
	@echo "  help          - Show this help message"