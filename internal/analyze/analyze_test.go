package analyze

import (
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/config"
	"github.com/bral/git-sweep-go/internal/types"
)

func TestAnalyzeBranches(t *testing.T) {
	now := time.Now()
	ninetyDaysAgo := now.AddDate(0, 0, -91) // Slightly more than 90 days
	sixtyDaysAgo := now.AddDate(0, 0, -60)

	testCases := []struct {
		name            string
		branches        []types.BranchInfo
		mergedStatus    map[string]bool
		cfg             config.Config
		currentBranch   string // Added current branch field
		expectedCounts  map[types.BranchCategory]int
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
		// TODO: Add more test cases: different age thresholds, no protected, different main branch name, empty input etc.
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

			// Call AnalyzeBranches with the current branch name
			analyzed := AnalyzeBranches(tc.branches, tc.mergedStatus, tc.cfg, tc.currentBranch)

			if len(analyzed) != len(tc.branches) {
				t.Errorf("Expected %d analyzed branches, got %d", len(tc.branches), len(analyzed))
			}

			counts := make(map[types.BranchCategory]int)
			for _, b := range analyzed {
				counts[b.Category]++
			}

			for category, expectedCount := range tc.expectedCounts {
				if counts[category] != expectedCount {
					t.Errorf("Expected %d branches in category %s, got %d", expectedCount, category, counts[category])
				}
			}
			for category, count := range counts {
				if _, expected := tc.expectedCounts[category]; !expected && count > 0 {
					t.Errorf("Got %d branches in unexpected category %s", count, category)
				}
			}
		})
	}
}
