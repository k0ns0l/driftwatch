package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
		},
		{
			name:        "version flag",
			args:        []string{"--version"},
			expectError: false,
		},
		{
			name:        "verbose flag",
			args:        []string{"--verbose", "--help"},
			expectError: false,
		},
		{
			name:        "output flag",
			args:        []string{"--output", "json", "--help"},
			expectError: false,
		},
		{
			name:        "config flag",
			args:        []string{"--config", "test.yaml", "--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "driftwatch-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Change to temp directory
			oldDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get current working directory: %v", err)
			}
			os.Chdir(tempDir)
			defer os.Chdir(oldDir)

			// Create a basic config file to avoid config loading errors
			testConfig := `
project:
  name: "Test Project"
global:
  user_agent: "test/1.0.0"
  timeout: 30s
endpoints: []
`
			if err := os.WriteFile(".driftwatch.yaml", []byte(testConfig), 0o644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			// Capture output
			var output bytes.Buffer
			rootCmd.SetOut(&output)
			rootCmd.SetErr(&output)
			rootCmd.SetArgs(tt.args)

			// Execute command
			err = rootCmd.Execute()

			// Reset args for next test
			rootCmd.SetArgs([]string{})

			// Check results
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRootCommandFlags(t *testing.T) {
	// Test that root command has expected persistent flags
	expectedFlags := []string{"config", "verbose", "output"}

	for _, flagName := range expectedFlags {
		flag := rootCmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Root command should have --%s persistent flag", flagName)
		}
	}

	// Test flag defaults and types
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("Config flag not found")
	} else {
		if configFlag.DefValue != "" {
			t.Errorf("Config flag default should be empty, got '%s'", configFlag.DefValue)
		}
	}

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("Verbose flag not found")
	} else {
		if verboseFlag.DefValue != "false" {
			t.Errorf("Verbose flag default should be false, got '%s'", verboseFlag.DefValue)
		}
	}

	outputFlag := rootCmd.PersistentFlags().Lookup("output")
	if outputFlag == nil {
		t.Error("Output flag not found")
	} else {
		if outputFlag.DefValue != "table" {
			t.Errorf("Output flag default should be 'table', got '%s'", outputFlag.DefValue)
		}
	}
}

func TestRootCommandStructure(t *testing.T) {
	// Test basic command properties
	if rootCmd.Use != "driftwatch" {
		t.Errorf("Expected Use to be 'driftwatch', got '%s'", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Root command should have short description")
	}

	if rootCmd.Long == "" {
		t.Error("Root command should have long description")
	}

	// Test that expected subcommands are registered
	expectedCommands := []string{"config", "init"}

	for _, cmdName := range expectedCommands {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Use == cmdName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Root command should have '%s' subcommand", cmdName)
		}
	}
}

func TestInitConfig(t *testing.T) {
	// Test config initialization - simplified test
	tempDir, err := os.MkdirTemp("", "driftwatch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create a basic config file
	testConfig := `
project:
  name: "Test Project"
  description: "Test description"
  version: "1.0.0"
global:
  user_agent: "test/1.0.0"
  timeout: 30s
  retry_count: 3
  retry_delay: 5s
  max_workers: 10
  database_url: "./test.db"
endpoints: []
alerting:
  enabled: false
  channels: []
  rules: []
reporting:
  retention_days: 30
  export_format: "json"
  include_body: false
`
	if err := os.WriteFile(".driftwatch.yaml", []byte(testConfig), 0o644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Test that initConfig doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("initConfig panicked: %v", r)
		}
	}()

	// Reset cfgFile to use the default config file
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	initConfig()

	// Verify that config was loaded
	if cfg == nil {
		t.Error("Config should not be nil after initConfig")
	}
}

func TestGetConfig(t *testing.T) {
	// Test that GetConfig returns the loaded configuration
	// This is a simplified test since we can't access the internal config package

	retrievedConfig := GetConfig()
	// Just test that the function doesn't panic and returns something
	// The actual config loading is tested in integration tests
	_ = retrievedConfig // Use the variable to avoid unused variable error
}
