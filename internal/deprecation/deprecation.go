// Package deprecation provides utilities for handling deprecated features in DriftWatch
package deprecation

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/logging"
)

// Severity represents the severity level of a deprecation
type Severity int

const (
	// SeverityInfo for informational deprecations
	SeverityInfo Severity = iota
	// SeverityWarning for deprecations that should be addressed soon
	SeverityWarning
	// SeverityCritical for deprecations that will be removed soon
	SeverityCritical
)

// String returns the string representation of the severity
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Notice represents a deprecation notice
type Notice struct {
	Feature      string    `json:"feature"`
	Version      string    `json:"version"`
	RemovalDate  time.Time `json:"removal_date"`
	Replacement  string    `json:"replacement"`
	Reason       string    `json:"reason"`
	MigrationURL string    `json:"migration_url"`
	Severity     Severity  `json:"severity"`
}

// Manager handles deprecation notices and warnings
type Manager struct {
	logger   *logging.Logger
	notices  map[string]*Notice
	warnings map[string]bool // Track which warnings have been shown
}

// NewManager creates a new deprecation manager
func NewManager(logger *logging.Logger) *Manager {
	return &Manager{
		logger:   logger,
		notices:  make(map[string]*Notice),
		warnings: make(map[string]bool),
	}
}

// RegisterNotice registers a new deprecation notice
func (m *Manager) RegisterNotice(notice *Notice) {
	m.notices[notice.Feature] = notice
}

// WarnOnce shows a deprecation warning only once per session
func (m *Manager) WarnOnce(feature string) {
	if m.warnings[feature] {
		return // Already warned about this feature
	}

	notice, exists := m.notices[feature]
	if !exists {
		return // No deprecation notice for this feature
	}

	m.warnings[feature] = true
	m.showWarning(notice)
}

// Warn shows a deprecation warning every time (for critical deprecations)
func (m *Manager) Warn(feature string) {
	notice, exists := m.notices[feature]
	if !exists {
		return // No deprecation notice for this feature
	}

	m.showWarning(notice)
}

// showWarning displays the deprecation warning
func (m *Manager) showWarning(notice *Notice) {
	// Check if warnings are suppressed
	if os.Getenv("DRIFTWATCH_SUPPRESS_DEPRECATION_WARNINGS") == "true" {
		return
	}

	var prefix string
	switch notice.Severity {
	case SeverityInfo:
		prefix = "ℹ️  INFO"
	case SeverityWarning:
		prefix = "⚠️  WARNING"
	case SeverityCritical:
		prefix = "🚨 CRITICAL"
	}

	message := fmt.Sprintf("%s: %s is deprecated", prefix, notice.Feature)

	if notice.Replacement != "" {
		message += fmt.Sprintf(" - use %s instead", notice.Replacement)
	}

	if !notice.RemovalDate.IsZero() {
		message += fmt.Sprintf(" (will be removed on %s)", notice.RemovalDate.Format("2006-01-02"))
	}

	if notice.MigrationURL != "" {
		message += fmt.Sprintf("\nMigration guide: %s", notice.MigrationURL)
	}

	// Use different output methods based on severity
	switch notice.Severity {
	case SeverityInfo:
		m.logger.Info(message)
	case SeverityWarning:
		m.logger.Warn(message)
	case SeverityCritical:
		m.logger.Error(message)
		// Also print to stderr for critical warnings
		fmt.Fprintf(os.Stderr, "%s\n", message)
	}
}

// GetNotices returns all registered deprecation notices
func (m *Manager) GetNotices() map[string]*Notice {
	return m.notices
}

// GetActiveNotices returns notices that haven't been removed yet
func (m *Manager) GetActiveNotices() map[string]*Notice {
	active := make(map[string]*Notice)
	now := time.Now()

	for feature, notice := range m.notices {
		if notice.RemovalDate.IsZero() || notice.RemovalDate.After(now) {
			active[feature] = notice
		}
	}

	return active
}

// CheckFeature checks if a feature is deprecated and returns the notice
func (m *Manager) CheckFeature(feature string) (*Notice, bool) {
	notice, exists := m.notices[feature]
	return notice, exists
}

