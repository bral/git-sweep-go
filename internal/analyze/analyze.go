package analyze

import (
	"time"

	"github.com/bral/git-sweep-go/internal/config" // Use the actual config package
	"github.com/bral/git-sweep-go/internal/types"
)

// AnalyzeBranches categorizes branches based on merge status, age, and protection rules.
// It takes raw branch info, a map indicating which branches are merged into the primary main branch,
// and the application configuration.
func AnalyzeBranches(branches []types.BranchInfo, mergedStatus map[string]bool, cfg config.Config) []types.AnalyzedBranch {
	analyzedBranches := make([]types.AnalyzedBranch, 0, len(branches))
	now := time.Now()
	ageThreshold := time.Duration(cfg.AgeDays) * 24 * time.Hour

	// The ProtectedBranchMap is assumed to be populated by LoadConfig now.
	// Ensure it's not nil just in case, though LoadConfig should handle this.
	protectedMap := cfg.ProtectedBranchMap
	if protectedMap == nil {
		protectedMap = make(map[string]bool)
	}

	for _, branch := range branches {
		analyzed := types.AnalyzedBranch{
			BranchInfo:  branch,
			IsMerged:    mergedStatus[branch.Name],
			IsProtected: protectedMap[branch.Name], // Use the map from the config struct
			// Calculate IsOldByAge based on config and last commit date
			IsOldByAge: now.Sub(branch.LastCommitDate) > ageThreshold,
		}

		// Determine Category
		if analyzed.IsProtected {
			analyzed.Category = types.CategoryProtected
		} else if branch.Name == cfg.PrimaryMainBranch {
			// The primary main branch itself is considered protected/active implicitly
			analyzed.Category = types.CategoryProtected
		} else if analyzed.IsMerged {
			// Merged branches are candidates for deletion regardless of age (though age is useful info)
			analyzed.Category = types.CategoryMergedOld
		} else if analyzed.IsOldByAge {
			// Unmerged but old branches are candidates
			analyzed.Category = types.CategoryUnmergedOld
		} else {
			// Neither protected, merged, nor old - considered active
			analyzed.Category = types.CategoryActive
		}

		analyzedBranches = append(analyzedBranches, analyzed)
	}

	return analyzedBranches
}
