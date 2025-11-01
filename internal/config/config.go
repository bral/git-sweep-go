// Package config handles loading, saving, and defining the application's configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const defaultAgeDays = 90        // Default max age for unmerged branches (days)
const defaultMainBranch = "main" // Default primary development branch

// Config holds the application configuration
// All fields must be kept in sync with usage in main.go, setup.go, analyze.go, and tests.
type Config struct {
	// General settings
	Debug bool `mapstructure:"debug" yaml:"debug"`

	// Branch protection settings
	ProtectedBranches  []string        `mapstructure:"protected_branches" yaml:"protected_branches"`
	ProtectedBranchMap map[string]bool `mapstructure:"-" yaml:"-"` // Not loaded from file, populated at runtime

	// Cleanup settings
	MinBranchAge int  `mapstructure:"min_branch_age" yaml:"min_branch_age"` // Min age in days
	AgeDays      int  `mapstructure:"age_days" yaml:"age_days"`             // Legacy compat field
	DeleteRemote   bool `mapstructure:"delete_remote" yaml:"delete_remote"`
	RequireConfirm bool `mapstructure:"require_confirm" yaml:"require_confirm"`

	// Main branch settings
	PrimaryMainBranch string `mapstructure:"primary_main_branch" yaml:"primary_main_branch"`

	// Version tracking
	LastVersionCheck   int64  `mapstructure:"last_version_check" yaml:"last_version_check"`
	LatestKnownVersion string `mapstructure:"latest_known_version" yaml:"latest_known_version"`
}

// ErrConfigNotFound is returned when no config file is found.
var ErrConfigNotFound = errors.New("configuration file not found")

// LoadConfig loads the configuration from the given path (if provided) or the default location.
// Returns ErrConfigNotFound if no config file is found.
func LoadConfig(customPath string) (*Config, error) {
	config := &Config{}

	if customPath != "" {
		viper.SetConfigFile(customPath)
		// Let viper auto-detect the file type from the extension
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		configDir := getConfigDir()
		viper.AddConfigPath(configDir)
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		// Check for viper's ConfigFileNotFoundError or any wrapped os.ErrNotExist
		if errors.As(err, &configFileNotFoundError) {
			return nil, ErrConfigNotFound
		}
		// Check if the underlying error is a file not found error
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigNotFound
		}
		return nil, errors.New("error reading config file: " + err.Error())
	}

	viper.SetEnvPrefix("GIT_SWEEP")
	viper.AutomaticEnv()

	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.New("error unmarshaling config: " + err.Error())
	}

	// Post-process: apply defaults only for fields missing from YAML
	present := map[string]bool{}
	for _, k := range viper.AllKeys() {
		present[k] = true
	}
	if !present["age_days"] && config.AgeDays == 0 {
		config.AgeDays = defaultAgeDays
	}
	if !present["primary_main_branch"] && config.PrimaryMainBranch == "" {
		config.PrimaryMainBranch = defaultMainBranch
	}
	if !present["protected_branches"] && len(config.ProtectedBranches) == 0 {
		config.ProtectedBranches = []string{"main", "master", "develop"}
	}
	if config.ProtectedBranchMap == nil {
		config.ProtectedBranchMap = make(map[string]bool)
		for _, branch := range config.ProtectedBranches {
			config.ProtectedBranchMap[branch] = true
		}
	}
	return config, nil
}

func getConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(configDir, "git-sweep-go")
}

// SaveConfig saves the provided configuration to the specified path or the default location.
// It creates the necessary directories if they don't exist.
// It returns the path where the file was saved and any error encountered.
func SaveConfig(cfg Config, customPath string) (string, error) {
	savePath := ""

	if customPath != "" {
		savePath = customPath
	} else {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("could not determine user config directory: %w", err)
		}
		savePath = filepath.Join(userConfigDir, "git-sweep-go", "config.yaml")
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

	// Encode the config to YAML
	encoder := yaml.NewEncoder(file)
	if err := encoder.Encode(cfg); err != nil {
		return savePath, fmt.Errorf("could not encode config to YAML file %q: %w", savePath, err)
	}

	return savePath, nil
}

// DefaultConfig returns a Config struct with all default values set.
func DefaultConfig() Config {
	return Config{
		Debug:              false,
		ProtectedBranches:  []string{"main", "master", "develop"},
		ProtectedBranchMap: make(map[string]bool),
		MinBranchAge:       30,
		AgeDays:            defaultAgeDays,
		DeleteRemote:       false,
		RequireConfirm:     true,
		PrimaryMainBranch:  defaultMainBranch,
		LastVersionCheck:   0,
		LatestKnownVersion: "",
	}
}
