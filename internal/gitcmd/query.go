package gitcmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	// Assuming 'git-sweep-go' is the module name defined in go.mod
	// If it's different, this path will need adjustment.
	"github.com/bral/git-sweep-go/internal/types"
)

const (
	// Format: branchname<NULL>upstream:short<NULL>upstream:remotename<NULL>committerdate:iso8601<NULL>objectname<NEWLINE>
	// Using NULL character (\x00) as the field separator and newline (\n) as the record separator.
	branchInfoFormat = "%(refname:short)%00%(upstream:short)%00%(upstream:remotename)%00%(committerdate:iso8601)%00%(objectname)"
	fieldSeparator   = "\x00" // Null character
)

// GetAllLocalBranchInfo fetches details for all local branches using git for-each-ref.
// It parses the output into a slice of BranchInfo structs.
func GetAllLocalBranchInfo(ctx context.Context) ([]types.BranchInfo, error) {
	args := []string{
		"for-each-ref",
		"refs/heads/", // Limit to local branches
		"--format=" + branchInfoFormat,
	}

	// Execute the git command using the helper function
	output, err := RunGitCommand(ctx, args...)
	if err != nil {
		// Check if the error indicates no refs were found (e.g., new repo)
		// A more robust check might involve specific error types or exit codes if possible.
		// For now, we assume any error is potentially significant.
		// TODO: Improve error handling to distinguish "no branches" from other git errors.
		return nil, fmt.Errorf("failed to execute git for-each-ref: %w", err)
	}

	// If the output is empty, it means there are no local branches.
	// Trim trailing newline which might be present even if empty.
	output = strings.TrimSpace(output)
	if output == "" {
		return []types.BranchInfo{}, nil
	}

	var branches []types.BranchInfo
	// Split the output into records based on the newline character
	records := strings.Split(output, "\n")

	for _, record := range records {
		// Skip empty records which might occur (though less likely with newline separator)
		if record == "" {
			continue
		}

		// Split each record into fields based on the Null character
		fields := strings.Split(record, fieldSeparator)
		if len(fields) != 5 {
			// This indicates unexpected output format from git. Log or handle appropriately.
			// For now, print a warning and skip the malformed record.
			fmt.Printf("warning: skipping malformed branch record from git (expected 5 fields, got %d): %q\n", len(fields), record) // TODO: Replace with proper logging
			continue
		}

		// Extract fields
		name := fields[0]
		upstream := fields[1]
		remote := fields[2]
		dateStr := fields[3] // Format: "YYYY-MM-DD HH:MM:SS +/-ZZZZ"
		hash := fields[4]

		// Parse the commit date string
		commitDate, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			// Failed to parse date, skip this branch and warn.
			fmt.Printf("warning: skipping branch %q due to date parse error ('%s'): %v\n", name, dateStr, err) // TODO: Replace with proper logging
			continue
		}

		// Append the parsed branch info to the slice
		branches = append(branches, types.BranchInfo{
			Name:           name,
			Upstream:       upstream,
			Remote:         remote,
			LastCommitDate: commitDate,
			CommitHash:     hash,
		})
	}

	return branches, nil
}

// GetMainBranchHash retrieves the commit hash for the specified branch name.
func GetMainBranchHash(ctx context.Context, branchName string) (string, error) {
	if branchName == "" {
		return "", fmt.Errorf("main branch name cannot be empty")
	}
	args := []string{"rev-parse", branchName}
	hash, err := RunGitCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get hash for branch %q: %w", branchName, err)
	}
	if hash == "" {
		return "", fmt.Errorf("no hash returned for branch %q (does it exist?)", branchName)
	}
	return hash, nil
}

// GetMergedBranches returns a map of branch names that are fully merged
// into the specified commit hash. The map value is always true.
func GetMergedBranches(ctx context.Context, targetHash string) (map[string]bool, error) {
	if targetHash == "" {
		return nil, fmt.Errorf("target hash cannot be empty")
	}
	args := []string{"branch", "--merged", targetHash}
	output, err := RunGitCommand(ctx, args...)
	if err != nil {
		// If the target hash doesn't exist, git branch --merged might error.
		return nil, fmt.Errorf("failed to get merged branches for hash %q: %w", targetHash, err)
	}

	mergedBranches := make(map[string]bool)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// The current branch is marked with '* '. Remove it.
		branchName := strings.TrimPrefix(trimmedLine, "* ")
		if branchName != "" {
			mergedBranches[branchName] = true
		}
	}

	return mergedBranches, nil
}

// IsInGitRepo checks if the current directory is within a Git working tree.
func IsInGitRepo(ctx context.Context) (bool, error) {
	args := []string{"rev-parse", "--is-inside-work-tree"}
	output, err := RunGitCommand(ctx, args...)
	if err != nil {
		// If the command fails (e.g., not a git repo), stderr might contain useful info,
		// but the error itself usually indicates we're not in a repo.
		// We return false and no error in this case, as it's an expected condition.
		// A more specific error check could be added if needed.
		return false, nil
	}
	// If the command succeeds, it outputs "true".
	return output == "true", nil
}
