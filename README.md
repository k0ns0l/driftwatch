# DriftWatch

<div align="center">
  <img src="https://i.imgur.com/GrVcmlL.png" alt="DriftWatch Logo" style="max-width: 200px; height: auto;">
  
  <h3>Continuous API Monitoring & Drift Detection</h3>
  <p><em>Catch breaking changes before they impact your users</em></p>
  
  <p>
    <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue?style=for-the-badge" alt="License"></a>
    <a href="https://github.com/k0ns0l/driftwatch/actions"><img src="https://img.shields.io/github/actions/workflow/status/k0ns0l/driftwatch/ci.yml?style=for-the-badge&logo=github&label=Build" alt="Build Status"></a>
  </p>
  <p>
    <a href="https://codecov.io/gh/k0ns0l/driftwatch"><img src="https://img.shields.io/codecov/c/github/k0ns0l/driftwatch?style=for-the-badge&logo=codecov&logoColor=white" alt="Coverage"></a>
    <a href="https://goreportcard.com/report/github.com/k0ns0l/driftwatch"><img src="https://img.shields.io/badge/Go%20Report-A+-brightgreen?style=for-the-badge&logo=go&logoColor=white" alt="Go Report"></a>
    <a href="https://github.com/k0ns0l/driftwatch/releases"><img src="https://img.shields.io/github/v/release/k0ns0l/driftwatch?style=for-the-badge&logo=github&color=purple" alt="Latest Release"></a>
  </p>
</div>

---

**DriftWatch** is a powerful CLI tool that continuously monitors your API endpoints and detects when their actual behavior drifts from their documented specifications. It helps development teams catch breaking changes, undocumented modifications, and API evolution before they impact downstream consumers.

## Features

- Monitor API endpoints against OpenAPI specs
- Alert on drift via Slack, Discord, email, or webhooks
- Track response times and generate reports
- CI/CD integration with exit codes
- Local SQLite storage

## Table of Contents

- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Configuration](#️-configuration)
- [Commands](#-commands)
- [CI/CD Integration](#-cicd-integration)
- [Examples](#-examples)
- [Development](#️-development)
- [Contributing](#-contributing)

## Quick Start

Get up and running with DriftWatch in under 5 minutes:

```bash
# 1. Install DriftWatch
go install github.com/k0ns0l/driftwatch@latest

# 2. Initialize your project
driftwatch init

# 3. Add your first endpoint
driftwatch add https://api.example.com/users --spec ./openapi.yaml --interval 5m

# 4. Start monitoring
driftwatch monitor

# 5. View results
driftwatch status
```

## Installation

<details>
<summary><strong>📥 Installation Methods</strong></summary>

### Using Go Install (Recommended)
```bash
go install github.com/k0ns0l/driftwatch@latest
```

### Download Binary
Download the latest binary from [GitHub Releases](https://github.com/k0ns0l/driftwatch/releases) for your platform.

### Build from Source
```bash
git clone https://github.com/k0ns0l/driftwatch.git
cd driftwatch
go build -o driftwatch .
```

### Verify Installation
```bash
driftwatch version
```

</details>

## Configuration

<details>
<summary><strong>📝 Configuration File</strong></summary>

DriftWatch uses a `.driftwatch.yaml` configuration file. Here's a comprehensive example:

```yaml
project:
  name: "My API Monitoring Project"
  description: "Production API drift detection"

global:
  timeout: 30s
  retry_count: 3
  retry_delay: 5s
  user_agent: "driftwatch/1.0.0"

endpoints:
  - id: "users-api"
    url: "https://api.example.com/v1/users"
    method: GET
    spec_file: "./specs/users-openapi.yaml"
    interval: 5m
    headers:
      Authorization: "Bearer ${API_TOKEN}"
      Accept: "application/json"
    validation:
      strict_mode: false
      ignore_fields: ["timestamp", "request_id"]
      required_fields: ["id", "email", "name"]

  - id: "orders-api"
    url: "https://api.example.com/v1/orders"
    method: POST
    spec_file: "./specs/orders.yaml"
    interval: 10m
    request_body_file: "./fixtures/order-request.json"

alerting:
  enabled: true
  slack:
    webhook_url: "${SLACK_WEBHOOK_URL}"
    channel: "#api-alerts"
    on_events: ["breaking_change", "error_threshold"]
  
  email:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "${EMAIL_USER}"
    password: "${EMAIL_PASS}"
    from: "alerts@company.com"
    to: ["team@company.com"]

reporting:
  retention_days: 90
  auto_cleanup: true
  export_format: "json"
```

### Environment Variables
Set sensitive values using environment variables:
```bash
export API_TOKEN="your-api-token"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
export EMAIL_USER="your-email@company.com"
export EMAIL_PASS="your-app-password"
```

</details>

## Commands

<details>
<summary><strong>🔧 Project Management</strong></summary>

```bash
# Initialize a new DriftWatch project
driftwatch init [--config-file .driftwatch.yaml]

# Show current configuration
driftwatch config show

# Validate configuration file
driftwatch config validate
```

</details>

<details>
<summary><strong>🎯 Endpoint Management</strong></summary>

```bash
# Add an endpoint to monitor
driftwatch add <url> [options]
  --method GET|POST|PUT|DELETE     HTTP method (default: GET)
  --spec path/to/spec.yaml         OpenAPI specification file
  --interval 5m                    Monitoring interval (1m to 24h)
  --timeout 30s                    Request timeout
  --header "key=value"             HTTP headers (can be used multiple times)
  --id string                      Custom endpoint ID
  --strict                         Enable strict validation mode

# List all monitored endpoints
driftwatch list [--format table|json|yaml]

# Remove an endpoint
driftwatch remove <endpoint-id>

# Update endpoint configuration
driftwatch update <endpoint-id> [options]
```

</details>

<details>
<summary><strong>📊 Monitoring & Checking</strong></summary>

```bash
# Start continuous monitoring (daemon mode)
driftwatch monitor [--duration 1h] [--endpoints endpoint1,endpoint2]

# Perform one-time check of all endpoints
driftwatch check [--endpoints endpoint1,endpoint2]

# Show endpoint health and status
driftwatch health [--endpoint endpoint-id]
driftwatch status [--endpoint endpoint-id]
```

</details>

<details>
<summary><strong>📈 Reporting & Analysis</strong></summary>

```bash
# Generate drift reports
driftwatch report [options]
  --period 24h|7d|30d             Time period for report (default: 24h)
  --endpoint endpoint-id          Filter by specific endpoint
  --severity low|medium|high|critical  Filter by severity level
  --output table|json|yaml        Output format

# Export monitoring data
driftwatch export [--format csv|json] [--output data.csv]
```

</details>

<details>
<summary><strong>🚨 Alerting</strong></summary>

```bash
# Test alert configuration
driftwatch alert test [--channel slack|email]

# Show alert history
driftwatch alert history
```

</details>

<details>
<summary><strong>🔄 CI/CD & Baselines</strong></summary>

```bash
# Run in CI/CD mode with exit codes
driftwatch ci [options]
  --baseline-file baseline.json   Compare against baseline
  --fail-on high|critical         Exit with error on severity level

# Create baseline response data
driftwatch baseline [--output baseline.json]

# Validate a baseline file
driftwatch validate-baseline <baseline-file>
```

</details>

## CI/CD Integration

<details>
<summary><strong>🚀 Pipeline Integration</strong></summary>

DriftWatch integrates seamlessly with popular CI/CD platforms using meaningful exit codes.

### Exit Codes
- `0`: Success (no drift detected)
- `1`: General error (network, config, etc.)
- `2`: Breaking changes detected
- `3`: Configuration error

### GitHub Actions
```yaml
name: API Drift Detection
on: [push, pull_request]

jobs:
  api-drift-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
          
      - name: Install DriftWatch
        run: go install github.com/k0ns0l/driftwatch@latest
        
      - name: Run API Drift Check
        run: |
          driftwatch ci \
            --baseline-file .github/baselines/api-baseline.json \
            --fail-on high \
            --format json \
            --output drift-report.json
        env:
          API_TOKEN: ${{ secrets.API_TOKEN }}
          
      - name: Upload drift report
        uses: actions/upload-artifact@v3
        if: always()
        with:
          name: drift-report
          path: drift-report.json
```

### GitLab CI
```yaml
api_drift_check:
  stage: test
  image: golang:1.24
  script:
    - go install github.com/k0ns0l/driftwatch@latest
    - driftwatch ci --baseline-file baseline.json --fail-on critical
  artifacts:
    reports:
      junit: drift-report.xml
    when: always
```

### Jenkins Pipeline
```groovy
pipeline {
    agent any
    stages {
        stage('API Drift Check') {
            steps {
                sh 'go install github.com/k0ns0l/driftwatch@latest'
                sh 'driftwatch ci --baseline-file baseline.json --fail-on high'
            }
            post {
                always {
                    archiveArtifacts artifacts: 'drift-report.*', allowEmptyArchive: true
                }
            }
        }
    }
}
```

</details>

## Examples

<details>
<summary><strong>🔰 Basic REST API Monitoring</strong></summary>

```bash
# Monitor a simple GET endpoint
driftwatch add https://jsonplaceholder.typicode.com/users \
  --interval 2m \
  --timeout 10s

# Start monitoring
driftwatch monitor --duration 30m
```

</details>

<details>
<summary><strong>🔐 Advanced Configuration with Authentication</strong></summary>

```bash
# Monitor authenticated endpoint with custom headers
driftwatch add https://api.github.com/user \
  --spec ./github-api.yaml \
  --header "Authorization=Bearer ${GITHUB_TOKEN}" \
  --header "Accept=application/vnd.github.v3+json" \
  --interval 10m
```

</details>

<details>
<summary><strong>📤 POST Endpoint with Request Body</strong></summary>

```bash
# Monitor POST endpoint with JSON payload
driftwatch add https://api.example.com/orders \
  --method POST \
  --spec ./orders-spec.yaml \
  --request-body ./fixtures/sample-order.json \
  --header "Content-Type=application/json" \
  --header "Authorization=Bearer ${API_TOKEN}"
```

</details>

<details>
<summary><strong>🌍 Monitoring Multiple Environments</strong></summary>

```yaml
# .driftwatch.yaml for multi-environment setup
endpoints:
  - id: "users-dev"
    url: "https://dev-api.example.com/users"
    spec_file: "./specs/users.yaml"
    interval: 5m
    
  - id: "users-staging"
    url: "https://staging-api.example.com/users"
    spec_file: "./specs/users.yaml"
    interval: 10m
    
  - id: "users-prod"
    url: "https://api.example.com/users"
    spec_file: "./specs/users.yaml"
    interval: 15m
```

</details>

## Development

<details>
<summary><strong>🔧 Development Setup</strong></summary>

### Prerequisites
- Go 1.24 (or later)
- Make

### Setup Development Environment
```bash
# Clone the repository
git clone https://github.com/k0ns0l/driftwatch.git
cd driftwatch

# Install dependencies
go mod download

# Build the binary
go build -o bin/driftwatch .

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run linting (requires golangci-lint)
golangci-lint run

# Build for multiple platforms
make build-all
```

</details>

<details>
<summary><strong>📁 Project Structure</strong></summary>

```
driftwatch/
├── api/                   # API definitions and OpenAPI specs
├── cmd/                   # CLI commands (Cobra)
├── docs/                  # Documentation
│   ├── project/           # Project documentation (changelog, policies)
│   └── migrations/        # Version migration guides
├── examples/              # Example configurations
├── internal/              # Internal packages
│   ├── alerting/          # Notification system
│   ├── auth/              # Authentication handling
│   ├── config/            # Configuration management
│   ├── drift/             # Drift detection logic
│   ├── http/              # HTTP client utilities
│   ├── monitor/           # Core monitoring engine
│   ├── storage/           # SQLite database operations
│   └── validator/         # OpenAPI validation
├── scripts/               # Build and deployment scripts
├── test/                  # Integration and end-to-end tests
├── CONTRIBUTING.md        # Contribution guidelines
├── main.go                # Application entry point
└── go.mod                 # Go module definition
```

</details>

<details>
<summary><strong>🧪 Testing</strong></summary>

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific test package
go test ./internal/monitor

# Run integration tests
go test -tags=integration ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

</details>

## Contributing

<details>
<summary><strong>🚀 Quick Contribution Guide</strong></summary>

We welcome contributions! Here's how to get started:

1. **Fork** the repository on GitHub
2. **Clone** your fork locally
3. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
4. **Make** your changes and add tests
5. **Run** tests and ensure they pass (`go test ./...`)
6. **Commit** your changes (`git commit -m 'Add amazing feature'`)
7. **Push** to your branch (`git push origin feature/amazing-feature`)
8. **Open** a Pull Request

</details>

<details>
<summary><strong>📋 Development Guidelines</strong></summary>

- Write tests for new functionality
- Follow Go conventions and best practices
- Update documentation for user-facing changes
- Ensure CI checks pass before submitting PR
- Keep commits focused and write clear commit messages

</details>

<details>
<summary><strong>🐛 Reporting Issues</strong></summary>

- Use GitHub Issues for bug reports and feature requests
- Include steps to reproduce for bug reports
- Provide system information (OS, Go version, DriftWatch version)
- Check existing issues before creating new ones

</details>

For detailed contribution guidelines, see [CONTRIBUTING](CONTRIBUTING.md).