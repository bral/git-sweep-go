package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// FirstRunSetup prompts the user for initial configuration values when no config file is found.
// It takes an input reader and output writer for flexibility (e.g., testing).
// It returns the generated Config struct based on user input or defaults.
func FirstRunSetup(reader *bufio.Reader, writer io.Writer) (Config, error) {
	// Ignore bytes written and error
	_, _ = fmt.Fprintln(writer, "Configuration file not found. Let's set up some defaults.")
	cfg := DefaultConfig() // Start with defaults

	// Prompt for Age Days
	// Ignore bytes written and error
	_, _ = fmt.Fprintf(writer,
		"Enter max age (in days) for unmerged branches to be considered old [%d]: ", defaultAgeDays)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		age, err := strconv.Atoi(input)
		if err != nil || age <= 0 {
			// Ignore bytes written and error
			_, _ = fmt.Fprintf(writer, "Invalid input. Using default age: %d days.\n", defaultAgeDays)
			// Keep default age
		} else {
			cfg.AgeDays = age
		}
	} // else keep default

	// Prompt for Primary Main Branch
	// Ignore bytes written and error
	_, _ = fmt.Fprintf(writer,
		"Enter the name of your primary development branch (e.g., main, master) [%s]: ",
		defaultMainBranch)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" { // Add missing opening brace
		// TODO: Add validation? Check if branch exists? For now, just accept input.
		cfg.PrimaryMainBranch = input
	} // else keep default

	// Prompt for Protected Branches
	_, _ = fmt.Fprint(writer, "Enter any branches to protect from deletion ") // Ignore bytes written and error
	_, _ = fmt.Fprintln(writer, "(comma-separated, e.g., develop,release): ") // Ignore bytes written and error
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		protected := strings.Split(input, ",")
		cfg.ProtectedBranches = make([]string, 0, len(protected)) // Initialize slice
		for _, p := range protected {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cfg.ProtectedBranches = append(cfg.ProtectedBranches, trimmed)
			}
		}
	} // else keep default (empty list)

	// Prompt for Merge Strategy
	_, _ = fmt.Fprintln(writer, "\nSelect merge detection strategy:")
	_, _ = fmt.Fprintln(writer, "  1. Standard - Use Git's native 'branch --merged' only (Recommended)")
	_, _ = fmt.Fprintln(writer, "     This detects branches that Git considers fully merged.")
	_, _ = fmt.Fprintln(writer, "     Branches can be deleted with 'git branch -d'.")
	_, _ = fmt.Fprintln(writer, "  2. Enhanced - Also detect using Git cherry-pick check")
	_, _ = fmt.Fprintln(writer, "     This can find branches with equivalent changes.")
	_, _ = fmt.Fprintln(writer, "     Some branches will require 'git branch -D' to delete.")
	_, _ = fmt.Fprintf(writer, "Choose strategy [1]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "2" {
		cfg.MergeStrategy = MergeStrategyEnhanced
		_, _ = fmt.Fprintln(writer, "Using Enhanced merge detection.")
	} else {
		// Default to standard for empty input or any non-2 value
		cfg.MergeStrategy = MergeStrategyStandard
		_, _ = fmt.Fprintln(writer, "Using Standard merge detection.")
	}

	// Populate the map based on the final list
	cfg.ProtectedBranchMap = make(map[string]bool)
	for _, branch := range cfg.ProtectedBranches {
		cfg.ProtectedBranchMap[branch] = true
	}

	_, _ = fmt.Fprintln(writer, "\nConfiguration setup complete.") // Ignore bytes written and error
	// The caller will be responsible for saving this config and informing the user.

	return cfg, nil
}