// IsDeprecated checks if a feature is deprecated
func (m *Manager) IsDeprecated(feature string) bool {
	_, exists := m.notices[feature]
	return exists
}

// FormatNoticeForCLI formats a deprecation notice for CLI display
func (m *Manager) FormatNoticeForCLI(notice *Notice) string {
	var lines []string

	// Header
	header := fmt.Sprintf("DEPRECATION %s: %s", notice.Severity.String(), notice.Feature)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("-", len(header)))

	// Reason
	if notice.Reason != "" {
		lines = append(lines, fmt.Sprintf("Reason: %s", notice.Reason))
	}

	// Replacement
	if notice.Replacement != "" {
		lines = append(lines, fmt.Sprintf("Use instead: %s", notice.Replacement))
	}

	// Timeline
	if !notice.RemovalDate.IsZero() {
		lines = append(lines, fmt.Sprintf("Removal date: %s", notice.RemovalDate.Format("2006-01-02")))

		// Time remaining
		remaining := time.Until(notice.RemovalDate)
		if remaining > 0 {
			days := int(remaining.Hours() / 24)
			lines = append(lines, fmt.Sprintf("Time remaining: %d days", days))
		} else {
			lines = append(lines, "⚠️  This feature should have been removed!")
		}
	}

	// Migration guide
	if notice.MigrationURL != "" {
		lines = append(lines, fmt.Sprintf("Migration guide: %s", notice.MigrationURL))
	}

	return strings.Join(lines, "\n")
}

// Default deprecation manager instance
var defaultManager *Manager

// InitDefault initializes the default deprecation manager
func InitDefault(logger *logging.Logger) {
	defaultManager = NewManager(logger)
	registerBuiltinNotices()
}

// GetDefault returns the default deprecation manager
func GetDefault() *Manager {
	return defaultManager
}

// WarnOnce is a convenience function that uses the default manager
func WarnOnce(feature string) {
	if defaultManager != nil {
		defaultManager.WarnOnce(feature)
	}
}

// Warn is a convenience function that uses the default manager
func Warn(feature string) {
	if defaultManager != nil {
		defaultManager.Warn(feature)
	}
}

// IsDeprecated is a convenience function that uses the default manager
func IsDeprecated(feature string) bool {
	if defaultManager != nil {
		return defaultManager.IsDeprecated(feature)
	}
	return false
}

// registerBuiltinNotices registers built-in deprecation notices
func registerBuiltinNotices() {
	if defaultManager == nil {
		return
	}

	// Example deprecation notices (these would be real deprecations in practice)

	// Future deprecations for v2.0.0
	v2RemovalDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)

	// Example: Old configuration format
	defaultManager.RegisterNotice(&Notice{
		Feature:      "legacy-config-format",
		Version:      "v1.0.0",
		RemovalDate:  v2RemovalDate,
		Replacement:  "new configuration structure in .driftwatch.yaml",
		Reason:       "Improved configuration organization and validation",
		MigrationURL: "https://github.com/k0ns0l/driftwatch/blob/main/docs/migrations/config-v2.md",
		Severity:     SeverityWarning,
	})

	// Example: Old CLI flag
	defaultManager.RegisterNotice(&Notice{
		Feature:      "monitor-old-flag",
		Version:      "v1.6.0",
		RemovalDate:  v2RemovalDate,
		Replacement:  "--interval flag",
		Reason:       "Standardization of CLI flag naming",
		MigrationURL: "https://github.com/k0ns0l/driftwatch/blob/main/docs/migrations/cli-v2.md",
		Severity:     SeverityWarning,
	})
}

// Helper functions for common deprecation patterns

// WarnConfigField warns about deprecated configuration fields
func WarnConfigField(fieldName, replacement string) {
	feature := fmt.Sprintf("config-field-%s", fieldName)
	WarnOnce(feature)
}

// WarnCLIFlag warns about deprecated CLI flags
func WarnCLIFlag(flagName, replacement string) {
	feature := fmt.Sprintf("cli-flag-%s", flagName)
	WarnOnce(feature)
}

// WarnAPIEndpoint warns about deprecated API endpoints
func WarnAPIEndpoint(endpoint, replacement string) {
	feature := fmt.Sprintf("api-endpoint-%s", endpoint)
	WarnOnce(feature)
}
