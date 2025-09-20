# DriftWatch Code Documentation

Generated on: Sat Sep 20 07:41:29 AM WAT 2025

## Project Structure

```
.
├── api
├── cmd
│   ├── alert.go
│   ├── backup.go
│   ├── backup_test.go
│   ├── baseline.go
│   ├── baseline_test.go
│   ├── ci.go
│   ├── ci_integration_test.go
│   ├── ci_test.go
│   ├── cleanup.go
│   ├── cleanup_test.go
│   ├── config.go
│   ├── config_test.go
│   ├── endpoint.go
│   ├── endpoint_test.go
│   ├── health.go
│   ├── init.go
│   ├── migrate.go
│   ├── monitor.go
│   ├── repair.go
│   ├── report.go
│   ├── report_test.go
│   ├── restore.go
│   ├── root.go
│   ├── root_test.go
│   └── version.go
├── CONTRIBUTING.md
├── Dockerfile
├── docs
│   ├── CLI.md
│   └── CODE.md
├── driftwatch
├── examples
│   ├── auth-config.yaml
│   └── error_handling_demo.go
├── go.mod
├── go.sum
├── internal
│   ├── alerting
│   │   ├── alerting.go
│   │   ├── alerting_test.go
│   │   ├── discord.go
│   │   ├── discord_test.go
│   │   ├── email.go
│   │   ├── email_test.go
│   │   ├── integration_test.go
│   │   ├── slack.go
│   │   ├── slack_test.go
│   │   ├── webhook.go
│   │   └── webhook_test.go
│   ├── auth
│   │   ├── auth.go
│   │   ├── auth_test.go
│   │   └── integration_test.go
│   ├── config
│   │   ├── auth_validation_test.go
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── utils.go
│   │   ├── validation.go
│   │   └── validation_test.go
│   ├── deprecation
│   │   ├── deprecation.go
│   │   └── deprecation_test.go
│   ├── drift
│   │   ├── drift.go
│   │   ├── drift_test.go
│   │   └── example_test.go
│   ├── errors
│   │   ├── errors.go
│   │   └── errors_test.go
│   ├── http
│   │   ├── client.go
│   │   ├── client_test.go
│   │   ├── example_test.go
│   │   └── integration_test.go
│   ├── logging
│   │   ├── logger.go
│   │   └── logger_test.go
│   ├── monitor
│   │   ├── monitor.go
│   │   └── scheduler_test.go
│   ├── recovery
│   │   ├── recovery.go
│   │   └── recovery_test.go
│   ├── retention
│   │   ├── integration_test.go
│   │   ├── service.go
│   │   └── service_test.go
│   ├── security
│   │   ├── fileops.go
│   │   └── fileops_test.go
│   ├── storage
│   │   ├── integration_test.go
│   │   ├── integrity_test.go
│   │   ├── memory.go
│   │   ├── memory_test.go
│   │   ├── migrations.go
│   │   ├── migrations_test.go
│   │   ├── sqlite.go
│   │   ├── sqlite_test.go
│   │   └── storage.go
│   ├── test.test
│   ├── validator
│   │   ├── example_test.go
│   │   ├── integration_test.go
│   │   ├── testdata
│   │   ├── validator.go
│   │   └── validator_test.go
│   └── version
│       ├── version.go
│       └── version_test.go
├── LICENSE
├── main.go
├── Makefile
├── README.md
├── RELEASE_NOTES.md
├── sbom.cyclonedx.json
├── sbom.spdx.json
├── scripts
│   └── generate-release-notes.sh
└── test

24 directories, 100 files
```

## Package Overview

### Main Packages
- **cmd/**: CLI commands and user interface
- **internal/config/**: Configuration management
- **internal/monitor/**: Core monitoring logic
- **internal/validator/**: OpenAPI validation
- **internal/storage/**: Data persistence
- **internal/alerting/**: Notification system

### Key Types and Functions

```go
func ConfigExists(filename string) bool
func CreateDefaultConfigFile(filename string) error
func GetConfigFilePath(configFile string) string
func SaveConfig(config *Config, filename string) error
func ValidateConfig(config *Config) error
type APIKeyAuth struct{ ... }
type AlertChannelConfig struct{ ... }
type AlertRuleConfig struct{ ... }
type AlertingConfig struct{ ... }
type AuthConfig struct{ ... }
type AuthType string
    const AuthTypeNone AuthType = "none" ...
type BasicAuth struct{ ... }
type BearerAuth struct{ ... }
type Config struct{ ... }
    func DefaultConfig() *Config
    func LoadConfig(configFile string) (*Config, error)
type EndpointConfig struct{ ... }
type GlobalConfig struct{ ... }
type OAuth2Auth struct{ ... }
```

