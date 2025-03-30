package gitcmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	// Assuming 'git-sweep-go' is the module name defined in go.mod
	// If it's different, this path will need adjustment.
	"github.com/bral/git-sweep-go/internal/types"
)

const (
	cmdForEachRef = "for-each-ref"
	// Format: branchname<NULL>upstream:short<NULL>upstream:remotename<NULL>committerdate:iso8601<NULL>objectname<NEWLINE>
	// Using NULL character (\x00) as the field separator and newline (\n) as the record separator.
	branchInfoFormat = "%(refname:short)%00" +
		"%(upstream:short)%00" +
		"%(upstream:remotename)%00" +
		"%(committerdate:iso8601)%00" +
		"%(objectname)"
	fieldSeparator  = "\x00" // Null character
	detachedHeadStr = "HEAD" // Constant for detached HEAD string
)

// GetAllLocalBranchInfo retrieves details of all local Git branches by executing the "for-each-ref" command.
// It parses the command output into BranchInfo structures that include the branch name, upstream configuration,
// remote, last commit date, and commit hash. An empty slice is returned if no branches are found, and an error
// is returned if the Git command fails. Malformed records or branches with unparseable dates are skipped.
func GetAllLocalBranchInfo(ctx context.Context) ([]types.BranchInfo, error) {
	args := []string{
		cmdForEachRef,
		"refs/heads/",
		fmt.Sprintf("--format=%s", branchInfoFormat),
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

	// Split the output into records based on the newline character
	records := strings.Split(output, "\n")
	// Pre-allocate slice with estimated capacity
	branches := make([]types.BranchInfo, 0, len(records))

	for _, record := range records {
		// Skip empty records which might occur (though less likely with newline separator)
		if record == "" {
			continue
		}

		// Split each record into fields based on the Null character
		fields := strings.Split(record, fieldSeparator)
		if len(fields) != 5 {
			// This indicates unexpected output format from git. Log or handle appropriately.
			// For now, print a warning to stderr and skip the malformed record.
			// TODO: Replace with proper logging
			_, _ = fmt.Fprintf(os.Stderr,
				"warning: skipping malformed branch record from git (expected 5 fields, got %d): %q\n",
				len(fields), record) // Use Fprintf to os.Stderr
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
			// TODO: Replace with proper logging
			_, _ = fmt.Fprintf(os.Stderr,
				"warning: skipping branch %q due to date parse error ('%s'): %v\n",
				name, dateStr, err) // Use Fprintf to os.Stderr
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

// GetCurrentBranchName retrieves the name of the currently checked-out branch.
// GetCurrentBranchName returns the name of the currently checked out Git branch.
// It first attempts to retrieve the branch name using "git branch --show-current". If this command fails
// due to an unsupported Git version, it falls back to "git rev-parse --abbrev-ref HEAD". If a detached HEAD is
// detected or no branch name is found, the function returns an empty string. Any errors encountered during
// command execution are wrapped and returned.
func GetCurrentBranchName(ctx context.Context) (string, error) {
	// `git branch --show-current` is simpler and preferred if available (Git 2.22+)
	// `git rev-parse --abbrev-ref HEAD` is a fallback that works on older versions
	// but returns "HEAD" if detached.
	args := []string{"branch", "--show-current"}
	branchName, err := RunGitCommand(ctx, args...)
	if err != nil {
		// If `git branch --show-current` fails, try the fallback
		// Check if the error indicates the command is unknown (older Git)
		errStr := err.Error()
		if strings.Contains(errStr, "unknown switch `show-current'") ||
			strings.Contains(errStr, "unknown option `show-current'") {
			args = []string{"rev-parse", "--abbrev-ref", "HEAD"}
			branchName, err = RunGitCommand(ctx, args...)
			if err != nil {
				return "", fmt.Errorf("failed to get current branch using fallback rev-parse: %w", err)
			}
			// If HEAD is detached, rev-parse returns "HEAD"
			if branchName == detachedHeadStr {
				return "", nil // Treat detached HEAD as no current branch for our purposes
			}
			return branchName, nil
		}
		// Other error with `git branch --show-current`
		return "", fmt.Errorf("failed to get current branch using branch --show-current: %w", err)
	}

	// Handle potential empty output from `git branch --show-current` (e.g., detached HEAD in some cases)
	if branchName == "" {
		return "", nil
	}

	return branchName, nil
}

// areChangesIncludedFunc defines the signature for the function.
type areChangesIncludedFunc func(ctx context.Context, upstreamBranch, headBranch string) (bool, error)

// AreChangesIncluded is a variable holding the implementation, allowing mocking.
// It checks if all changes in headBranch are included in upstreamBranch using 'git cherry -v'.
var AreChangesIncluded areChangesIncludedFunc = areChangesIncludedImpl

// areChangesIncludedImpl is the actual implementation.
func areChangesIncludedImpl(ctx context.Context, upstreamBranch, headBranch string) (bool, error) {
	if upstreamBranch == "" || headBranch == "" {
		return false, fmt.Errorf("upstream and head branch names cannot be empty for cherry check")
	}

	// Ensure branches exist locally before running cherry? Maybe not necessary, cherry might handle it.
	// Consider adding checks if needed.

	args := []string{"cherry", "-v", upstreamBranch, headBranch}
	// Use the global Runner variable which might be mocked by tests
	output, err := Runner(ctx, args...)
	if err != nil {
		// Handle specific errors? e.g., unknown branch?
		// For now, wrap the generic error.
		// TODO: Consider checking stderr for specific git errors like "unknown upstream"
		return false, fmt.Errorf("failed to run git cherry for %s..%s: %w", upstreamBranch, headBranch, err)
	}

	// If output is empty, it means no commits are unique to headBranch relative to upstreamBranch.
	// This implies all changes are included.
	if strings.TrimSpace(output) == "" {
		return true, nil
	}

	// Check if any line starts with '+', indicating a commit unique to headBranch.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "+") {
			// Found a commit in headBranch not present in upstreamBranch
			return false, nil
		}
		// Lines starting with '-' mean the commit is equivalent to one in upstream.
		// Empty lines can be ignored.
	}

	// If we looped through all lines and found no '+', all changes are included.
	return true, nil
}
