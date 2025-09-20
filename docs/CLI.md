# DriftWatch CLI Documentation

Generated on: Sat Sep 20 07:40:57 AM WAT 2025

## Main Command
```
DriftWatch is a CLI tool that continuously monitors API endpoints 
and detects when their actual behavior drifts from their documented specifications.

The tool helps development teams catch breaking changes, undocumented modifications, 
and API evolution before they impact downstream consumers.

Usage:
  driftwatch [flags]
  driftwatch [command]

Available Commands:
  add               Add an API endpoint to monitor
  alert             Manage alert configuration and testing
  backup            Create a backup of the DriftWatch database
  baseline          Create baseline response data for CI/CD integration
  check             Perform a one-time check of all endpoints
  ci                Run DriftWatch in CI/CD mode
  cleanup           Clean up old monitoring data and optimize database
  completion        Generate the autocompletion script for the specified shell
  config            Manage configuration
  db-health         Check the health status of the DriftWatch database
  export            Export monitoring data and drift history
  health            Show endpoint health and monitoring status
  help              Help about any command
  init              Initialize a new DriftWatch project
  list              List all monitored endpoints
  migrate           Migration tools for deprecated features
  monitor           Start continuous monitoring of endpoints
  remove            Remove an endpoint from monitoring
  repair            Repair database integrity issues
  report            Generate drift reports and analysis
  restore           Restore the DriftWatch database from a backup
  status            Show monitoring status and endpoint health
  update            Update an endpoint configuration
  validate-baseline Validate a baseline file
  version           Show version information

Flags:
      --config string   config file (default is .driftwatch.yaml)
  -h, --help            help for driftwatch
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
      --version         show version information

Use "driftwatch [command] --help" for more information about a command.
```

## Subcommands

### driftwatch init
```
Initialize a new DriftWatch project by creating a default configuration file.

This command creates a .driftwatch.yaml configuration file in the current directory
with sensible defaults and example settings to get you started quickly.

Examples:
  driftwatch init                    # Create config in current directory
  driftwatch init --config my.yaml  # Create config with custom filename
  driftwatch init --force           # Overwrite existing config file

Usage:
  driftwatch init [flags]

Flags:
  -f, --force   overwrite existing configuration file
  -h, --help    help for init

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch add
```
Add an API endpoint to the monitoring configuration.

This command registers a new endpoint for monitoring with the specified URL and options.
The endpoint will be added to both the configuration file and the database.

Examples:
  driftwatch add https://api.example.com/v1/users
  driftwatch add https://api.example.com/v1/users --method POST --spec openapi.yaml
  driftwatch add https://api.example.com/v1/users --header "Authorization=Bearer token" --interval 5m
  driftwatch add https://api.example.com/v1/users --id my-users-api --timeout 30s

Usage:
  driftwatch add <url> [flags]

Flags:
  -H, --header strings            HTTP headers (format: key=value)
  -h, --help                      help for add
      --id string                 endpoint ID (auto-generated if not provided)
      --ignore-fields strings     fields to ignore during validation
  -i, --interval duration         monitoring interval (1m to 24h) (default 5m0s)
  -m, --method string             HTTP method (GET, POST, PUT, DELETE) (default "GET")
      --request-body string       file containing request body for POST/PUT requests
      --required-fields strings   fields that must be present
      --retry-count int           retry count (uses global default if not set)
  -s, --spec string               OpenAPI specification file path
      --strict                    enable strict validation mode
      --timeout duration          request timeout (uses global default if not set)

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch list
```
List all registered API endpoints with their current configuration and status.

This command displays all endpoints that are configured for monitoring,
including their URLs, methods, intervals, and current status.

Examples:
  driftwatch list                    # List in table format
  driftwatch list --output json     # List in JSON format
  driftwatch list --output yaml     # List in YAML format
  driftwatch list --enabled-only    # Show only enabled endpoints

Usage:
  driftwatch list [flags]

Flags:
      --enabled-only    show only enabled endpoints
  -h, --help            help for list
  -o, --output string   output format (table, json, yaml) (default "table")

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch remove
```
Remove an API endpoint from the monitoring configuration.

This command removes the specified endpoint from both the configuration file
and the database. Historical monitoring data will be preserved unless --purge is used.

Examples:
  driftwatch remove my-api-endpoint
  driftwatch remove my-api-endpoint --purge    # Also remove historical data

Usage:
  driftwatch remove <id> [flags]

