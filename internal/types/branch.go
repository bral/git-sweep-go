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
const (
	CategoryProtected BranchCategory = "Protected"
	CategoryActive    BranchCategory = "Active"
	CategoryMergedOld BranchCategory = "MergedOld"
	CategoryUnmergedOld BranchCategory = "UnmergedOld"
)

// AnalyzedBranch contains processed branch info for UI and decisions.
type AnalyzedBranch struct {
	BranchInfo     // Embedded raw info
	IsMerged       bool
	IsOldByAge     bool
	IsProtected    bool
	Category       BranchCategory
}

// DeleteResult holds outcome of one delete attempt.
type DeleteResult struct {
    BranchName string
    IsRemote   bool
    RemoteName string // Only if IsRemote is true
    Success    bool
    Message    string // Success message or error details
    Cmd        string // The command attempted
}
