package gitcmd

import (
	"context"
	"fmt"
)

// FetchAndPrune runs 'git fetch <remote> --prune' to update local refs
// and remove any stale remote-tracking branches.
// It returns an error if the command fails, but the plan suggests treating
// this as a warning rather than a fatal error in the main application flow.
func FetchAndPrune(ctx context.Context, remoteName string) error {
	if remoteName == "" {
		return fmt.Errorf("remote name cannot be empty for fetch --prune")
	}

	args := []string{"fetch", remoteName, "--prune"}

	_, err := RunGitCommand(ctx, args...)
	if err != nil {
		// Wrap the error with more context.
		// The caller can decide how to handle this (e.g., log a warning).
		return fmt.Errorf("failed to fetch and prune remote %q: %w", remoteName, err)
	}

	return nil
}
