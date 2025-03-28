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
		name           string
		branches       []types.BranchInfo
		mergedStatus   map[string]bool
		cfg            config.Config
		expectedCounts map[types.BranchCategory]int
	}{
		{
			name: "Basic Scenario",
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
				"main":               true, // Main is always considered merged into itself
			},
			cfg: config.Config{
				AgeDays:            90,
				PrimaryMainBranch:  "main",
				ProtectedBranches:  []string{"develop"},
				ProtectedBranchMap: map[string]bool{"develop": true},
			},
			expectedCounts: map[types.BranchCategory]int{
				types.CategoryProtected:   2, // main, develop
				types.CategoryActive:      1, // feature/new
				types.CategoryMergedOld:   2, // feature/old-merged, hotfix/done
				types.CategoryUnmergedOld: 1, // dev/stale
			},
		},
		// TODO: Add more test cases: different age thresholds, no protected, different main branch name, empty input etc.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure ProtectedBranchMap is populated if not explicitly set in test case
			// (though it's good practice to set it explicitly for clarity)
			if tc.cfg.ProtectedBranchMap == nil && len(tc.cfg.ProtectedBranches) > 0 {
				tc.cfg.ProtectedBranchMap = make(map[string]bool)
				for _, pb := range tc.cfg.ProtectedBranches {
					tc.cfg.ProtectedBranchMap[pb] = true
				}
			} else if tc.cfg.ProtectedBranchMap == nil {
				tc.cfg.ProtectedBranchMap = make(map[string]bool)
			}


			analyzed := AnalyzeBranches(tc.branches, tc.mergedStatus, tc.cfg)

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
			// Check if any unexpected categories appeared
			for category, count := range counts {
				if _, expected := tc.expectedCounts[category]; !expected && count > 0 {
					t.Errorf("Got %d branches in unexpected category %s", count, category)
				}
			}
		})
	}
}