Flags:
  -h, --help    help for remove
      --purge   also remove historical monitoring data

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch update
```
Update the configuration of an existing API endpoint.

This command allows you to modify the settings of an already registered endpoint
without removing and re-adding it.

Examples:
  driftwatch update my-api --interval 10m
  driftwatch update my-api --method POST --spec new-spec.yaml
  driftwatch update my-api --header "Authorization=Bearer newtoken"
  driftwatch update my-api --disable    # Disable monitoring for this endpoint

Usage:
  driftwatch update <id> [flags]

Flags:
      --disable                   disable monitoring for this endpoint
      --enable                    enable monitoring for this endpoint
  -H, --header strings            HTTP headers (format: key=value)
  -h, --help                      help for update
      --ignore-fields strings     fields to ignore during validation
  -i, --interval duration         monitoring interval (1m to 24h)
  -m, --method string             HTTP method (GET, POST, PUT, DELETE)
      --request-body string       file containing request body for POST/PUT requests
      --required-fields strings   fields that must be present
      --retry-count int           retry count
  -s, --spec string               OpenAPI specification file path
      --strict                    enable strict validation mode
      --timeout duration          request timeout

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch monitor
```
Start continuous monitoring of all configured endpoints.

This command starts a background process that polls all registered endpoints
according to their configured intervals. The monitoring will continue until
stopped with Ctrl+C or a termination signal.

Examples:
  driftwatch monitor                    # Start monitoring all endpoints
  driftwatch monitor --duration 1h     # Monitor for 1 hour then stop
  driftwatch monitor --endpoints api1,api2  # Monitor specific endpoints only

Usage:
  driftwatch monitor [flags]

Flags:
      --daemon              run in daemon mode (background)
      --duration duration   monitoring duration (0 for indefinite)
      --endpoints strings   specific endpoints to monitor (comma-separated)
  -h, --help                help for monitor

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch check
```
Perform a one-time check of all configured endpoints.

This command checks all registered endpoints once and reports the results.
It does not start continuous monitoring.

Examples:
  driftwatch check                      # Check all endpoints
  driftwatch check --endpoints api1,api2  # Check specific endpoints only
  driftwatch check --timeout 30s       # Use custom timeout

Usage:
  driftwatch check [flags]

Flags:
      --endpoints strings   specific endpoints to check (comma-separated)
  -h, --help                help for check
  -o, --output string       output format (table, json, yaml) (default "table")
      --timeout duration    timeout for the entire check operation

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch health
```
Display the current health status of all monitored endpoints including
response times, success rates, and recent drift activity.

This command provides a quick overview of your API monitoring setup and
helps identify endpoints that may need attention.

Examples:
  driftwatch health                    # Show health for all endpoints
  driftwatch health --endpoint my-api # Show health for specific endpoint
  driftwatch health --unhealthy-only  # Show only unhealthy endpoints
  driftwatch health --output json     # Output in JSON format

Usage:
  driftwatch health [flags]

Flags:
  -e, --endpoint string   show health for specific endpoint ID
  -h, --help              help for health
  -o, --output string     output format (table, json, yaml) (default "table")
      --unhealthy-only    show only unhealthy endpoints

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch status
```
Show the current status of the monitoring system and health of all endpoints.

This command displays information about the monitoring scheduler, endpoint statuses,
recent check results, and overall system health.

Examples:
  driftwatch status                     # Show status in table format
  driftwatch status --output json      # Show status in JSON format
  driftwatch status --detailed         # Show detailed endpoint information

Usage:
  driftwatch status [flags]

Flags:
      --detailed        show detailed endpoint information
  -h, --help            help for status
  -o, --output string   output format (table, json, yaml) (default "table")

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch report
```
Generate comprehensive reports about detected API drifts and changes over time.

This command analyzes monitoring data and provides insights into API evolution,
breaking changes, and drift patterns across different time periods.

Examples:
  driftwatch report                    # Generate report for last 24 hours
  driftwatch report --period 7d       # Generate report for last 7 days
  driftwatch report --period 30d      # Generate report for last 30 days
  driftwatch report --endpoint my-api # Report for specific endpoint
  driftwatch report --severity high   # Show only high severity drifts
  driftwatch report --output json     # Output in JSON format

Usage:
  driftwatch report [flags]

Flags:
      --acknowledged      show only acknowledged drifts
  -e, --endpoint string   filter by specific endpoint ID
  -h, --help              help for report
  -o, --output string     output format (table, json, yaml) (default "table")
  -p, --period string     time period for report (24h, 7d, 30d) (default "24h")
  -s, --severity string   filter by severity (low, medium, high, critical)
      --unacknowledged    show only unacknowledged drifts

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch alert
```
The alert command provides functionality to manage alert channels,
test alert configurations, and view alert history.

Examples:
  driftwatch alert test                    # Test all configured alert channels
  driftwatch alert test --channel slack   # Test specific alert channel
  driftwatch alert history                # View alert history
  driftwatch alert history --drift-id 123 # View alerts for specific drift

Usage:
  driftwatch alert [command]

Available Commands:
  history     View alert delivery history
  test        Test alert channel configurations

Flags:
  -h, --help   help for alert

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output

Use "driftwatch alert [command] --help" for more information about a command.
```

