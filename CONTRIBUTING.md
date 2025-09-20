# Contributing to DriftWatch 🤝

Thank you for your interest in contributing to DriftWatch! We welcome contributions from developers of all experience levels. This guide will help you get started and ensure your contributions have the maximum impact.

## 🎯 Quick Start

1. **Fork** the repository on GitHub
2. **Clone** your fork locally
3. **Create** a feature branch
4. **Make** your changes
5. **Test** thoroughly
6. **Submit** a pull request

```bash
git clone https://github.com/k0ns0l/driftwatch.git
cd driftwatch
git checkout -b feature/your-feature-name
# Make changes
go test ./...
git commit -m "Add your feature"
git push origin feature/your-feature-name
# Open PR on GitHub
```

<details>
<summary>🛠️ Development Setup</summary>

### Prerequisites

- **Go 1.24+** ([install guide](https://golang.org/doc/install))
- **Git** ([install guide](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git))
- **Make** (optional, for convenience scripts)

### Local Development

```bash
# Clone the repository
git clone https://github.com/k0ns0l/driftwatch.git
cd driftwatch

# Install dependencies
go mod download

# Build the project
go build -o bin/driftwatch .

# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run linting (if you have golangci-lint installed)
golangci-lint run
```

### Project Structure

```
driftwatch/
├── api/                 # API related code and definitions
├── cmd/                 # CLI command implementations
├── docs/                # Documentation files
├── examples/            # Example configurations and demos
├── internal/            # Private application packages
│   ├── alerting/        # Alert notification systems
│   ├── auth/            # Authentication handling
│   ├── config/          # Configuration management
│   ├── deprecation/     # Deprecation warnings and handling
│   ├── drift/           # Drift detection logic
│   ├── errors/          # Error handling utilities
│   ├── http/            # HTTP client functionality
│   ├── logging/         # Logging utilities
│   ├── monitor/         # Monitoring and scheduling
│   ├── recovery/        # Recovery mechanisms
│   ├── retention/       # Data retention policies
│   ├── security/        # Security utilities
│   ├── storage/         # Data storage backends
│   ├── validator/       # Validation logic
│   └── version/         # Version management
├── scripts/             # Build and utility scripts
└── test/                # Test files and fixtures
```

</details>

<details>
<summary>🎨 Code Style</summary>

### Go Standards

We follow standard Go conventions:

- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions small and focused

### Naming Conventions

```go
// Good
type EndpointValidator struct {}
func (v *EndpointValidator) ValidateResponse() error {}
var ErrInvalidSchema = errors.New("invalid schema")

// Avoid
type endpointvalidator struct {}
func (v *endpointvalidator) validate_response() error {}
var InvalidSchemaError = errors.New("invalid schema")
```

### Error Handling

```go
// Good - wrap errors with context
if err != nil {
    return fmt.Errorf("failed to validate endpoint %s: %w", endpoint.URL, err)
}

// Good - define sentinel errors
var ErrEndpointNotFound = errors.New("endpoint not found")

// Avoid - generic error messages
if err != nil {
    return errors.New("something went wrong")
}
```

</details>

<details>
<summary>🧪 Testing Guidelines</summary>

### Test Structure

- Use table-driven tests for multiple scenarios
- Test both happy path and error conditions
- Include integration tests for CLI commands
- Mock external dependencies (HTTP calls, file system)

### Example Test

```go
func TestEndpointValidator_ValidateResponse(t *testing.T) {
    tests := []struct {
        name           string
        response       []byte
        schema         []byte
        expectedError  string
        expectedDrifts []Drift
    }{
        {
            name:     "valid response matches schema",
            response: []byte(`{"id": 1, "name": "John"}`),
            schema:   []byte(`{"type": "object", "properties": {...}}`),
            expectedError: "",
            expectedDrifts: nil,
        },
        {
            name:     "missing required field",
            response: []byte(`{"name": "John"}`),
            schema:   []byte(`{"type": "object", "required": ["id"]}`),
            expectedError: "",
            expectedDrifts: []Drift{{Type: "missing_field", Field: "id"}},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            validator := NewEndpointValidator()
            drifts, err := validator.ValidateResponse(tt.response, tt.schema)
            
            if tt.expectedError != "" {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedDrifts, drifts)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./internal/validator
go test ./cmd

# Run with verbose output
go test -v ./...
```

</details>

<details>
<summary>📝 Documentation</summary>

### Code Documentation

- Add godoc comments for all exported functions, types, and packages
- Include examples in documentation where helpful
- Document complex algorithms and business logic

```go
// ValidateResponse compares an API response against its OpenAPI schema
// and returns a list of detected drifts. It returns an error only if
// validation cannot be performed (e.g., invalid schema).
//
// Example:
//   drifts, err := validator.ValidateResponse(responseBody, schemaData)
//   if err != nil {
//       return fmt.Errorf("validation failed: %w", err)
//   }
func (v *EndpointValidator) ValidateResponse(response, schema []byte) ([]Drift, error) {
    // Implementation...
}
```

### README Updates

- Update feature lists when adding new functionality
- Add examples for new CLI commands
- Update installation instructions if needed

### Changelog

Follow [Keep a Changelog](https://keepachangelog.com/) format:

```markdown
## [Unreleased]
### Added
- New `driftwatch alert setup` command for configuring notifications
- Support for custom validation rules in configuration

### Changed
- Improved error messages for schema validation failures

### Fixed
- Fixed memory leak in monitoring loop for large responses
```

</details>

<details>
<summary>🔄 Pull Request Process</summary>

### Before Submitting

- [ ] Code compiles without warnings
- [ ] All tests pass (`go test ./...`)
- [ ] Code is formatted (`gofmt -s -w .`)
- [ ] New functionality includes tests
- [ ] Documentation is updated
- [ ] Commit messages follow convention

### PR Template

When opening a PR, include:

```markdown
## Description
Brief description of the changes and motivation.

## Changes Made
- [ ] Added new feature X
- [ ] Fixed bug in component Y
- [ ] Updated documentation for Z

## Testing
- [ ] Added unit tests
- [ ] Added integration tests
- [ ] Tested manually with: `driftwatch ...`

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added/updated
```

### Review Process

1. **Automated Checks**: CI builds, tests, and linting must pass
2. **Maintainer Review**: At least one maintainer reviews the code
3. **Community Feedback**: Contributors may provide feedback
4. **Integration**: Approved PRs are merged using squash commits

</details>

<details>
<summary>🐛 Reporting Issues</summary>

### Bug Reports

Use the bug report template and include:

- **Version**: DriftWatch version (`driftwatch version`)
- **Environment**: OS, Go version
- **Steps to Reproduce**: Minimal example to reproduce the issue
- **Expected Behavior**: What should happen
- **Actual Behavior**: What actually happens
- **Configuration**: Relevant config files (sanitized)

### Feature Requests

Use the feature request template and include:

- **Use Case**: Why is this feature needed?
- **Proposed Solution**: How should it work?
- **Alternatives**: What alternatives have you considered?
- **Examples**: Similar features in other tools

</details>

<details>
<summary>🏗️ Contributing Areas</summary>

### 🔰 Good First Issues

Perfect for newcomers:
- Documentation improvements
- Error message enhancements  
- Test coverage increases
- Example configurations and demos
- CLI help text improvements
- Adding more OpenAPI validation rules
- Improving command output formatting

### 🚀 Feature Development

Looking for contributors to work on:
- Enhanced alert integrations (Discord, Teams, webhooks)
- GraphQL schema support
- Performance optimizations for large responses
- Additional output formats (HTML reports, CSV exports)
- Advanced CI/CD integrations
- Custom validation rules and plugins
- Response caching and optimization
- Multi-environment comparison features

### 🧪 Testing & Quality

Help improve reliability:
- Integration test coverage
- Performance benchmarks
- Error handling improvements
- Edge case testing
- Documentation testing

### 📖 Documentation

Documentation improvements needed:
- API documentation
- Tutorial content
- Best practices guides
- Troubleshooting guides
- Video tutorials

</details>

<details>
<summary>🎖️ Recognition</summary>

### Contributors

All contributors are recognized in:
- GitHub contributors list
- CHANGELOG.md acknowledgments
- Annual contributor highlights
- Conference talk acknowledgments

### Levels of Involvement

**🌟 Occasional Contributor**
- Bug reports and fixes
- Documentation improvements
- Feature suggestions

**⚡ Regular Contributor** 
- Multiple merged PRs
- Issue triage and support
- Feature development

**🏆 Core Contributor**
- Significant feature development
- Architecture decisions
- Code review responsibilities

</details>

<details>
<summary>🤔 Getting Help</summary>

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: General questions and ideas

### Asking for Help

When asking for help:
1. Search existing issues first
2. Provide context and relevant information
3. Include code examples or logs
4. Be specific about what you've tried

### Mentoring

New contributors can request mentoring:
- Code review guidance
- Architecture explanation
- Best practices coaching
- Go language help

</details>

<details>
<summary>📋 Development Guidelines</summary>

### Commit Messages

Follow [Conventional Commits](https://conventionalcommits.org/):

```bash
feat: add slack alert integration
fix: resolve memory leak in monitoring loop  
docs: update contributing guidelines
test: add integration tests for CLI commands
refactor: extract validation logic to separate package
```

### Branch Naming

Use descriptive branch names:
- `feature/slack-alerts`
- `fix/memory-leak-monitor`
- `docs/api-documentation`
- `refactor/validator-package`

### Release Process

1. Version bump in accordance with [Semantic Versioning](https://semver.org/)
2. Update CHANGELOG.md
3. Tag release (`git tag v0.1.0`)
4. GitHub Actions builds and publishes binaries
5. Announcement in discussions and README

</details>

<details>
<summary>⚖️ Code of Conduct</summary>

### Our Standards

- **Be welcoming** to contributors of all experience levels
- **Be respectful** of differing viewpoints and experiences
- **Accept constructive criticism** gracefully
- **Show empathy** towards other community members

### Enforcement

- Minor issues: Direct communication with involved parties
- Major issues: Report to maintainers via email
- Serious violations: May result in temporary or permanent bans

### Attribution

This Code of Conduct is adapted from the [Contributor Covenant](https://contributor-covenant.org/).

</details>

*Last updated: September 2025*
