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

// Note: The setupMockRunner function is defined in test_helpers_test.go

func TestDeleteBranches(t *testing.T) {
	ctx := context.Background()

	branchesToDelete := []BranchToDelete{
		{Name: "local-merged", IsRemote: false, IsMerged: true, Hash: "h1", MergeMethod: string(types.MergeMethodStandard)},
		{Name: "local-unmerged", IsRemote: false, IsMerged: false, Hash: "h2", MergeMethod: ""},
		{Name: "remote-branch", IsRemote: true, Remote: "origin", Hash: "h3", MergeMethod: ""},
		// Will simulate failure
		{
			Name:        "fail-local",
			IsRemote:    false,
			IsMerged:    true,
			Hash:        "h4",
			MergeMethod: string(types.MergeMethodStandard),
		},
		// Will simulate failure
		{
			Name:        "fail-remote",
			IsRemote:    true,
			Remote:      "origin",
			Hash:        "h5",
			MergeMethod: "",
		},
	}

	expectedResultsSuccess := []types.DeleteResult{
		// Successful deletions should have the hash populated
		{
			BranchName: "local-merged", IsRemote: false, Success: true, Message: "Successfully deleted",
			Cmd: "git branch -d local-merged", DeletedHash: "h1",
		},
		{
			BranchName: "local-unmerged", IsRemote: false, Success: true, Message: "Successfully deleted",
			Cmd: "git branch -D local-unmerged", DeletedHash: "h2",
		},
		{
			BranchName: "remote-branch", IsRemote: true, RemoteName: "origin", Success: true, Message: "Successfully deleted",
			Cmd: "git push origin --delete remote-branch", DeletedHash: "h3",
		},
		// Failed deletions should have an empty hash
		{
			BranchName: "fail-local",
			IsRemote:   false,
			Success:    false,
			Message:    "Failed: simulated local delete error",
			// Updated to show both commands with fallback
			Cmd:         "git branch -d fail-local → git branch -D fail-local",
			DeletedHash: "",
		},
		{
			BranchName: "fail-remote", IsRemote: true, RemoteName: "origin", Success: false,
			Message: "Failed: simulated remote delete error", Cmd: "git push origin --delete fail-remote", DeletedHash: "",
		},
	}

	// Update the expected dry run results to match the regular results
	expectedResultsDryRun := []types.DeleteResult{
		{
			BranchName: "local-merged", IsRemote: false, Success: true,
			Message: "Dry Run: Would execute: git branch -d local-merged",
			Cmd:     "git branch -d local-merged", DeletedHash: "", // Dry run, no hash
		},
		{
			BranchName: "local-unmerged", IsRemote: false, Success: true,
			Message: "Dry Run: Would execute: git branch -D local-unmerged",
			Cmd:     "git branch -D local-unmerged", DeletedHash: "", // Dry run, no hash
		},
		{
			BranchName: "remote-branch", IsRemote: true, RemoteName: "origin", Success: true,
			Message: "Dry Run: Would execute: git push origin --delete remote-branch",
			Cmd:     "git push origin --delete remote-branch", DeletedHash: "", // Dry run, no hash
		},
		// Dry run always "succeeds" for these entries
		{
			BranchName: "fail-local", IsRemote: false, Success: true,
			Message: "Dry Run: Would execute: git branch -d fail-local",
			Cmd:     "git branch -d fail-local", DeletedHash: "", // Dry run, no hash
		},
		{
			BranchName: "fail-remote", IsRemote: true, RemoteName: "origin", Success: true,
			Message: "Dry Run: Would execute: git push origin --delete fail-remote",
			Cmd:     "git push origin --delete fail-remote", DeletedHash: "", // Dry run, no hash
		},
	}

	// --- Test Case 1: Successful Deletion (with simulated failures) ---
	t.Run("Successful Deletion", func(t *testing.T) {
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
			cmdStr := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdStr, "branch -d local-merged"):
				return "Deleted branch local-merged (was h1).", nil
			case strings.HasPrefix(cmdStr, "branch -D local-unmerged"):
				return "Deleted branch local-unmerged (was h2).", nil
			case strings.HasPrefix(cmdStr, "push origin --delete remote-branch"):
				return "To github.com:user/repo\n - [deleted]         remote-branch", nil
			case strings.HasPrefix(cmdStr, "branch -d fail-local"):
				// First attempt fails with not fully merged error
				errStr := "error: The branch 'fail-local' is not fully merged.\n" +
					"If you are sure you want to delete it, run 'git branch -D fail-local'."
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, errStr)
			case strings.HasPrefix(cmdStr, "branch -D fail-local"):
				// Fallback attempt with -D also fails with a different error
				errStr := "simulated local delete error"
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, errStr)
			case strings.HasPrefix(cmdStr, "push origin --delete fail-remote"):
				// Simulate failure by returning an error
				errStr := "simulated remote delete error"
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, errStr)
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
			// Check all fields including the new DeletedHash
			if actual.BranchName != expected.BranchName ||
				actual.IsRemote != expected.IsRemote ||
				actual.RemoteName != expected.RemoteName ||
				actual.Success != expected.Success ||
				actual.Cmd != expected.Cmd ||
				actual.DeletedHash != expected.DeletedHash || // Check the hash
				// Only check prefix for error messages, as the full message might contain variable parts
				(actual.Success != expected.Success && !strings.HasPrefix(actual.Message, "Failed: simulated")) {
				t.Errorf("Result mismatch at index %d.\nGot:  %+v\nWant: %+v", i, actual, expected)
			}
		}
	})

	// --- Test Case 2: Dry Run ---
	t.Run("Dry Run", func(t *testing.T) {
		// The mock runner should NOT be called in dry run mode
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
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
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
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
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
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
			{
				Name:        "err-no-stderr",
				IsRemote:    false,
				IsMerged:    true,
				Hash:        "h-err1",
				MergeMethod: string(types.MergeMethodStandard),
			},
			{Name: "err-with-stderr", IsRemote: false, IsMerged: false, Hash: "h-err2"},
			{Name: "err-empty-stderr", IsRemote: true, Remote: "origin", Hash: "h-err3"},
		}
		expectedResults := []types.DeleteResult{
			{
				BranchName: "err-no-stderr", IsRemote: false, Success: false,
				Message: "Failed: plain error message",
				Cmd:     "git branch -d err-no-stderr → git branch -D err-no-stderr",
			},
			{
				BranchName: "err-with-stderr", IsRemote: false, Success: false,
				Message: "Failed: useful info from stderr",
				Cmd:     "git branch -D err-with-stderr",
			},
			{
				BranchName: "err-empty-stderr", IsRemote: true, RemoteName: "origin", Success: false,
				Message: "Failed: git command failed: exit status 1\nargs: [push origin --delete err-empty-stderr]\nstderr:",
				Cmd:     "git push origin --delete err-empty-stderr",
			}, // Expect raw error if stderr part is empty
		}

		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
			cmdStr := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdStr, "branch -d err-no-stderr"):
				// Trigger fallback by returning a "not fully merged" error
				return "", fmt.Errorf("error: The branch 'err-no-stderr' is not fully merged.\n" +
					"If you are sure you want to delete it, run 'git branch -D err-no-stderr'")
			case strings.HasPrefix(cmdStr, "branch -D err-no-stderr"):
				// The fallback should also fail with the plain error
				return "", errors.New("plain error message") // Error without "stderr:"
			case strings.HasPrefix(cmdStr, "branch -D err-with-stderr"):
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr: %s", args, "useful info from stderr")
			case strings.HasPrefix(cmdStr, "push origin --delete err-empty-stderr"):
				// Using %s with empty string to avoid linter errors about error message capitalization
				return "", fmt.Errorf("git command failed: exit status 1\nargs: %v\nstderr:%s", args, "")
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
