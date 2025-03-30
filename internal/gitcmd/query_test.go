package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync" // Added for the new setup
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/types"
	"github.com/google/go-cmp/cmp" // Added for comparing slices
)

const (
	cmdBranch              = "branch"
	cmdRevParse            = "rev-parse"
	cmdCherry              = "cherry"
	flagMerged             = "--merged"
	flagIsInsideWorkTree   = "--is-inside-work-tree"
	flagShowCurrent        = "--show-current"
	flagAbbrevRef          = "--abbrev-ref"
	flagCherryVerbose      = "-v"
	detachedHeadTestStr    = "HEAD" // Separate from internal constant for test isolation
	simulatedGitError      = "simulated git error"
	simulatedBranchError   = "simulated branch error"
	simulatedRevParseError = "simulated rev-parse error"
	simulatedCherryError   = "simulated cherry error"
)

// --- Refactored Mock Setup ---

// commandExpectation defines an expected git command call and its result.
type commandExpectation struct {
	args   []string // Expected arguments
	output string   // Output to return
	err    error    // Error to return
}

// setupExpectations sets the package Runner to a mock that verifies calls against a sequence of expectations.
// It returns a teardown function to restore the original runner.
func setupExpectations(t *testing.T, expectations []commandExpectation) func() {
	t.Helper() // Mark this as a test helper

	originalRunner := Runner
	currentExpectationIndex := 0
	var mu sync.Mutex // Protect access to the index

	mockFunc := func(_ context.Context, args ...string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if currentExpectationIndex >= len(expectations) {
			t.Fatalf("Unexpected git command call: %v. No more expectations.", args)
			return "", errors.New("unexpected call") // Should not be reached
		}

		expected := expectations[currentExpectationIndex]

		// Use go-cmp for robust slice comparison
		if diff := cmp.Diff(expected.args, args); diff != "" {
			t.Fatalf("Unexpected git command arguments (-want +got):\n%s", diff)
			return "", errors.New("unexpected arguments") // Should not be reached
		}

		currentExpectationIndex++
		return expected.output, expected.err
	}

	Runner = mockFunc

	// Return a teardown function
	return func() {
		mu.Lock()
		defer mu.Unlock()
		// Check if all expectations were met
		if currentExpectationIndex < len(expectations) {
			t.Errorf("Not all expected git commands were called. Expected %d more.", len(expectations)-currentExpectationIndex)
			// Log the remaining expectations for easier debugging
			for i := currentExpectationIndex; i < len(expectations); i++ {
				t.Logf(
					"Remaining expectation %d: args=%v, output=%q, err=%v",
					i, expectations[i].args, expectations[i].output, expectations[i].err,
				)
			}
		}
		Runner = originalRunner
	}
}

// --- Original Tests (Modified to use setupExpectations) ---

