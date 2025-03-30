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
// FirstRunSetup prompts the user for initial configuration values and returns a Config struct populated with either user input or default settings.
//
// It notifies the user that no configuration file was found and then requests input for the maximum age (in days) for unmerged branches,
// the primary development branch name, and any branches to protect from deletion (as a comma-separated list). Invalid or empty inputs 
// result in the retention of default values. The function also constructs a map for protected branches for efficient lookups.
// The returned Config should be persisted by the caller.
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

	// Populate the map based on the final list
	cfg.ProtectedBranchMap = make(map[string]bool)
	for _, branch := range cfg.ProtectedBranches {
		cfg.ProtectedBranchMap[branch] = true
	}

	_, _ = fmt.Fprintln(writer, "\nConfiguration setup complete.") // Ignore bytes written and error
	// The caller will be responsible for saving this config and informing the user.

	return cfg, nil
}
