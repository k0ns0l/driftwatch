package version

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	info := GetVersion()

	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.GitCommit)
	assert.NotEmpty(t, info.GoVersion)
	assert.NotEmpty(t, info.Platform)

	// Check that Go version matches runtime
	assert.Equal(t, runtime.Version(), info.GoVersion)

	// Check platform format
	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	assert.Equal(t, expectedPlatform, info.Platform)
}

func TestGetVersionString(t *testing.T) {
	// Test development build
	originalCommit := GitCommit
	GitCommit = "dev"

	versionStr := GetVersionString()
	assert.Contains(t, versionStr, "DriftWatch")
	assert.Contains(t, versionStr, Version)
	assert.Contains(t, versionStr, "development build")

	// Test production build
	GitCommit = "abc123"
	BuildDate = "2023-01-01T00:00:00Z"

	versionStr = GetVersionString()
	assert.Contains(t, versionStr, "DriftWatch")
	assert.Contains(t, versionStr, Version)
	assert.Contains(t, versionStr, "abc123")
	assert.Contains(t, versionStr, "2023-01-01T00:00:00Z")

	// Restore original
	GitCommit = originalCommit
}

func TestGetDetailedVersionString(t *testing.T) {
	detailedStr := GetDetailedVersionString()

	assert.Contains(t, detailedStr, "DriftWatch Version Information:")
	assert.Contains(t, detailedStr, "Version:")
	assert.Contains(t, detailedStr, "Git Commit:")
	assert.Contains(t, detailedStr, "Build Date:")
	assert.Contains(t, detailedStr, "Go Version:")
	assert.Contains(t, detailedStr, "Platform:")

	// Check that all values are present
	assert.Contains(t, detailedStr, Version)
	assert.Contains(t, detailedStr, GitCommit)
	assert.Contains(t, detailedStr, runtime.Version())
	assert.Contains(t, detailedStr, runtime.GOOS)
	assert.Contains(t, detailedStr, runtime.GOARCH)
}

func TestSetBuildInfo(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildDate := BuildDate

	// Test setting all values
	SetBuildInfo("2.0.0", "def456", "2023-12-31T23:59:59Z")

	assert.Equal(t, "2.0.0", Version)
	assert.Equal(t, "def456", GitCommit)
	assert.Equal(t, "2023-12-31T23:59:59Z", BuildDate)

	// Test setting partial values
	SetBuildInfo("3.0.0", "", "")

	assert.Equal(t, "3.0.0", Version)
	assert.Equal(t, "def456", GitCommit)                  // Should remain unchanged
	assert.NotEqual(t, "2023-12-31T23:59:59Z", BuildDate) // Should be updated to current time

	// Test empty version (should not change)
	SetBuildInfo("", "ghi789", "")

	assert.Equal(t, "3.0.0", Version) // Should remain unchanged
	assert.Equal(t, "ghi789", GitCommit)

	// Restore original values
	Version = originalVersion
	GitCommit = originalCommit
	BuildDate = originalBuildDate
}

func TestVersionInfoJSON(t *testing.T) {
	info := GetVersion()

	// Test JSON marshaling
	jsonData, err := json.Marshal(info)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaled Info
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, info.Version, unmarshaled.Version)
	assert.Equal(t, info.GitCommit, unmarshaled.GitCommit)
	assert.Equal(t, info.BuildDate, unmarshaled.BuildDate)
	assert.Equal(t, info.GoVersion, unmarshaled.GoVersion)
	assert.Equal(t, info.Platform, unmarshaled.Platform)
}

func TestVersionConstants(t *testing.T) {
	// Test that version constants are reasonable
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, GitCommit)
	assert.NotEmpty(t, GoVersion)
	assert.NotEmpty(t, Platform)

	// Test Go version format
	assert.True(t, strings.HasPrefix(GoVersion, "go"))

	// Test platform format
	assert.Contains(t, Platform, "/")
	parts := strings.Split(Platform, "/")
	assert.Len(t, parts, 2)
	assert.NotEmpty(t, parts[0]) // OS
	assert.NotEmpty(t, parts[1]) // Architecture
}

func TestVersionStringFormats(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		gitCommit    string
		buildDate    string
		expectDev    bool
		expectCommit bool
		expectDate   bool
	}{
		{
			name:         "development build",
			version:      "1.0.0",
			gitCommit:    "dev",
			buildDate:    "unknown",
			expectDev:    true,
			expectCommit: false,
			expectDate:   false,
		},
		{
			name:         "production build",
			version:      "1.0.0",
			gitCommit:    "abc123",
			buildDate:    "2023-01-01T00:00:00Z",
			expectDev:    false,
			expectCommit: true,
			expectDate:   true,
		},
		{
			name:         "tagged release",
			version:      "v1.2.3",
			gitCommit:    "def456",
			buildDate:    "2023-06-15T12:30:45Z",
			expectDev:    false,
			expectCommit: true,
			expectDate:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalVersion := Version
			originalCommit := GitCommit
			originalBuildDate := BuildDate

			// Set test values
			Version = tt.version
			GitCommit = tt.gitCommit
			BuildDate = tt.buildDate

			versionStr := GetVersionString()

			assert.Contains(t, versionStr, "DriftWatch")
			assert.Contains(t, versionStr, tt.version)

			if tt.expectDev {
				assert.Contains(t, versionStr, "development build")
			} else {
				assert.NotContains(t, versionStr, "development build")
			}

			if tt.expectCommit {
				assert.Contains(t, versionStr, tt.gitCommit)
			}

			if tt.expectDate {
				assert.Contains(t, versionStr, tt.buildDate)
			}

			// Restore original values
			Version = originalVersion
			GitCommit = originalCommit
			BuildDate = originalBuildDate
		})
	}
}

// Benchmark tests
func BenchmarkGetVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetVersion()
	}
}

func BenchmarkGetVersionString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetVersionString()
	}
}

func BenchmarkGetDetailedVersionString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetDetailedVersionString()
	}
}
