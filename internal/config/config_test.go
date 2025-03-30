package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfig_NotFound(t *testing.T) {
	// Test loading when no config file exists (custom or default)
	// We expect default config and ErrConfigNotFound
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "nonexistent.toml")

	cfg, err := LoadConfig(nonExistentPath)

	if !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("Expected ErrConfigNotFound, got: %v", err)
	}

	defaultCfg := DefaultConfig()
	if !reflect.DeepEqual(cfg, defaultCfg) {
		t.Errorf("Expected default config when file not found, got %+v", cfg)
	}
	// Check map initialization specifically
	if cfg.ProtectedBranchMap == nil {
		t.Error("Expected ProtectedBranchMap to be initialized even on default config, got nil")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "test_config.toml")

	// 1. Create a config to save
	configToSave := Config{
		AgeDays:            60,
		PrimaryMainBranch:  "develop",
		ProtectedBranches:  []string{"main", "release/v1"},
		ProtectedBranchMap: nil, // Map should be ignored by save, populated by load
	}

	// 2. Save the config
	savedPath, err := SaveConfig(configToSave, customPath)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	if savedPath != customPath {
		t.Errorf("SaveConfig returned unexpected path: got %q, want %q", savedPath, customPath)
	}

	// 3. Load the config back
	loadedCfg, err := LoadConfig(customPath)
	if err != nil {
		t.Fatalf("LoadConfig failed after save: %v", err)
	}

	// 4. Verify the loaded config matches the saved one (excluding the map)
	if loadedCfg.AgeDays != configToSave.AgeDays {
		t.Errorf("Loaded AgeDays mismatch: got %d, want %d", loadedCfg.AgeDays, configToSave.AgeDays)
	}
	if loadedCfg.PrimaryMainBranch != configToSave.PrimaryMainBranch {
		t.Errorf("Loaded PrimaryMainBranch mismatch: got %q, want %q",
			loadedCfg.PrimaryMainBranch, configToSave.PrimaryMainBranch)
	}
	if !reflect.DeepEqual(loadedCfg.ProtectedBranches, configToSave.ProtectedBranches) {
		t.Errorf("Loaded ProtectedBranches mismatch: got %v, want %v",
			loadedCfg.ProtectedBranches, configToSave.ProtectedBranches)
	}

	// 5. Verify the ProtectedBranchMap was populated correctly by LoadConfig
	expectedMap := map[string]bool{"main": true, "release/v1": true}
	if !reflect.DeepEqual(loadedCfg.ProtectedBranchMap, expectedMap) {
		t.Errorf("Loaded ProtectedBranchMap mismatch: got %v, want %v", loadedCfg.ProtectedBranchMap, expectedMap)
	}
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "partial_config.toml")

	// Create a config file with missing/invalid values
	partialContent := `
# age_days = 0 # Invalid, should use default
primary_main_branch = "" # Empty, should use default
# protected_branches is omitted, should use default empty slice
`
	err := os.WriteFile(customPath, []byte(partialContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write partial config file: %v", err)
	}

	loadedCfg, err := LoadConfig(customPath)
	if err != nil {
		t.Fatalf("LoadConfig failed for partial config: %v", err)
	}

	// Check if defaults were applied
	if loadedCfg.AgeDays != defaultAgeDays {
		t.Errorf("Expected default AgeDays %d, got %d", defaultAgeDays, loadedCfg.AgeDays)
	}
	if loadedCfg.PrimaryMainBranch != defaultMainBranch {
		t.Errorf("Expected default PrimaryMainBranch %q, got %q", defaultMainBranch, loadedCfg.PrimaryMainBranch)
	}
	if len(loadedCfg.ProtectedBranches) != 0 {
		t.Errorf("Expected empty ProtectedBranches slice, got %v", loadedCfg.ProtectedBranches)
	}
	if len(loadedCfg.ProtectedBranchMap) != 0 {
		t.Errorf("Expected empty ProtectedBranchMap, got %v", loadedCfg.ProtectedBranchMap)
	}
}

func TestLoadConfig_InvalidToml(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "invalid.toml")

	invalidContent := `age_days = 90\nprimary_main_branch = "main` // Missing closing quote
	err := os.WriteFile(customPath, []byte(invalidContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = LoadConfig(customPath)
	if err == nil {
		t.Errorf("Expected an error when loading invalid TOML, got nil")
	}
	// We could check for a specific error type from the toml library if needed,
	// but for now, just ensuring an error occurred is sufficient.
}

// Note: Testing the default path loading (~/.config/...) is tricky in unit tests
// as it involves the actual user's filesystem. It's often better tested manually
// or via integration tests. The logic is largely shared with custom path loading.
