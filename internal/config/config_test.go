package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bral/git-sweep-go/internal/config"
)

func TestLoadConfig_NotFound(t *testing.T) {
	// Test loading when no config file exists (custom or default)
	// We expect nil config and ErrConfigNotFound
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "nonexistent.toml")

	cfg, err := config.LoadConfig(nonExistentPath)
	if !errors.Is(err, config.ErrConfigNotFound) {
		t.Errorf("Expected ErrConfigNotFound, got: %v", err)
	}
	if cfg != nil {
		t.Errorf("Expected nil config when file not found, got: %+v", cfg)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "test_config.yaml")

	// 1. Create a config to save
	configToSave := config.Config{
		AgeDays:            60,
		PrimaryMainBranch:  "develop",
		ProtectedBranches:  []string{"main", "release/v1"},
		ProtectedBranchMap: nil, // Map should be ignored by save, populated by load
	}

	// 2. Save the config
	savedPath, err := config.SaveConfig(configToSave, customPath)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	if savedPath != customPath {
		t.Errorf("SaveConfig returned unexpected path: got %q, want %q", savedPath, customPath)
	}

	// 3. Load the config back
	loadedCfg, err := config.LoadConfig(customPath)
	if err != nil {
		t.Fatalf("LoadConfig failed after save: %v", err)
	}

	// 4. Compare only persisted fields
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

	// 5. ProtectedBranchMap should match loaded ProtectedBranches
	expectedMap := make(map[string]bool)
	for _, branch := range loadedCfg.ProtectedBranches {
		expectedMap[branch] = true
	}
	if !reflect.DeepEqual(loadedCfg.ProtectedBranchMap, expectedMap) {
		t.Errorf("Loaded ProtectedBranchMap mismatch: got %v, want %v", loadedCfg.ProtectedBranchMap, expectedMap)
	}
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "partial_config.yaml")

	// Create a config file with missing/invalid values (YAML)
	partialContent := `age_days: 0
primary_main_branch: ""
# protected_branches is omitted, should use default
`
	err := os.WriteFile(customPath, []byte(partialContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write partial config file: %v", err)
	}

	loadedCfg, err := config.LoadConfig(customPath)
	if err != nil {
		t.Fatalf("LoadConfig failed for partial config: %v", err)
	}

	// Only omitted fields get defaults; present fields (even zero/empty) are preserved
	if loadedCfg.AgeDays != 0 {
		t.Errorf("Expected AgeDays 0 (explicit in YAML), got %d", loadedCfg.AgeDays)
	}
	if loadedCfg.PrimaryMainBranch != "" {
		t.Errorf("Expected PrimaryMainBranch empty (explicit in YAML), got %q", loadedCfg.PrimaryMainBranch)
	}
	expectedBranches := []string{"main", "master", "develop"}
	if !reflect.DeepEqual(loadedCfg.ProtectedBranches, expectedBranches) {
		t.Errorf("Expected default ProtectedBranches %v, got %v", expectedBranches, loadedCfg.ProtectedBranches)
	}
	// ProtectedBranchMap should match loaded ProtectedBranches
	expectedMap := make(map[string]bool)
	for _, branch := range loadedCfg.ProtectedBranches {
		expectedMap[branch] = true
	}
	if !reflect.DeepEqual(loadedCfg.ProtectedBranchMap, expectedMap) {
		t.Errorf("Expected ProtectedBranchMap to match ProtectedBranches, got %v", loadedCfg.ProtectedBranchMap)
	}
}

func TestLoadConfig_InvalidYaml(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `age_days: 90
primary_main_branch: "main` // Missing closing quote
	err := os.WriteFile(customPath, []byte(invalidContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = config.LoadConfig(customPath)
	if err == nil {
		t.Errorf("Expected an error when loading invalid YAML, got nil")
	}
	// We could check for a specific error type from the yaml library if needed,
	// but for now, just ensuring an error occurred is sufficient.
}

// Note: Testing the default path loading (~/.config/...) is tricky in unit tests
// as it involves the actual user's filesystem. It's often better tested manually
// or via integration tests. The logic is largely shared with custom path loading.
