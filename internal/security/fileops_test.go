package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		wantErr     bool
	}{
		{
			name:    "valid simple path",
			path:    "test.txt",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "dir/test.txt",
			wantErr: false,
		},
		{
			name:    "directory traversal attempt",
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "directory traversal in middle",
			path:    "dir/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:        "path within allowed directory",
			path:        "allowed/test.txt",
			allowedDirs: []string{"allowed"},
			wantErr:     false,
		},
		{
			name:        "path outside allowed directory",
			path:        "forbidden/test.txt",
			allowedDirs: []string{"allowed"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path, tt.allowedDirs...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSafeCreateFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("create file in allowed directory", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")

		file, err := SafeCreateFile(testFile, tempDir)
		require.NoError(t, err)
		require.NotNil(t, file)
		defer file.Close()

		// Check file exists and has correct permissions
		info, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("reject file outside allowed directory", func(t *testing.T) {
		testFile := "/tmp/test.txt"

		file, err := SafeCreateFile(testFile, tempDir)
		assert.Error(t, err)
		assert.Nil(t, file)
	})

	t.Run("reject directory traversal", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "../../../etc/passwd")

		file, err := SafeCreateFile(testFile, tempDir)
		assert.Error(t, err)
		assert.Nil(t, file)
	})
}

func TestSafeReadFile(t *testing.T) {
	tempDir := t.TempDir()
	testContent := []byte("test content")

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, testContent, 0o600)
	require.NoError(t, err)

	t.Run("read file in allowed directory", func(t *testing.T) {
		data, err := SafeReadFile(testFile, tempDir)
		require.NoError(t, err)
		assert.Equal(t, testContent, data)
	})

	t.Run("reject file outside allowed directory", func(t *testing.T) {
		data, err := SafeReadFile("/etc/passwd", tempDir)
		assert.Error(t, err)
		assert.Nil(t, data)
	})

	t.Run("reject directory traversal", func(t *testing.T) {
		traversalPath := filepath.Join(tempDir, "../../../etc/passwd")
		data, err := SafeReadFile(traversalPath, tempDir)
		assert.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestSafeWriteFile(t *testing.T) {
	tempDir := t.TempDir()
	testContent := []byte("test content")

	t.Run("write file in allowed directory", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")

		err := SafeWriteFile(testFile, testContent, tempDir)
		require.NoError(t, err)

		// Verify file was written correctly
		data, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, data)

		// Check file permissions
		info, err := os.Stat(testFile)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("reject file outside allowed directory", func(t *testing.T) {
		testFile := "/tmp/test.txt"

		err := SafeWriteFile(testFile, testContent, tempDir)
		assert.Error(t, err)
	})

	t.Run("reject directory traversal", func(t *testing.T) {
		traversalPath := filepath.Join(tempDir, "../../../tmp/test.txt")

		err := SafeWriteFile(traversalPath, testContent, tempDir)
		assert.Error(t, err)
	})

	t.Run("create nested directories", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "nested", "dir", "test.txt")

		err := SafeWriteFile(testFile, testContent, tempDir)
		require.NoError(t, err)

		// Verify file was written correctly
		data, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, data)
	})
}

func TestValidateFilePathEdgeCases(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		err := ValidateFilePath("")
		assert.NoError(t, err) // Empty path resolves to current directory
	})

	t.Run("current directory", func(t *testing.T) {
		err := ValidateFilePath(".")
		assert.NoError(t, err)
	})

	t.Run("parent directory", func(t *testing.T) {
		err := ValidateFilePath("..")
		assert.Error(t, err)
	})

	t.Run("multiple allowed directories", func(t *testing.T) {
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()

		// Test file in first allowed directory
		testFile1 := filepath.Join(tempDir1, "test.txt")
		err := ValidateFilePath(testFile1, tempDir1, tempDir2)
		assert.NoError(t, err)

		// Test file in second allowed directory
		testFile2 := filepath.Join(tempDir2, "test.txt")
		err = ValidateFilePath(testFile2, tempDir1, tempDir2)
		assert.NoError(t, err)

		// Test file outside both directories
		err = ValidateFilePath("/tmp/test.txt", tempDir1, tempDir2)
		assert.Error(t, err)
	})
}
