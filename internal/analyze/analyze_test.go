package analyze

import (
	"context" // Added for mocking
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/config"
	"github.com/bral/git-sweep-go/internal/gitcmd" // Added for mocking
	"github.com/bral/git-sweep-go/internal/types"
)

// Helper to setup mock for AreChangesIncluded
func setupAreChangesIncludedMock(
	_ *testing.T, mockFunc func(ctx context.Context, upstream, head string) (bool, error),
) func() {
	originalFunc := gitcmd.AreChangesIncluded
	gitcmd.AreChangesIncluded = mockFunc
	return func() {
		gitcmd.AreChangesIncluded = originalFunc
	}
}

func TestAnalyzeBranches(t *testing.T) {
	now := time.Now()
	ninetyDaysAgo := now.AddDate(0, 0, -91) // Slightly more than 90 days
	sixtyDaysAgo := now.AddDate(0, 0, -60)

	testCases := []struct {
		name           string
		branches       []types.BranchInfo
		mergedStatus   map[string]bool
		cfg            config.Config
		currentBranch  string // Added current branch field
		expectedCounts map[types.BranchCategory]int
	}{
		{
			name: "Basic Scenario - current branch is main",
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "feature/new", LastCommitDate: sixtyDaysAgo, CommitHash: "featNewHash"},
				{Name: "feature/old-merged", LastCommitDate: ninetyDaysAgo, CommitHash: "featOldMergedHash"},
				{Name: "hotfix/done", LastCommitDate: sixtyDaysAgo, CommitHash: "hotfixHash"},
				{Name: "dev/stale", LastCommitDate: ninetyDaysAgo, CommitHash: "staleHash"},
				{Name: "develop", LastCommitDate: sixtyDaysAgo, CommitHash: "developHash"}, // Protected branch
			},
			mergedStatus: map[string]bool{
				"feature/old-merged": true,
				"hotfix/done":        true,
				"main":               true,
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{"develop"},
				ProtectedBranchMap: map[string]bool{"develop": true},
			},
			currentBranch: "main", // Current branch is main
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   2, // main (implicit + current), develop
				types.CategoryActive:      1, // feature/new
				types.CategoryMergedOld:   2, // feature/old-merged, hotfix/done
				types.CategoryUnmergedOld: 1, // dev/stale
			},
		},
		{
			name: "Current branch is feature/new",
			// Same branches as above
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "feature/new", LastCommitDate: sixtyDaysAgo, CommitHash: "featNewHash"},
				{Name: "feature/old-merged", LastCommitDate: ninetyDaysAgo, CommitHash: "featOldMergedHash"},
				{Name: "hotfix/done", LastCommitDate: sixtyDaysAgo, CommitHash: "hotfixHash"},
				{Name: "dev/stale", LastCommitDate: ninetyDaysAgo, CommitHash: "staleHash"},
				{Name: "develop", LastCommitDate: sixtyDaysAgo, CommitHash: "developHash"},
			},
			mergedStatus: map[string]bool{
				"feature/old-merged": true,
				"hotfix/done":        true,
				"main":               true,
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{"develop"},
				ProtectedBranchMap: map[string]bool{"develop": true},
			},
			currentBranch: "feature/new", // Current branch is feature/new
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   3, // main (implicit), develop, feature/new (current)
				types.CategoryActive:      0, // feature/new is now protected
				types.CategoryMergedOld:   2, // feature/old-merged, hotfix/done
				types.CategoryUnmergedOld: 1, // dev/stale
			},
		},
		{
			name: "Current branch is dev/stale (unmerged old)",
			// Same branches as above
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "feature/new", LastCommitDate: sixtyDaysAgo, CommitHash: "featNewHash"},
				{Name: "feature/old-merged", LastCommitDate: ninetyDaysAgo, CommitHash: "featOldMergedHash"},
				{Name: "hotfix/done", LastCommitDate: sixtyDaysAgo, CommitHash: "hotfixHash"},
				{Name: "dev/stale", LastCommitDate: ninetyDaysAgo, CommitHash: "staleHash"},
				{Name: "develop", LastCommitDate: sixtyDaysAgo, CommitHash: "developHash"},
			},
			mergedStatus: map[string]bool{
				"feature/old-merged": true,
				"hotfix/done":        true,
				"main":               true,
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{"develop"},
				ProtectedBranchMap: map[string]bool{"develop": true},
			},
			currentBranch: "dev/stale", // Current branch is dev/stale
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   3, // main (implicit), develop, dev/stale (current)
				types.CategoryActive:      1, // feature/new
				types.CategoryMergedOld:   2, // feature/old-merged, hotfix/done
				types.CategoryUnmergedOld: 0, // dev/stale is now protected
			},
		},
		{
			name: "Different Age Threshold (30 days)",
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "feature/new", LastCommitDate: sixtyDaysAgo, CommitHash: "featNewHash"}, // Now old
				{Name: "feature/old-merged", LastCommitDate: ninetyDaysAgo, CommitHash: "featOldMergedHash"},
				{Name: "dev/stale", LastCommitDate: ninetyDaysAgo, CommitHash: "staleHash"}, // Still old
			},
			mergedStatus: map[string]bool{
				"feature/old-merged": true,
				"main":               true,
			},
			cfg: config.Config{
				AgeDays:            30, // Lower threshold
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{}, // No protected branches
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "main",
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   1, // main (implicit + current)
				types.CategoryActive:      0, // feature/new is now old
				types.CategoryMergedOld:   1, // feature/old-merged
				types.CategoryUnmergedOld: 2, // feature/new, dev/stale
			},
		},
		{
			name: "No Protected Branches Configured",
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "develop", LastCommitDate: sixtyDaysAgo, CommitHash: "developHash"}, // No longer protected by config
			},
			mergedStatus: map[string]bool{"main": true},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{}, // Empty list
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "main",
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   1, // main (implicit + current)
				types.CategoryActive:      1, // develop is now active
				types.CategoryMergedOld:   0,
				types.CategoryUnmergedOld: 0,
			},
		},
		{
			name: "Different Primary Main Branch (master)",
			branches: []types.BranchInfo{
				{Name: "master", LastCommitDate: now, CommitHash: "masterHash"},
				{Name: "feature/merged-to-master", LastCommitDate: sixtyDaysAgo, CommitHash: "featMasterHash"},
				{Name: "feature/not-merged", LastCommitDate: sixtyDaysAgo, CommitHash: "featNotMergedHash"},
			},
			mergedStatus: map[string]bool{ // Merged into master
				"master":                   true,
				"feature/merged-to-master": true,
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "master", // Using master
				ProtectedBranches:  []string{},
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "master",
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   1, // master (implicit + current)
				types.CategoryActive:      1, // feature/not-merged
				types.CategoryMergedOld:   1, // feature/merged-to-master
				types.CategoryUnmergedOld: 0,
			},
		},
		{
			name:           "Empty Input Branches",
			branches:       []types.BranchInfo{}, // Empty slice
			mergedStatus:   map[string]bool{},
			cfg:            config.DefaultConfig(),         // Use defaults
			currentBranch:  "main",                         // Doesn't really matter here
			expectedCounts: map[types.BranchCategory]int{}, // Expect zero counts
		},
		{
			name: "Branch Exactly on Age Threshold",
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				// This branch's age is exactly 90 days (using -90), so it should NOT be considered old
				{Name: "feature/on-threshold", LastCommitDate: now.AddDate(0, 0, -90), CommitHash: "thresholdHash"},
			},
			mergedStatus: map[string]bool{"main": true},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{},
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "main",
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   1, // main
				types.CategoryActive:      1, // feature/on-threshold is active, not old
				types.CategoryMergedOld:   0,
				types.CategoryUnmergedOld: 0,
			},
		},
		{
			name: "Squash Merged Branch Detected via Cherry Check",
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{Name: "feature/squashed", LastCommitDate: sixtyDaysAgo, CommitHash: "squashHash"}, // Not in mergedStatus initially
				{Name: "feature/active", LastCommitDate: sixtyDaysAgo, CommitHash: "activeHash"},
			},
			mergedStatus: map[string]bool{
				"main": true, // Only main is merged by ancestry
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{},
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "main",
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   1, // main
				types.CategoryActive:      1, // feature/active
				types.CategoryMergedOld:   1, // feature/squashed (detected by mock)
				types.CategoryUnmergedOld: 0,
			},
			// This test case requires mocking gitcmd.AreChangesIncluded
		},
		{
			name: "Cherry Check Fails", // Test when AreChangesIncluded returns an error
			branches: []types.BranchInfo{
				{Name: "main", LastCommitDate: now, CommitHash: "mainHash"},
				{ // Not merged, cherry check will fail
					Name:           "feature/cherry-fails",
					LastCommitDate: sixtyDaysAgo,
					CommitHash:     "cherryFailHash",
				},
			},
			mergedStatus: map[string]bool{
				"main": true,
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{},
				ProtectedBranchMap: map[string]bool{},
			},
			currentBranch: "main",
			expectedCounts: map[types.BranchCategory]int{
				// Expect error, so counts don't matter as much, but should reflect state *before* error
				types.CategoryProtected:   1, // main
				types.CategoryActive:      1, // feature/cherry-fails (treated as active before error)
				types.CategoryMergedOld:   0,
				types.CategoryUnmergedOld: 0,
			},
			// This test case requires mocking gitcmd.AreChangesIncluded to return an error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure ProtectedBranchMap is populated
			if tc.cfg.ProtectedBranchMap == nil {
				tc.cfg.ProtectedBranchMap = make(map[string]bool)
				for _, pb := range tc.cfg.ProtectedBranches {
					tc.cfg.ProtectedBranchMap[pb] = true
				}
			}

			// --- Mocking Setup ---
			// Default mock: Assume cherry check returns false (not included) and no error
			// This prevents real git commands from running in most test cases.
			mockFunc := func(_ context.Context, _ /*upstream*/, _ /*head*/ string) (bool, error) {
				// Default mock doesn't need upstream or head, only specific overrides do.
				return false, nil
			}
			mockErr := fmt.Errorf("simulated cherry check error") // Define error for reuse

			// Override mock for specific test cases
			switch tc.name {
			case "Squash Merged Branch Detected via Cherry Check":
				mockFunc = func(_ context.Context, upstream, head string) (bool, error) { // Renamed ctx to _
					if upstream == tc.cfg.PrimaryMainBranch && head == "feature/squashed" {
						return true, nil // Simulate cherry check passing for this branch
					}
					return false, nil // Default for others in this specific test
				}
			case "Cherry Check Fails":
				mockFunc = func(_ context.Context, upstream, head string) (bool, error) { // Renamed ctx to _
					if upstream == tc.cfg.PrimaryMainBranch && head == "feature/cherry-fails" {
						return false, mockErr // Simulate cherry check failing
					}
					return false, nil // Default for others in this specific test
				}
			}

			// Apply the chosen mock function and ensure teardown
			teardown := setupAreChangesIncludedMock(t, mockFunc)
			defer teardown()
			// --- End Mocking Setup ---

			// Call Branches with the current branch name (renamed from AnalyzeBranches)
			// Add context.Background() and handle the error return value
			analyzed, err := Branches(context.Background(), tc.branches, tc.mergedStatus, tc.cfg, tc.currentBranch)

			// --- Error Handling based on test case ---
			if tc.name == "Cherry Check Fails" {
				if err == nil {
					t.Fatalf("Expected an error from AnalyzeBranches due to cherry check failure, but got nil")
				}
				// Check if the error wraps the specific mocked error
				if !errors.Is(err, mockErr) {
					t.Errorf("Expected error to wrap '%v', but got: %v", mockErr, err)
				}
				// Since we expected an error and got one, we can stop further checks for this case.
				// We might want to verify the state *before* the error, but the current expectedCounts
				// already reflect that.
				return
			}
			// For other tests, we don't expect an error from AnalyzeBranches itself
			if err != nil {
				// If an error occurred unexpectedly (e.g., mock setup issue), fail the test.
				t.Fatalf("AnalyzeBranches returned an unexpected error for test case '%s': %v", tc.name, err)
			}
			// --- End Error Handling ---

			if len(analyzed) != len(tc.branches) {
				t.Errorf("Expected %d analyzed branches, got %d", len(tc.branches), len(analyzed))
			}

			counts := make(map[types.BranchCategory]int)
			foundCurrent := ""
			for _, b := range analyzed {
				counts[b.Category]++
				if b.IsCurrent {
					if foundCurrent != "" {
						t.Errorf("Found multiple current branches: %s and %s", foundCurrent, b.Name)
					}
					foundCurrent = b.Name
				}
			}

			// Check category counts
			for category, expectedCount := range tc.expectedCounts {
				if counts[category] != expectedCount {
					t.Errorf("Expected %d branches in category %s, got %d", expectedCount, category, counts[category])
				}
			}
			// Check if any unexpected categories appeared
			for category, count := range counts {
				if _, expected := tc.expectedCounts[category]; !expected && count > 0 {
					t.Errorf("Got %d branches in unexpected category %s", count, category)
				}
			}

			// Check if the correct branch was marked as current
			// Special case for empty branches - nothing should be marked as current
			if len(tc.branches) == 0 {
				if foundCurrent != "" {
					t.Errorf("Expected no branch to be marked as current when branches list is empty, but %q was marked", foundCurrent)
				}
			} else if foundCurrent != tc.currentBranch {
				t.Errorf("Expected current branch to be %q, but %q was marked", tc.currentBranch, foundCurrent)
			}
		})
	}
}
