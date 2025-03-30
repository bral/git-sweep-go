// Package types defines shared data structures used across the git-sweep application.
package types

import "time"

// BranchInfo holds raw Git data for a local branch.
type BranchInfo struct {
	Name           string
	Upstream       string // e.g., "origin/feature/x"
	Remote         string // e.g., "origin"
	LastCommitDate time.Time
	CommitHash     string
}

// BranchCategory classifies a branch after analysis.
type BranchCategory string

// Branch category constants.
const (
	// CategoryProtected indicates a branch protected by config, the primary main branch, or the current branch.
	CategoryProtected BranchCategory = "Protected"
	// CategoryActive indicates a branch that is not protected, not merged, and not old.
	CategoryActive BranchCategory = "Active"
	// CategoryMergedOld indicates a branch that is merged into the primary main branch.
	CategoryMergedOld BranchCategory = "MergedOld"
	// CategoryUnmergedOld indicates a branch that is not protected, not merged, but is old based on commit date.
	CategoryUnmergedOld BranchCategory = "UnmergedOld"
)

// AnalyzedBranch contains processed branch info for UI and decisions.
type AnalyzedBranch struct {
	BranchInfo  // Embedded raw info
	IsMerged    bool
	IsOldByAge  bool
	IsProtected bool
	IsCurrent   bool // Added flag for current branch
	Category    BranchCategory
}

// DeleteResult holds outcome of one delete attempt.
type DeleteResult struct {
	BranchName  string
	IsRemote    bool
	RemoteName  string // Only if IsRemote is true
	Success     bool
	Message     string // Success message or error details
	Cmd         string // The command attempted
	DeletedHash string // Commit hash of the branch before deletion (if successful)
}
