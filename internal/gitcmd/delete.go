// Package gitcmd provides functions for interacting with the git command-line tool.
package gitcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bral/git-sweep-go/internal/types"
)

// BranchToDelete holds information needed to delete a specific branch.
type BranchToDelete struct {
	Name     string
	IsRemote bool
	Remote   string // Only used if IsRemote is true
	IsMerged bool   // Used to determine -d vs -D for local delete
	Hash     string // Potentially useful for logging/confirmation
}

// DeleteBranches attempts to delete the specified local and remote branches.
// It takes a slice of BranchToDelete structs and returns a slice of DeleteResult
// DeleteBranches deletes the specified local and remote Git branches.
// It iterates over each branch in the input slice and determines the appropriate Git command based on the branch's properties.
// For remote branches, it constructs a deletion command using "git push <remote> --delete <branch>" and fails if the remote name is empty.
// For local branches, it chooses between safe deletion ("git branch -d <branch>") if the branch is merged, or forced deletion ("git branch -D <branch>") otherwise.
// When the dryRun flag is enabled, the function simulates deletion by recording the command without executing it.
// The function returns a slice of DeleteResult, each detailing the outcome of a deletion attempt, including success status, relevant messages, and the deleted branch's hash when applicable.
func DeleteBranches(ctx context.Context, branches []BranchToDelete, dryRun bool) []types.DeleteResult {
	results := make([]types.DeleteResult, 0, len(branches))

	for _, branch := range branches {
		var cmdArgs []string
		var cmdString string // For logging/result
		var result types.DeleteResult

		result.BranchName = branch.Name
		result.IsRemote = branch.IsRemote
		result.RemoteName = branch.Remote

		if branch.IsRemote {
			// Remote deletion
			if branch.Remote == "" {
				result.Success = false
				result.Message = "Cannot delete remote branch: remote name is empty"
				results = append(results, result)
				continue
			}
			cmdArgs = []string{"push", branch.Remote, "--delete", branch.Name}
			cmdString = fmt.Sprintf("git push %s --delete %s", branch.Remote, branch.Name)
		} else {
			// Local deletion
			if branch.IsMerged {
				cmdArgs = []string{"branch", "-d", branch.Name} // Safe delete
				cmdString = fmt.Sprintf("git branch -d %s", branch.Name)
			} else {
				cmdArgs = []string{"branch", "-D", branch.Name} // Force delete
				cmdString = fmt.Sprintf("git branch -D %s", branch.Name)
			}
		}
		result.Cmd = cmdString

		if dryRun {
			result.Success = true // Indicate success in dry-run context
			result.Message = fmt.Sprintf("Dry Run: Would execute: %s", cmdString)
			results = append(results, result)
			continue
		}

		// Execute the actual command
		_, err := RunGitCommand(ctx, cmdArgs...)
		if err != nil {
			result.Success = false
			// Attempt to extract a cleaner error message from the potentially multi-line stderr
			errMsg := err.Error()
			if strings.Contains(errMsg, "stderr:") {
				parts := strings.SplitN(errMsg, "stderr:", 2)
				if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
					errMsg = strings.TrimSpace(parts[1])
				}
			}
			result.Message = fmt.Sprintf("Failed: %s", errMsg)
		} else {
			result.Success = true
			result.Message = "Successfully deleted"
			// Store the hash of the deleted branch for potential recovery info
			result.DeletedHash = branch.Hash
		}
		results = append(results, result)
	}

	return results
}
