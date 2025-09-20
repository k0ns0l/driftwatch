// Package version provides version information for DriftWatch
package version

import (
	"fmt"
	"runtime"
	"time"
)

var (
	// Version is the current version of DriftWatch
	Version = "1.0.0"

	// GitCommit is the git commit hash (set during build)
	GitCommit = "dev"

	// BuildDate is the build date (set during build)
	BuildDate = "unknown"

	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()

	// Platform is the target platform
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// Info represents version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetVersion returns the version information
func GetVersion() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  Platform,
	}
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	info := GetVersion()
	if info.GitCommit == "dev" {
		return fmt.Sprintf("DriftWatch %s (development build)", info.Version)
	}
	return fmt.Sprintf("DriftWatch %s (commit %s, built %s)", info.Version, info.GitCommit, info.BuildDate)
}

// GetDetailedVersionString returns a detailed version string with all information
func GetDetailedVersionString() string {
	info := GetVersion()
	return fmt.Sprintf(`DriftWatch Version Information:
  Version:    %s
  Git Commit: %s
  Build Date: %s
  Go Version: %s
  Platform:   %s`, info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Platform)
}

// SetBuildInfo sets build-time information (used by build scripts)
func SetBuildInfo(version, gitCommit, buildDate string) {
	if version != "" {
		Version = version
	}
	if gitCommit != "" {
		GitCommit = gitCommit
	}
	if buildDate != "" {
		BuildDate = buildDate
	} else {
		BuildDate = time.Now().UTC().Format(time.RFC3339)
	}
}