### driftwatch baseline
```
Create baseline response data by capturing current API responses.

This command captures the current state of your API endpoints and saves them
as baseline data that can be used for drift detection in CI/CD pipelines.

The baseline file contains response data (status codes, headers, and bodies)
that will be used as the reference point for detecting changes.

Examples:
  driftwatch baseline                          # Create baseline for all endpoints
  driftwatch baseline --output baseline.json  # Save to specific file
  driftwatch baseline --endpoints api1,api2   # Create baseline for specific endpoints
  driftwatch baseline --pretty                # Pretty-print JSON output

Usage:
  driftwatch baseline [flags]

Flags:
      --endpoints strings   specific endpoints to capture (comma-separated)
  -h, --help                help for baseline
      --include-body        include response body in baseline (default true)
      --include-headers     include response headers in baseline (default true)
  -o, --output string       output file for baseline data (default "baseline.json")
      --overwrite           overwrite existing baseline file
      --pretty              pretty-print JSON output
      --timeout duration    timeout for each endpoint request (default 30s)

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch ci
```
Run DriftWatch in CI/CD mode with machine-readable output and appropriate exit codes.

This command is designed for integration with CI/CD pipelines. It performs a single
check of all endpoints and exits with specific codes based on the results:

Exit Codes:
  0 - Success (no breaking changes detected)
  1 - General error (configuration, network, etc.)
  2 - Breaking changes detected
  3 - Configuration error
  4 - Network error
  5 - Validation error

Examples:
  driftwatch ci                        # Run CI check with default settings
  driftwatch ci --format json         # Output results in JSON format
  driftwatch ci --format junit        # Output results in JUnit XML format
  driftwatch ci --fail-on high        # Fail on high severity changes or above
  driftwatch ci --timeout 60s         # Set timeout for the entire operation
  driftwatch ci --no-storage          # Run without persistent storage
  driftwatch ci --endpoints api1,api2 # Check specific endpoints only

Usage:
  driftwatch ci [flags]

Flags:
      --baseline-file string   JSON file containing baseline responses for comparison
      --endpoints strings      specific endpoints to check (comma-separated)
      --fail-on string         minimum severity to fail on (low, medium, high, critical) (default "high")
      --fail-on-breaking       fail if any breaking changes are detected (default true)
  -f, --format string          output format (json, junit, summary) (default "json")
  -h, --help                   help for ci
      --include-performance    include performance changes in results
      --no-storage             run without persistent storage (in-memory only)
      --output-file string     write results to file instead of stdout
      --timeout duration       timeout for the entire CI operation (default 5m0s)

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output
```

### driftwatch config
```
Manage DriftWatch configuration including viewing, validating, and initializing config files.

Examples:
  driftwatch config show          # Show current configuration
  driftwatch config validate     # Validate configuration
  driftwatch config init         # Initialize default configuration file

Usage:
  driftwatch config [command]

Available Commands:
  init        Initialize default configuration file
  show        Show current configuration
  validate    Validate configuration

Flags:
  -h, --help   help for config

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
  -v, --verbose         verbose output

Use "driftwatch config [command] --help" for more information about a command.
```

### driftwatch export
```
Export historical monitoring data and drift information to various formats
for analysis, reporting, or backup purposes.

This command allows you to extract data from the DriftWatch database
in structured formats suitable for external analysis tools.

Examples:
  driftwatch export --format csv      # Export to CSV format
  driftwatch export --format json     # Export to JSON format
  driftwatch export --period 30d      # Export last 30 days of data
  driftwatch export --endpoint my-api # Export data for specific endpoint
  driftwatch export --type drifts     # Export only drift data
  driftwatch export --type runs       # Export only monitoring runs

Usage:
  driftwatch export [flags]

Flags:
  -e, --endpoint string   filter by specific endpoint ID
  -f, --format string     export format (json, csv, yaml) (default "json")
  -h, --help              help for export
  -o, --output string     output file (default: stdout)
  -p, --period string     time period to export (24h, 7d, 30d) (default "30d")
  -t, --type string       data type to export (drifts, runs, all) (default "all")

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

### driftwatch validate-baseline
```
Validate the structure and content of a baseline file.

This command checks that a baseline file is properly formatted and contains
valid response data that can be used for drift detection.

Examples:
  driftwatch validate-baseline baseline.json
  driftwatch validate-baseline --file baseline.json --verbose

Usage:
  driftwatch validate-baseline [flags]

Flags:
  -f, --file string   baseline file to validate
  -h, --help          help for validate-baseline
  -v, --verbose       verbose validation output

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -o, --output string   output format (table, json, yaml) (default "table")
```

### driftwatch version
```
Display version information for DriftWatch including version number,
git commit, build date, Go version, and platform information.

Examples:
  driftwatch version              # Show basic version info
  driftwatch version --output json  # Show version info in JSON format
  driftwatch version --detailed   # Show detailed version information

Usage:
  driftwatch version [flags]

Flags:
  -d, --detailed        show detailed version information
  -h, --help            help for version
  -o, --output string   output format (text, json, yaml) (default "text")

Global Flags:
      --config string   config file (default is .driftwatch.yaml)
  -v, --verbose         verbose output
```

