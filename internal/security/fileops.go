// Package security provides security utilities for DriftWatch
package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateFilePath validates that a file path is safe to use
// It prevents directory traversal attacks and ensures the path is within expected bounds
func ValidateFilePath(path string, allowedDirs ...string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// If allowed directories are specified, ensure the path is within one of them
	if len(allowedDirs) > 0 {
		allowed := false
		for _, allowedDir := range allowedDirs {
			absAllowedDir, err := filepath.Abs(allowedDir)
			if err != nil {
				continue
			}

			// Check if the path is within the allowed directory
			relPath, err := filepath.Rel(absAllowedDir, absPath)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("path is not within allowed directories: %s", path)
		}
	}

	return nil
}

// SafeCreateFile creates a file with validation and secure permissions
func SafeCreateFile(path string, allowedDirs ...string) (*os.File, error) {
	if err := ValidateFilePath(path, allowedDirs...); err != nil {
		return nil, err
	}

	// Ensure directory exists with secure permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// #nosec G304 - path is validated by ValidateFilePath above
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
}

// SafeReadFile reads a file with path validation
func SafeReadFile(path string, allowedDirs ...string) ([]byte, error) {
	if err := ValidateFilePath(path, allowedDirs...); err != nil {
		return nil, err
	}

	// #nosec G304 - path is validated by ValidateFilePath above
	return os.ReadFile(path)
}

// SafeWriteFile writes a file with path validation and secure permissions
func SafeWriteFile(path string, data []byte, allowedDirs ...string) error {
	if err := ValidateFilePath(path, allowedDirs...); err != nil {
		return err
	}

	// Ensure directory exists with secure permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}