func TestGetAllLocalBranchInfo(t *testing.T) {
	ctx := context.Background()

	// Sample output using null separators and newline records
	sampleOutput := "main\x00origin/main\x00origin\x002025-03-27 20:00:00 -0400\x00hash1\n" +
		"feature/a\x00\x00\x002025-03-26 10:00:00 -0400\x00hash2\n" + // No upstream/remote
		"hotfix/b\x00upstream/hotfix/b\x00upstream\x002025-03-25 15:30:00 -0400\x00hash3"
		// No trailing newline needed

	expectedDate1, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-27 20:00:00 -0400")
	expectedDate2, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-26 10:00:00 -0400")
	expectedDate3, _ := time.Parse("2006-01-02 15:04:05 -0700", "2025-03-25 15:30:00 -0400")

	expectedBranches := []types.BranchInfo{
		{Name: "main", Upstream: "origin/main", Remote: "origin", LastCommitDate: expectedDate1, CommitHash: "hash1"},
		{Name: "feature/a", Upstream: "", Remote: "", LastCommitDate: expectedDate2, CommitHash: "hash2"},
		{
			Name: "hotfix/b", Upstream: "upstream/hotfix/b", Remote: "upstream",
			LastCommitDate: expectedDate3, CommitHash: "hash3",
		},
	}

	// --- Test Case 1: Successful parsing ---
	t.Run("Successful Parsing", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args: []string{
					"for-each-ref",
					"refs/heads/",
					fmt.Sprintf("--format=%s", branchInfoFormat),
				},
				output: sampleOutput,
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		branches, err := GetAllLocalBranchInfo(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(branches) != len(expectedBranches) {
			t.Fatalf("Expected %d branches, got %d", len(expectedBranches), len(branches))
		}

		for i, expected := range expectedBranches {
			actual := branches[i]
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("Branch %d mismatch:\nExpected: %+v\nActual: %+v", i, expected, actual)
			}
		}
	})

	// --- Test Case 2: Empty output (no branches) ---
	t.Run("Empty Output", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args: []string{
					"for-each-ref",
					"refs/heads/",
					fmt.Sprintf("--format=%s", branchInfoFormat),
				},
				output: "",
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
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
		expectedErr := errors.New(simulatedGitError)
		expectations := []commandExpectation{
			{
				args: []string{
					"for-each-ref",
					"refs/heads/",
					fmt.Sprintf("--format=%s", branchInfoFormat),
				},
				output: "",
				err:    expectedErr,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		_, err := GetAllLocalBranchInfo(ctx)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		// Check if the error message contains the original simulated error
		if !strings.Contains(err.Error(), simulatedGitError) {
			t.Errorf("Expected error to contain '%s', got: %v", simulatedGitError, err)
		}
	})

	// --- Test Case 4: Malformed record ---
	t.Run("Malformed Record", func(t *testing.T) {
		malformedOutput := "main\x00origin/main\x00origin\x002025-03-27 20:00:00 -0400\x00hash1\n" +
			"feature/a\x00malformed_no_separators\n" + // Malformed line
			"hotfix/b\x00upstream/hotfix/b\x00upstream\x002025-03-25 15:30:00 -0400\x00hash3"

		// Expect only the valid branches
		expectedValid := []types.BranchInfo{expectedBranches[0], expectedBranches[2]}

		expectations := []commandExpectation{
			{
				args: []string{
					"for-each-ref",
					"refs/heads/",
					fmt.Sprintf("--format=%s", branchInfoFormat),
				},
				output: malformedOutput,
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

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
		expectations := []commandExpectation{
			{
				args:   []string{cmdBranch, flagMerged, targetHash},
				output: sampleOutput,
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
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
		expectations := []commandExpectation{
			{
				args:   []string{cmdBranch, flagMerged, targetHash},
				output: "",
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
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
		expectedErr := errors.New(simulatedBranchError)
		expectations := []commandExpectation{
			{
				args:   []string{cmdBranch, flagMerged, targetHash},
				output: "",
				err:    expectedErr,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		_, err := GetMergedBranches(ctx, targetHash)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		if !strings.Contains(err.Error(), simulatedBranchError) {
			t.Errorf("Expected error to contain '%s', got: %v", simulatedBranchError, err)
		}
	})
}

func TestGetMainBranchHash(t *testing.T) {
	ctx := context.Background()
	branchName := "main"
	expectedHash := "abcdef123456"

	// --- Test Case 1: Successful retrieval ---
	t.Run("Successful Retrieval", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, branchName},
				output: expectedHash,
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		hash, err := GetMainBranchHash(ctx, branchName)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if hash != expectedHash {
			t.Errorf("Expected hash %q, got %q", expectedHash, hash)
		}
	})

	// --- Test Case 2: Empty branch name ---
	t.Run("Empty Branch Name", func(t *testing.T) {
		// No setup needed as it should error before calling Runner
		_, err := GetMainBranchHash(ctx, "")
		if err == nil {
			t.Fatal("Expected an error for empty branch name, got nil")
		}
		if !strings.Contains(err.Error(), "branch name cannot be empty") {
			t.Errorf("Expected error message about empty branch name, got: %v", err)
		}
	})

	// --- Test Case 3: Git command error ---
	t.Run("Git Command Error", func(t *testing.T) {
		expectedErr := errors.New(simulatedRevParseError)
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, branchName},
				output: "",
				err:    expectedErr,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		_, err := GetMainBranchHash(ctx, branchName)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		// Check if the error message contains the branch name for context
		if !strings.Contains(err.Error(), branchName) {
			t.Errorf("Expected error message to contain branch name %q, got: %v", branchName, err)
		}
		if !strings.Contains(err.Error(), simulatedRevParseError) {
			t.Errorf("Expected error message to contain original error '%s', got: %v", simulatedRevParseError, err)
		}
	})

	// --- Test Case 4: Empty hash returned (e.g., branch doesn't exist) ---
	t.Run("Empty Hash Returned", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, branchName},
				output: "", // Simulate empty output
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		_, err := GetMainBranchHash(ctx, branchName)
		if err == nil {
			t.Fatal("Expected an error for empty hash, got nil")
		}
		if !strings.Contains(err.Error(), "no hash returned") {
			t.Errorf("Expected error message about empty hash, got: %v", err)
		}
	})
}

func TestIsInGitRepo(t *testing.T) {
	ctx := context.Background()

	// --- Test Case 1: Inside Git Repo ---
	t.Run("Inside Git Repo", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, flagIsInsideWorkTree},
				output: "true",
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		isInside, err := IsInGitRepo(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !isInside {
			t.Error("Expected IsInGitRepo to return true, got false")
		}
	})

	// --- Test Case 2: Outside Git Repo (Command Fails) ---
	t.Run("Outside Git Repo", func(t *testing.T) {
		// Simulate the command failing, which IsInGitRepo interprets as false
		gitError := errors.New("fatal: not a git repository")
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, flagIsInsideWorkTree},
				output: "",
				err:    gitError,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		isInside, err := IsInGitRepo(ctx)
		if err != nil {
			// The function IsInGitRepo should swallow the error in this case
			t.Fatalf("Expected no error when command fails, got %v", err)
		}
		if isInside {
			t.Error("Expected IsInGitRepo to return false, got true")
		}
	})

	// --- Test Case 3: Command Succeeds but returns "false" (unlikely but test) ---
	t.Run("Command Returns False", func(t *testing.T) {
		expectations := []commandExpectation{
			{
				args:   []string{cmdRevParse, flagIsInsideWorkTree},
				output: "false",
				err:    nil,
			},
		}
		teardown := setupExpectations(t, expectations)
		defer teardown()

		isInside, err := IsInGitRepo(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if isInside {
			t.Error("Expected IsInGitRepo to return false when output is 'false', got true")
		}
	})
}

// --- TestGetCurrentBranchName (Refactored) ---
func TestGetCurrentBranchName(t *testing.T) {
	ctx := context.Background()
	expectedBranch := "current-feature"
	showCurrentArgs := []string{cmdBranch, flagShowCurrent}
	revParseArgs := []string{cmdRevParse, flagAbbrevRef, detachedHeadTestStr}
	showCurrentFailError := errors.New(
		"git command failed: exit status 1\nargs: [branch --show-current]\nstderr: unknown switch `show-current'",
	)

	testCases := []struct {
		name           string
		expectations   []commandExpectation
		expectedBranch string
		expectedError  bool
		errorContains  string // Substring to check in error message
	}{
		{
			name: "Success with show-current",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: expectedBranch, err: nil},
			},
			expectedBranch: expectedBranch,
			expectedError:  false,
		},
		{
			name: "Fallback Success with rev-parse",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: "", err: showCurrentFailError}, // Simulate --show-current failing
				{args: revParseArgs, output: expectedBranch, err: nil},         // Fallback succeeds
			},
			expectedBranch: expectedBranch,
			expectedError:  false,
		},
		{
			name: "Detached HEAD with show-current",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: "", err: nil}, // Simulate empty output for detached HEAD
			},
			expectedBranch: "", // Expect empty string for detached HEAD
			expectedError:  false,
		},
		{
			name: "Detached HEAD with rev-parse fallback",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: "", err: showCurrentFailError}, // Simulate --show-current failing
				{args: revParseArgs, output: detachedHeadTestStr, err: nil},    // Fallback returns "HEAD"
			},
			expectedBranch: "", // Expect empty string for detached HEAD
			expectedError:  false,
		},
		{
			name: "Error in show-current (not fallback related)",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: "", err: errors.New("some other git error")},
			},
			expectedBranch: "",
			expectedError:  true,
			errorContains:  "some other git error",
		},
		{
			name: "Error in rev-parse fallback",
			expectations: []commandExpectation{
				{args: showCurrentArgs, output: "", err: showCurrentFailError},            // Simulate --show-current failing
				{args: revParseArgs, output: "", err: errors.New(simulatedRevParseError)}, // Fallback also fails
			},
			expectedBranch: "",
			expectedError:  true,
			errorContains:  simulatedRevParseError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			teardown := setupExpectations(t, tc.expectations)
			defer teardown()

			branch, err := GetCurrentBranchName(ctx)

			if tc.expectedError {
				if err == nil {
					t.Fatal("Expected an error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error message to contain %q, got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				if branch != tc.expectedBranch {
					t.Errorf("Expected branch %q, got %q", tc.expectedBranch, branch)
				}
			}
		})
	}
}

