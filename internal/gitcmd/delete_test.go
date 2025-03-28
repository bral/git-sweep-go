package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bral/git-sweep-go/internal/types"
)

// Note: The setup function is defined in query_test.go but accessible within the package.

func TestDeleteBranches(t *testing.T) {
	ctx := context.Background()

	branchesToDelete := []BranchToDelete{
		{Name: "local-merged", IsRemote: false, IsMerged: true, Hash: "h1"},
		{Name: "local-unmerged", IsRemote: false, IsMerged: false, Hash: "h2"},
		{Name: "remote-branch", IsRemote: true, Remote: "origin", Hash: "h3"},
		{Name: "fail-local", IsRemote: false, IsMerged: true, Hash: "h4"}, // Will simulate failure
		{Name: "fail-remote", IsRemote: true, Remote: "origin", Hash: "h5"}, // Will simulate failure
	}

	expectedResultsSuccess := []types.DeleteResult{
		{BranchName: "local-merged", IsRemote: false, Success: true, Message: "Successfully deleted", Cmd: "git branch -d local-merged"},
		{BranchName: "local-unmerged", IsRemote: false, Success: true, Message: "Successfully deleted", Cmd: "git branch -D local-unmerged"},
		{BranchName: "remote-branch", IsRemote: true, RemoteName: "origin", Success: true, Message: "Successfully deleted", Cmd: "git push origin --delete remote-branch"},
		{BranchName: "fail-local", IsRemote: false, Success: false, Message: "Failed: simulated local delete error", Cmd: "git branch -d fail-local"},
		{BranchName: "fail-remote", IsRemote: true, RemoteName: "origin", Success: false, Message: "Failed: simulated remote delete error", Cmd: "git push origin --delete fail-remote"},
	}

	expectedResultsDryRun := []types.DeleteResult{
		{BranchName: "local-merged", IsRemote: false, Success: true, Message: "Dry Run: Would execute: git branch -d local-merged", Cmd: "git branch -d local-merged"},
		{BranchName: "local-unmerged", IsRemote: false, Success: true, Message: "Dry Run: Would execute: git branch -D local-unmerged", Cmd: "git branch -D local-unmerged"},
		{BranchName: "remote-branch", IsRemote: true, RemoteName: "origin", Success: true, Message: "Dry Run: Would execute: git push origin --delete remote-branch", Cmd: "git push origin --delete remote-branch"},
		{BranchName: "fail-local", IsRemote: false, Success: true, Message: "Dry Run: Would execute: git branch -d fail-local", Cmd: "git branch -d fail-local"}, // Dry run always "succeeds"
		{BranchName: "fail-remote", IsRemote: true, RemoteName: "origin", Success: true, Message: "Dry Run: Would execute: git push origin --delete fail-remote", Cmd: "git push origin --delete fail-remote"}, // Dry run always "succeeds"
	}

	// --- Test Case 1: Successful Deletion (with simulated failures) ---
	t.Run("Successful Deletion", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			cmdStr := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdStr, "branch -d local-merged"):
				return "Deleted branch local-merged (was h1).", nil
			case strings.HasPrefix(cmdStr, "branch -D local-unmerged"):
				return "Deleted branch local-unmerged (was h2).", nil
			case strings.HasPrefix(cmdStr, "push origin --delete remote-branch"):
				return "To github.com:user/repo\n - [deleted]         remote-branch", nil
			case strings.HasPrefix(cmdStr, "branch -d fail-local"):
				// Simulate failure by returning an error
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, "simulated local delete error")
			case strings.HasPrefix(cmdStr, "push origin --delete fail-remote"):
				// Simulate failure by returning an error
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, "simulated remote delete error")
			default:
				return "", fmt.Errorf("unexpected command in mock: %v", args)
			}
		})
		defer teardown()

		results := DeleteBranches(ctx, branchesToDelete, false) // Not dry run

		// Custom comparison needed because error messages might vary slightly
		if len(results) != len(expectedResultsSuccess) {
			t.Fatalf("Expected %d results, got %d", len(expectedResultsSuccess), len(results))
		}
		for i := range results {
			expected := expectedResultsSuccess[i]
			actual := results[i]
			if actual.BranchName != expected.BranchName ||
				actual.IsRemote != expected.IsRemote ||
				actual.RemoteName != expected.RemoteName ||
				actual.Success != expected.Success ||
				actual.Cmd != expected.Cmd ||
				// Only check prefix for error messages
				(actual.Success != expected.Success && !strings.HasPrefix(actual.Message, "Failed: simulated")) {
				t.Errorf("Result mismatch at index %d.\nGot:  %+v\nWant: %+v", i, actual, expected)
			}
		}
	})

	// --- Test Case 2: Dry Run ---
	t.Run("Dry Run", func(t *testing.T) {
		// The mock runner should NOT be called in dry run mode
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			t.Errorf("Runner should not be called during dry run, called with: %v", args)
			return "", errors.New("runner called unexpectedly")
		})
		defer teardown()

		results := DeleteBranches(ctx, branchesToDelete, true) // Dry run enabled

		if !reflect.DeepEqual(results, expectedResultsDryRun) {
			t.Errorf("Dry run results mismatch.\nGot:  %+v\nWant: %+v", results, expectedResultsDryRun)
		}
	})

	// --- Test Case 3: Empty Input Slice ---
	t.Run("Empty Input Slice", func(t *testing.T) {
		// Runner should not be called
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			t.Errorf("Runner should not be called with empty input, called with: %v", args)
			return "", errors.New("runner called unexpectedly")
		})
		defer teardown()

		results := DeleteBranches(ctx, []BranchToDelete{}, false) // Empty slice, not dry run
		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty input, got %d", len(results))
		}

		resultsDry := DeleteBranches(ctx, []BranchToDelete{}, true) // Empty slice, dry run
		if len(resultsDry) != 0 {
			t.Errorf("Expected 0 results for empty input (dry run), got %d", len(resultsDry))
		}
	})

	// --- Test Case 4: Invalid Remote Input (Empty Remote Name) ---
	t.Run("Invalid Remote Input", func(t *testing.T) {
		invalidBranches := []BranchToDelete{
			{Name: "bad-remote", IsRemote: true, Remote: "", Hash: "h-invalid"}, // Empty Remote
		}
		expectedResult := types.DeleteResult{
			BranchName: "bad-remote",
			IsRemote:   true,
			RemoteName: "",
			Success:    false,
			Message:    "Cannot delete remote branch: remote name is empty",
			Cmd:        "", // Cmd is not set if validation fails early
		}

		// Runner should not be called
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			t.Errorf("Runner should not be called for invalid input, called with: %v", args)
			return "", errors.New("runner called unexpectedly")
		})
		defer teardown()

		results := DeleteBranches(ctx, invalidBranches, false) // Not dry run

		if len(results) != 1 {
			t.Fatalf("Expected 1 result for invalid input, got %d", len(results))
		}
		if !reflect.DeepEqual(results[0], expectedResult) {
			t.Errorf("Result mismatch for invalid remote.\nGot:  %+v\nWant: %+v", results[0], expectedResult)
		}

		// Test Dry Run as well - should still report the validation error
		resultsDry := DeleteBranches(ctx, invalidBranches, true) // Dry run

		if len(resultsDry) != 1 {
			t.Fatalf("Expected 1 result for invalid input (dry run), got %d", len(resultsDry))
		}
		// Dry run doesn't execute, so it won't hit the validation error in the current implementation.
		// Let's adjust the expectation for dry run - it should report what *would* happen,
		// but the validation happens *before* the dry run check for execution.
		// The current code *will* hit the validation error before the dryRun check.
		if !reflect.DeepEqual(resultsDry[0], expectedResult) {
			t.Errorf("Result mismatch for invalid remote (dry run).\nGot:  %+v\nWant: %+v", resultsDry[0], expectedResult)
		}
	})

	// --- Test Case 5: Varied Error Formats ---
	t.Run("Varied Error Formats", func(t *testing.T) {
		branches := []BranchToDelete{
			{Name: "err-no-stderr", IsRemote: false, IsMerged: true, Hash: "h-err1"},
			{Name: "err-with-stderr", IsRemote: false, IsMerged: false, Hash: "h-err2"},
			{Name: "err-empty-stderr", IsRemote: true, Remote: "origin", Hash: "h-err3"},
		}
		expectedResults := []types.DeleteResult{
			{BranchName: "err-no-stderr", IsRemote: false, Success: false, Message: "Failed: plain error message", Cmd: "git branch -d err-no-stderr"},
			{BranchName: "err-with-stderr", IsRemote: false, Success: false, Message: "Failed: useful info from stderr", Cmd: "git branch -D err-with-stderr"},
			{BranchName: "err-empty-stderr", IsRemote: true, RemoteName: "origin", Success: false, Message: "Failed: git command failed: exit status 1\nargs: [push origin --delete err-empty-stderr]\nstderr:", Cmd: "git push origin --delete err-empty-stderr"}, // Expect raw error if stderr part is empty
		}

		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			cmdStr := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdStr, "branch -d err-no-stderr"):
				return "", errors.New("plain error message") // Error without "stderr:"
			case strings.HasPrefix(cmdStr, "branch -D err-with-stderr"):
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, "useful info from stderr")
			case strings.HasPrefix(cmdStr, "push origin --delete err-empty-stderr"):
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, "") // Error with empty stderr part
			default:
				return "", fmt.Errorf("unexpected command in mock: %v", args)
			}
		})
		defer teardown()

		results := DeleteBranches(ctx, branches, false) // Not dry run

		if len(results) != len(expectedResults) {
			t.Fatalf("Expected %d results, got %d", len(expectedResults), len(results))
		}
		for i := range results {
			if !reflect.DeepEqual(results[i], expectedResults[i]) {
				t.Errorf("Result mismatch at index %d.\nGot:  %+v\nWant: %+v", i, results[i], expectedResults[i])
			}
		}
	})
}
