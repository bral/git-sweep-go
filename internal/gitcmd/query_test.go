package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/types"
)

// mockRunner is a helper for tests to mock git command execution.
type mockRunner struct {
	mock func(ctx context.Context, args ...string) (string, error)
}

func (m *mockRunner) run(ctx context.Context, args ...string) (string, error) {
	if m.mock != nil {
		return m.mock(ctx, args...)
	}
	return "", errors.New("mockRunner not implemented")
}

// setup sets the package Runner to the mock and returns a teardown function.
func setup(t *testing.T, mockFunc func(ctx context.Context, args ...string) (string, error)) func() {
	originalRunner := Runner
	mock := &mockRunner{mock: mockFunc}
	Runner = mock.run
	// Return a teardown function to restore the original runner
	return func() {
		Runner = originalRunner
	}
}

func TestGetAllLocalBranchInfo(t *testing.T) {
	ctx := context.Background()

	// Sample output using null separators and newline records
	sampleOutput := "main\x00origin/main\x00origin\x002025-03-27 20:00:00 -0400\x00hash1\n" +
		"feature/a\x00\x00\x002025-03-26 10:00:00 -0400\x00hash2\n" + // No upstream/remote
		"hotfix/b\x00upstream/hotfix/b\x00upstream\x002025-03-25 15:30:00 -0400\x00hash3" // No trailing newline needed

	expectedDate1, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-27 20:00:00 -0400")
	expectedDate2, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-26 10:00:00 -0400")
	expectedDate3, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-25 15:30:00 -0400")

	expectedBranches := []types.BranchInfo{
		{Name: "main", Upstream: "origin/main", Remote: "origin", LastCommitDate: expectedDate1, CommitHash: "hash1"},
		{Name: "feature/a", Upstream: "", Remote: "", LastCommitDate: expectedDate2, CommitHash: "hash2"},
		{Name: "hotfix/b", Upstream: "upstream/hotfix/b", Remote: "upstream", LastCommitDate: expectedDate3, CommitHash: "hash3"},
	}

	// --- Test Case 1: Successful parsing ---
	t.Run("Successful Parsing", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			// Check if the command is the expected one
			if len(args) > 0 && args[0] == "for-each-ref" {
				return sampleOutput, nil
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		})
		defer teardown()

		branches, err := GetAllLocalBranchInfo(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !reflect.DeepEqual(branches, expectedBranches) {
			t.Errorf("Branch list mismatch.\nGot:  %+v\nWant: %+v", branches, expectedBranches)
		}
	})

	// --- Test Case 2: Empty output (no branches) ---
	t.Run("Empty Output", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				return "", nil // Simulate empty output
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		})
		defer teardown()

		branches, err := GetAllLocalBranchInfo(ctx)
		if err != nil {
			t.Fatalf("Expected no error for empty output, got %v", err)
		}
		if len(branches) != 0 {
			t.Errorf("Expected empty branch slice, got %d branches", len(branches))
		}
	})

	// --- Test Case 3: Git command error ---
	t.Run("Git Command Error", func(t *testing.T) {
		expectedErr := errors.New("simulated git error")
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			return "", expectedErr // Simulate error from RunGitCommand
		})
		defer teardown()

		_, err := GetAllLocalBranchInfo(ctx)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		// Check if the error wraps the original simulated error (optional but good practice)
		// Note: The actual error will be wrapped by RunGitCommand's formatting.
		// if !errors.Is(err, expectedErr) {
		// 	t.Errorf("Expected error to wrap '%v', but it didn't", expectedErr)
		// }
	})

	// --- Test Case 4: Malformed record ---
	t.Run("Malformed Record", func(t *testing.T) {
		malformedOutput := "main\x00origin/main\x00origin\x002025-03-27 20:00:00 -0400\x00hash1\n" +
			"feature/a\x00malformed_no_separators\n" + // Malformed line
			"hotfix/b\x00upstream/hotfix/b\x00upstream\x002025-03-25 15:30:00 -0400\x00hash3"

		// Expect only the valid branches
		expectedValid := []types.BranchInfo{expectedBranches[0], expectedBranches[2]}

		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				return malformedOutput, nil
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		})
		defer teardown()

		// Suppress warning output during test (optional)
		// originalStdout := os.Stdout
		// r, w, _ := os.Pipe()
		// os.Stdout = w
		// defer func() {
		// 	w.Close()
		// 	os.Stdout = originalStdout
		// }()

		branches, err := GetAllLocalBranchInfo(ctx)
		if err != nil {
			t.Fatalf("Expected no error despite malformed record, got %v", err)
		}
		if !reflect.DeepEqual(branches, expectedValid) {
			t.Errorf("Branch list mismatch after malformed record.\nGot:  %+v\nWant: %+v", branches, expectedValid)
		}
	})

}

func TestGetMergedBranches(t *testing.T) {
	ctx := context.Background()
	targetHash := "targetCommitHash"

	sampleOutput := "  branch1\n* main\n  branch3\n" // Includes current branch marker

	expectedMap := map[string]bool{
		"branch1": true,
		"main":    true,
		"branch3": true,
	}

	// --- Test Case 1: Successful parsing ---
	t.Run("Successful Parsing", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			if len(args) == 3 && args[0] == "branch" && args[1] == "--merged" && args[2] == targetHash {
				return sampleOutput, nil
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		})
		defer teardown()

		mergedMap, err := GetMergedBranches(ctx, targetHash)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !reflect.DeepEqual(mergedMap, expectedMap) {
			t.Errorf("Merged map mismatch.\nGot:  %v\nWant: %v", mergedMap, expectedMap)
		}
	})

	// --- Test Case 2: Empty output ---
	t.Run("Empty Output", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			if len(args) == 3 && args[0] == "branch" && args[1] == "--merged" {
				return "", nil
			}
			return "", fmt.Errorf("unexpected command: %v", args)
		})
		defer teardown()

		mergedMap, err := GetMergedBranches(ctx, targetHash)
		if err != nil {
			t.Fatalf("Expected no error for empty output, got %v", err)
		}
		if len(mergedMap) != 0 {
			t.Errorf("Expected empty map, got %d entries", len(mergedMap))
		}
	})

	// --- Test Case 3: Git command error ---
	t.Run("Git Command Error", func(t *testing.T) {
		expectedErr := errors.New("simulated git branch error")
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			return "", expectedErr
		})
		defer teardown()

		_, err := GetMergedBranches(ctx, targetHash)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
	})
}

// TODO: Add tests for GetMainBranchHash, IsInGitRepo