func TestAreChangesIncluded(t *testing.T) {
	ctx := context.Background()
	upstreamBranch := "main"
	headBranch := "feature"

	cherryArgs := []string{cmdCherry, flagCherryVerbose, upstreamBranch, headBranch}
	testCases := []struct {
		name           string
		upstream       string
		head           string
		expectations   []commandExpectation
		expectedResult bool
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "All Included - Empty Output",
			upstream:       upstreamBranch,
			head:           headBranch,
			expectations:   []commandExpectation{{args: cherryArgs, output: "", err: nil}},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "All Included - Minus Lines Only",
			upstream:       upstreamBranch,
			head:           headBranch,
			expectations:   []commandExpectation{{args: cherryArgs, output: "- commit1\n- commit2", err: nil}},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "Not Included - Plus Line Present",
			upstream:       upstreamBranch,
			head:           headBranch,
			expectations:   []commandExpectation{{args: cherryArgs, output: "- commit1\n+ commit2\n- commit3", err: nil}},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "Not Included - Only Plus Line",
			upstream:       upstreamBranch,
			head:           headBranch,
			expectations:   []commandExpectation{{args: cherryArgs, output: "+ commit1", err: nil}},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "Git Command Error",
			upstream:       upstreamBranch,
			head:           headBranch,
			expectations:   []commandExpectation{{args: cherryArgs, output: "", err: errors.New(simulatedCherryError)}},
			expectedResult: false,
			expectedError:  true,
			errorContains:  simulatedCherryError,
		},
		{
			name:           "Empty Upstream Branch Name",
			upstream:       "", // Empty upstream
			head:           headBranch,
			expectations:   nil, // No git call expected
			expectedResult: false,
			expectedError:  true,
			errorContains:  "upstream and head branch names cannot be empty for cherry check",
		},
		{
			name:           "Empty Head Branch Name",
			upstream:       upstreamBranch,
			head:           "",  // Empty head
			expectations:   nil, // No git call expected
			expectedResult: false,
			expectedError:  true,
			errorContains:  "upstream and head branch names cannot be empty for cherry check",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Only setup mock if we expect git to be called
			if tc.expectations != nil {
				teardown := setupExpectations(t, tc.expectations)
				defer teardown()
			}

			result, err := AreChangesIncluded(ctx, tc.upstream, tc.head)

			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error message to contain %q, got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tc.expectedResult {
					t.Errorf("Expected result %v, but got %v", tc.expectedResult, result)
				}
			}
		})
	}
}
