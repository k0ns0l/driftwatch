package deprecation

import (
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.GetNotices())
}

func TestRegisterNotice(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	notice := &Notice{
		Feature:     "test-feature",
		Version:     "v1.0.0",
		Replacement: "new-feature",
		Severity:    SeverityWarning,
	}

	manager.RegisterNotice(notice)

	registered, exists := manager.CheckFeature("test-feature")
	assert.True(t, exists)
	assert.Equal(t, notice, registered)
}

func TestWarnOnce(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	notice := &Notice{
		Feature:     "test-feature",
		Version:     "v1.0.0",
		Replacement: "new-feature",
		Severity:    SeverityWarning,
	}

	manager.RegisterNotice(notice)

	// Test that WarnOnce only warns once
	manager.WarnOnce("test-feature")
	manager.WarnOnce("test-feature") // Should not warn again

	// We can't easily test the exact output without capturing it,
	// but we can test that the function doesn't panic
	assert.True(t, manager.IsDeprecated("test-feature"))
}

func TestWarn(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	notice := &Notice{
		Feature:     "test-feature",
		Version:     "v1.0.0",
		Replacement: "new-feature",
		Severity:    SeverityWarning,
	}

	manager.RegisterNotice(notice)

	// Test that Warn warns every time
	manager.Warn("test-feature")
	manager.Warn("test-feature")

	// We can't easily test the exact output without capturing it,
	// but we can test that the function doesn't panic
	assert.True(t, manager.IsDeprecated("test-feature"))
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityCritical, "CRITICAL"},
		{Severity(999), "UNKNOWN"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.severity.String())
	}
}

func TestGetActiveNotices(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	activeNotice := &Notice{
		Feature:     "active-feature",
		RemovalDate: future,
		Severity:    SeverityWarning,
	}

	expiredNotice := &Notice{
		Feature:     "expired-feature",
		RemovalDate: past,
		Severity:    SeverityWarning,
	}

	permanentNotice := &Notice{
		Feature:  "permanent-feature",
		Severity: SeverityInfo,
	}

	manager.RegisterNotice(activeNotice)
	manager.RegisterNotice(expiredNotice)
	manager.RegisterNotice(permanentNotice)

	active := manager.GetActiveNotices()

	assert.Len(t, active, 2)
	assert.Contains(t, active, "active-feature")
	assert.Contains(t, active, "permanent-feature")
	assert.NotContains(t, active, "expired-feature")
}

func TestCheckFeature(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	notice := &Notice{
		Feature:  "test-feature",
		Severity: SeverityWarning,
	}

	manager.RegisterNotice(notice)

	found, exists := manager.CheckFeature("test-feature")
	assert.True(t, exists)
	assert.Equal(t, notice, found)

	notFound, exists := manager.CheckFeature("non-existing")
	assert.False(t, exists)
	assert.Nil(t, notFound)
}

func TestIsDeprecated(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	notice := &Notice{
		Feature:  "test-feature",
		Severity: SeverityWarning,
	}

	manager.RegisterNotice(notice)

	assert.True(t, manager.IsDeprecated("test-feature"))
	assert.False(t, manager.IsDeprecated("non-existing"))
}

func TestFormatNoticeForCLI(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	removalDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	notice := &Notice{
		Feature:      "test-feature",
		Version:      "v1.0.0",
		RemovalDate:  removalDate,
		Replacement:  "new-feature",
		Reason:       "Better performance",
		MigrationURL: "https://example.com/migration",
		Severity:     SeverityWarning,
	}

	formatted := manager.FormatNoticeForCLI(notice)

	assert.Contains(t, formatted, "DEPRECATION WARNING: test-feature")
	assert.Contains(t, formatted, "Reason: Better performance")
	assert.Contains(t, formatted, "Use instead: new-feature")
	assert.Contains(t, formatted, "Removal date: 2025-12-31")
	assert.Contains(t, formatted, "Migration guide: https://example.com/migration")
}

func TestShowWarningDifferentSeverities(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	manager := NewManager(logger)

	infoNotice := &Notice{
		Feature:  "info-feature",
		Severity: SeverityInfo,
	}

	warningNotice := &Notice{
		Feature:  "warning-feature",
		Severity: SeverityWarning,
	}

	criticalNotice := &Notice{
		Feature:  "critical-feature",
		Severity: SeverityCritical,
	}

	manager.RegisterNotice(infoNotice)
	manager.RegisterNotice(warningNotice)
	manager.RegisterNotice(criticalNotice)

	// Test that warnings with different severities don't panic
	manager.Warn("info-feature")
	manager.Warn("warning-feature")
	manager.Warn("critical-feature")

	// Verify all features are registered
	assert.True(t, manager.IsDeprecated("info-feature"))
	assert.True(t, manager.IsDeprecated("warning-feature"))
	assert.True(t, manager.IsDeprecated("critical-feature"))
}

func TestHelperFunctions(t *testing.T) {
	config := logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
		Output: "stderr",
	}
	logger, err := logging.NewLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	InitDefault(logger)

	assert.NotPanics(t, func() {
		WarnConfigField("old-field", "new-field")
		WarnCLIFlag("old-flag", "new-flag")
		WarnAPIEndpoint("old-endpoint", "new-endpoint")
	})

	assert.False(t, IsDeprecated("non-existing-feature"))
}
