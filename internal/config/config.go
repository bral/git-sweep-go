// Package config handles loading, saving, and defining the application's configuration.
package config

import (
	"errors" // Import errors package
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ErrConfigNotFound is returned by LoadConfig when no config file is found.
var ErrConfigNotFound = errors.New("configuration file not found")

const (
	defaultConfigDir  = "git-sweep"
	defaultConfigFile = "config.toml"
	defaultAgeDays    = 90
	defaultMainBranch = "main"
)

// Config holds the application configuration settings.
// Tags correspond to the keys in the TOML configuration file.
type Config struct {
	AgeDays           int      `toml:"age_days"`
	PrimaryMainBranch string   `toml:"primary_main_branch"`
	ProtectedBranches []string `toml:"protected_branches"`

	// Internal map for faster lookups, not loaded from TOML directly
	ProtectedBranchMap map[string]bool `toml:"-"`
}

// DefaultConfig returns a Config struct with default values.
func DefaultConfig() Config {
	return Config{
		AgeDays:            defaultAgeDays,
		PrimaryMainBranch:  defaultMainBranch,
		ProtectedBranches:  []string{}, // Default is empty list
		ProtectedBranchMap: make(map[string]bool),
	}
}

// LoadConfig loads configuration from the specified path or the default location.
// If a custom path is provided and exists, it's used. Otherwise, it checks the default path.
// If neither exists, it returns default settings and ErrConfigNotFound.
// It also populates the ProtectedBranchMap.
func LoadConfig(customPath string) (Config, error) {
	cfg := DefaultConfig()
	configPath := ""
	configFound := false

	// Determine the path to load
	if customPath != "" {
		// If a custom path is provided, use it exclusively.
		configPath = customPath
		if _, err := os.Stat(customPath); err != nil {
			if os.IsNotExist(err) {
				// Custom path provided but doesn't exist
				return cfg, ErrConfigNotFound
			}
			// Other error checking custom path
			return cfg, fmt.Errorf("error checking custom config path %q: %w", customPath, err)
		}
		configFound = true // Custom path exists
	} else {
		// No custom path, check the default location.
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			// Cannot determine user config dir, return defaults and ErrConfigNotFound
			// (as the default path cannot be checked)
			return cfg, ErrConfigNotFound
		}
		configPath = filepath.Join(userConfigDir, defaultConfigDir, defaultConfigFile)
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				// Default path doesn't exist
				return cfg, ErrConfigNotFound
			}
			// Other error checking default path
			return cfg, fmt.Errorf("error checking default config path %q: %w", configPath, err)
		}
		configFound = true // Default path exists
	}

	// If we reach here, configFound must be true and configPath is set.
	// Load the configuration file.
	if configFound { // This check is slightly redundant now but kept for clarity
		if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
			return cfg, fmt.Errorf("error decoding config file %q: %w", configPath, err)
		}
		// Ensure defaults are applied if values are missing or invalid in the file
		if cfg.AgeDays <= 0 {
			cfg.AgeDays = defaultAgeDays
		}
		if cfg.PrimaryMainBranch == "" {
			cfg.PrimaryMainBranch = defaultMainBranch
		}
		// ProtectedBranches defaults to empty slice if nil
		if cfg.ProtectedBranches == nil {
			cfg.ProtectedBranches = []string{}
		}
	} else {
		// Config file not found at either custom or default path.
		// Return defaults and the specific ErrConfigNotFound error.
		return cfg, ErrConfigNotFound
	}

	// 4. Populate the ProtectedBranchMap
	cfg.ProtectedBranchMap = make(map[string]bool)
	for _, branch := range cfg.ProtectedBranches {
		cfg.ProtectedBranchMap[branch] = true
	}

	return cfg, nil
}

// SaveConfig saves the provided configuration to the specified path or the default location.
// It creates the necessary directories if they don't exist.
// SaveConfig writes the given configuration to a TOML file.
// If a custom file path is provided, it uses that; otherwise, it constructs
// a default path using the user's configuration directory along with preset
// directory and file names. It ensures that the target directory exists (with
// permissions 0o750), encodes the configuration (omitting the internal map)
// into TOML, and writes it to the file. The function returns the file path and
// any error encountered, including errors during file creation, encoding, or
// file closure.
func SaveConfig(cfg Config, customPath string) (string, error) {
	savePath := ""

	if customPath != "" {
		savePath = customPath
	} else {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("could not determine user config directory: %w", err)
		}
		savePath = filepath.Join(userConfigDir, defaultConfigDir, defaultConfigFile)
	}

	// Ensure the directory exists
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o750); err != nil { // Use 0750 for permissions (owner rwx, group rx, others ---)
		return savePath, fmt.Errorf("could not create config directory %q: %w", dir, err)
	}

	// Create or truncate the file
	file, err := os.Create(savePath)
	if err != nil {
		return savePath, fmt.Errorf("could not create config file %q: %w", savePath, err)
	}
	// Defer closing the file, checking for errors on close.
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			// If no previous error occurred, but closing failed, report the close error.
			err = fmt.Errorf("failed to close config file %q: %w", savePath, closeErr)
		}
	}()

	// Encode the config to TOML
	encoder := toml.NewEncoder(file)
	// We don't want to save the internal map
	configToSave := struct {
		AgeDays           int      `toml:"age_days"`
		PrimaryMainBranch string   `toml:"primary_main_branch"`
		ProtectedBranches []string `toml:"protected_branches"`
	}{
		AgeDays:           cfg.AgeDays,
		PrimaryMainBranch: cfg.PrimaryMainBranch,
		ProtectedBranches: cfg.ProtectedBranches,
	}

	if err := encoder.Encode(configToSave); err != nil {
		return savePath, fmt.Errorf("could not encode config to TOML file %q: %w", savePath, err)
	}

	return savePath, nil
}
