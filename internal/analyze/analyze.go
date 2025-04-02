// Package analyze contains the logic for analyzing Git branches based on configuration and merge status.
package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/bral/git-sweep-go/internal/config" // Use the actual config package
	"github.com/bral/git-sweep-go/internal/gitcmd"
	"github.com/bral/git-sweep-go/internal/types"
)

// Branches categorizes branches based on merge status, age, and protection rules.
// It takes raw branch info, a map indicating which branches are merged into the primary main branch,
// the application configuration, and the name of the currently checked-out branch.
// It now also performs a 'git cherry -v' check for non-merged, non-protected branches.
func Branches(
	ctx context.Context, branches []types.BranchInfo, mergedStatus map[string]bool,
	cfg config.Config, currentBranchName string,
) ([]types.AnalyzedBranch, error) {
	analyzedBranches := make([]types.AnalyzedBranch, 0, len(branches))
	now := time.Now()
	ageThreshold := time.Duration(cfg.AgeDays) * 24 * time.Hour

	// The ProtectedBranchMap is assumed to be populated by LoadConfig now.
	// Ensure it's not nil just in case, though LoadConfig should handle this.
	protectedMap := cfg.ProtectedBranchMap
	if protectedMap == nil {
		protectedMap = make(map[string]bool)
	}

	// Default to primary main branch if currentBranchName is empty or not provided
	// This helps in scenarios like CI where HEAD might be detached.
	if currentBranchName == "" {
		currentBranchName = cfg.PrimaryMainBranch
	}

	for _, branch := range branches {
		// Check if explicitly protected by config OR if it's the current branch OR if it's the primary main branch
		isCurrent := branch.Name == currentBranchName
		isProtected := protectedMap[branch.Name] || isCurrent || branch.Name == cfg.PrimaryMainBranch

		// Start with basic merge check (git branch --merged)
		isMerged := mergedStatus[branch.Name]
		mergeMethod := types.MergeMethodStandard // Default method is standard

		// If using enhanced strategy and branch not merged by ancestry check and not protected,
		// perform the 'git cherry -v' check
		if !isMerged && !isProtected && cfg.MergeStrategy == config.MergeStrategyEnhanced {
			var cherryErr error
			// Use the gitcmd.AreChangesIncluded function for advanced detection
			cherryDetected, cherryErr := gitcmd.AreChangesIncluded(ctx, cfg.PrimaryMainBranch, branch.Name)
			if cherryErr != nil {
				// Log the error and treat the branch as not merged for safety.
				// We return the error to halt processing, as a failed check is ambiguous.
				return nil, fmt.Errorf("failed git cherry check for branch %q: %w", branch.Name, cherryErr)
			}

			if cherryDetected {
				isMerged = true
				mergeMethod = types.MergeMethodEnhanced // This branch was detected via cherry check
			}
		}

		analyzed := types.AnalyzedBranch{
			BranchInfo:  branch,
			IsMerged:    isMerged,    // Use the potentially updated status
			MergeMethod: mergeMethod, // Track how branch was determined to be merged
			IsProtected: isProtected,
			IsCurrent:   isCurrent, // Set the new flag
			// Calculate IsOldByAge based on config and last commit date
			IsOldByAge: now.Sub(branch.LastCommitDate) > ageThreshold,
		}

		// Determine Category using a switch for clarity
		switch {
		case analyzed.IsProtected:
			analyzed.Category = types.CategoryProtected
		case analyzed.IsMerged:
			// Merged branches (including those detected by 'git cherry') are candidates for deletion regardless of age
			analyzed.Category = types.CategoryMergedOld
		case analyzed.IsOldByAge:
			// Unmerged but old branches are candidates
			analyzed.Category = types.CategoryUnmergedOld
		default:
			// Neither protected, merged (by either method), nor old - considered active
			analyzed.Category = types.CategoryActive
		}

		analyzedBranches = append(analyzedBranches, analyzed)
	}

	return analyzedBranches, nil
}
