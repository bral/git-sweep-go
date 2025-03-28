# Git Sweep

**Project Specification: `git-sweep` - Interactive Git Branch Cleanup Tool**

**Version:** 1.0 (Draft)
**Date:** March 27, 2025

**Table of Contents:**

1.  [Overview & Goal](#overview--goal)
2.  [Core Features](#core-features)
3.  [Technical Stack](#technical-stack)
4.  [Configuration](#configuration)
    - [File Location & Format](#file-location--format)
    - [Configuration Fields](#configuration-fields)
    - [First-Run Setup](#first-run-setup)
5.  [Core Logic Workflow](#core-logic-workflow)
6.  [Data Structures](#data-structures)
7.  [Project Structure](#project-structure)
8.  [Module Breakdown](#module-breakdown)
    - [`cmd/git-sweep/`](#cmdgit-sweep)
    - [`internal/analyze/`](#internalanalyze)
    - [`internal/config/`](#internalconfig)
    - [`internal/gitcmd/`](#internalgitcmd)
    - [`internal/tui/`](#internaltui)
    - [`internal/types/`](#internaltypes)
9.  [User Interface (TUI)](#user-interface-tui)
    - [UI Flow Overview](#ui-flow-overview)
    - [Key Screens (Mockups)](#key-screens-mockups)
    - [Interaction Model](#interaction-model)
10. [Command-Line Interface (CLI)](#command-line-interface-cli)
11. [Error Handling Strategy](#error-handling-strategy)

---

## 1. Overview & Goal

`git-sweep` will be a command-line tool written in Go, designed to help users keep their local Git repositories tidy. It interactively detects local Git branches that are potentially "old" (based on merge status or age), allows the user to select branches for deletion (including optionally selecting associated remote branches), and executes the deletion commands upon confirmation. The tool prioritizes safety, clarity, and ease of use through an interactive terminal UI (TUI).

## 2. Core Features

- **Git Repository Detection:** Operates only within a valid Git working directory.
- **Branch Analysis:** Identifies local branches.
- **"Old" Branch Criteria:** Considers a branch "old" if:
  - It has already been merged into the configured `primary_main_branch` (e.g., `main`, `master`).
  - **OR** Its last commit date is older than a user-configured age threshold (e.g., 90 days).
- **Configurable Behavior:**
  - Age threshold (days).
  - The `primary_main_branch` to check merge status against.
  - List of `protected` branches that are never suggested for deletion.
- **Remote Branch Awareness:** Detects the associated remote tracking branch (if any).
- **Interactive TUI (Terminal UI):**
  - Presents deletable branches grouped by "Merged" and "Unmerged".
  - Displays branch name, remote name (if applicable), and last commit age.
  - Allows multi-selection of branches for _local_ deletion.
  - Allows separate multi-selection for _remote_ branch deletion (only if local is selected and remote exists).
  - Requires explicit confirmation before any deletion actions.
- **Safe Deletion:**
  - Uses `git branch -d` (safe delete) for merged branches.
  - Uses `git branch -D` (force delete) for unmerged branches, clearly warning the user.
  - Uses `git push <remote> --delete <branch>` for remote deletions.
- **Dry Run Mode:** A `--dry-run` flag performs analysis and shows the interactive UI, including the confirmation screen listing intended actions, but executes no actual `git` deletion commands.
- **Clear Results:** Reports the success or failure of each individual local and remote deletion attempt.

## 3. Technical Stack

- **Language:** Go
- **Git Interaction:** Executing standard `git` CLI commands as subprocesses.
- **CLI Framework:** `github.com/spf13/cobra`
- **TUI Framework:** `github.com/charmbracelet/bubbletea`
- **TUI Styling:** `github.com/charmbracelet/lipgloss` (Optional, recommended)
- **Spinner/Activity:** `github.com/charmbracelet/bubbles/spinner`
- **Configuration Parsing:** `github.com/BurntSushi/toml`

## 4. Configuration

### 4.1 File Location & Format

- **Location:** `~/.config/git-sweep/config.toml` (Respects `os.UserConfigDir()`). A `--config` flag can override this path.
- **Format:** TOML

### 4.2 Configuration Fields

```toml
# Example config.toml

# Age in days for a branch's last commit to be considered "old" if unmerged.
age_days = 90

# The single main branch to check merge status against.
# The tool will check if other branches have been merged into this one.
primary_main_branch = "main"

# Branches that will never be suggested for deletion, regardless of status.
# Glob patterns are NOT currently supported, use exact names.
protected_branches = ["develop", "release"]
```

- **`age_days` (integer):** Default `90` if omitted or <= 0.
- **`primary_main_branch` (string):** Default `"main"` if omitted or empty. Must be a valid, existing local branch name.
- **`protected_branches` (array of strings):** Defaults to empty `[]` if omitted.

### 4.3 First-Run Setup

- If the configuration file is not found when `git-sweep` runs, it will trigger an interactive command-line prompt:
  - Asks the user for the desired `age_days`.
  - Asks for the `primary_main_branch` name (e.g., "main", "master").
  - Asks for comma-separated `protected_branches`.
- Uses sensible defaults if the user provides no input.
- Saves the generated configuration to the default file path.
- Informs the user where the file was saved.

## 5. Core Logic Workflow

The application executes the following steps sequentially:

1.  **Load Configuration:** Read `config.toml`. If not found, run First-Run Setup. Apply any command-line flag overrides (`--age`, `--primary-main`, `--protected`).
2.  **Check Environment:** Verify the current directory is inside a Git repository (`git rev-parse --is-inside-work-tree`). Exit if not.
3.  **Fetch Remote State:** Run `git fetch <remote> --prune` (where `<remote>` is from `--remote` flag or defaults to `origin`) to update local knowledge of remote branches and remove stale tracking refs. Warn on failure but attempt to continue.
4.  **Gather Branch Data:**
    - Get all local branches with upstream, last commit date, and hash (`git for-each-ref refs/heads/ --format=...`).
    - Determine the commit hash of the configured `primary_main_branch` (`git rev-parse <primary_main_branch>`). Error if this branch doesn't exist.
    - Get the set of branches already merged into the `primary_main_branch` hash (`git branch --merged <primary_main_hash>`).
5.  **Analyze Branches:** Iterate through all local branches, applying the configuration rules (`AgeDays`, `PrimaryMainBranch`, `ProtectedBranches`) and merge status data to categorize each branch (`Protected`, `Active`, `MergedOld`, `UnmergedOld`).
6.  **Filter Candidates:** Select only branches categorized as `MergedOld` or `UnmergedOld` for presentation in the TUI. Exit gracefully if no candidates are found.
7.  **Launch Interactive TUI:** Start the `bubbletea` application.
    - Display candidate branches (grouped).
    - Handle user navigation and selection (local/remote toggles).
    - Show confirmation screen listing actions.
    - Wait for user confirmation ('y'/'n').
8.  **Execute Deletions (if confirmed & not dry run):**
    - Call the deletion logic (`internal/gitcmd.DeleteBranches`).
    - Execute `git branch -d/-D` for selected local branches.
    - Execute `git push <remote> --delete` for selected remote branches.
9.  **Display Results:** Show the final TUI screen listing the success/failure status of each attempted deletion.
10. **Exit.**

## 6. Data Structures

Key Go types defined in `internal/types/branch.go`:

```go
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

// (From internal/gitcmd/delete.go) DeleteResult holds outcome of one delete attempt.
type DeleteResult struct {
    BranchName string
    IsRemote   bool
    RemoteName string // Only if IsRemote is true
    Success    bool
    Message    string // Success message or error details
    Cmd        string // The command attempted
}
```

## 7. Project Structure

```plaintext
git-sweep/
├── cmd/
│   └── git-sweep/
│       └── main.go            # Main application entry point, Cobra command, orchestration
├── internal/
│   ├── analyze/
│   │   └── analyze.go         # Branch classification logic
│   ├── config/
│   │   ├── config.go          # Config struct, Load/Save funcs
│   │   └── setup.go           # First-run interactive setup
│   ├── gitcmd/
│   │   ├── runner.go          # Core RunGitCommand helper
│   │   ├── query.go           # Git query funcs (branch info, merge status, etc.)
│   │   ├── fetch.go           # FetchAndPrune func
│   │   └── delete.go          # DeleteBranches func (local/remote)
│   ├── tui/
│   │   ├── model.go           # Bubbletea Model struct (state)
│   │   ├── update.go          # Bubbletea Update func (input/message handling)
│   │   ├── view.go            # Bubbletea View func (rendering UI)
│   │   ├── commands.go        # Bubbletea Cmds (e.g., triggering deletion)
│   │   └── styles.go          # (Optional) Lipgloss styling
│   └── types/
│       └── branch.go          # Shared data structures (BranchInfo, AnalyzedBranch)
├── go.mod
├── go.sum
└── README.md
```

## 8. Module Breakdown

- **`cmd/git-sweep/`**: Entry point. Uses `cobra` for CLI flags/args. Orchestrates the overall workflow by calling functions from `internal` packages. Handles top-level errors.
- **`internal/analyze/`**: Contains the `AnalyzeBranches` function. Takes raw branch data (`[]types.BranchInfo`), merge status (`map[string]bool` indicating merged into `primary_main_branch`), and configuration (`config.Config`) to produce analyzed branches (`[]types.AnalyzedBranch`) with categories. Pure logic, no Git interaction.
- **`internal/config/`**: Handles loading/saving the `config.toml` file. Defines the `Config` struct. Includes the interactive `FirstRunSetup` logic.
- **`internal/gitcmd/`**: The sole interface to the `git` CLI.
  - `runner.go`: Provides `RunGitCommand` to execute `git` commands safely, capture output, handle errors, and manage context/timeouts.
  - `query.go`: Implements functions to get specific info (e.g., `IsInGitRepo`, `GetAllLocalBranchInfo`, `GetMergedBranches`, `GetMainBranchHash`, `GetRemotes`) by running `git` commands and parsing their output into Go types. Key commands: `git rev-parse`, `git remote`, `git branch --merged`, `git for-each-ref`.
  - `fetch.go`: Implements `FetchAndPrune` (`git fetch --prune`).
  - `delete.go`: Implements `DeleteBranches` which takes the list of branches and user selections, then executes `git branch -d/-D` and `git push --delete`. Returns detailed `DeleteResult` for each attempt.
- **`internal/tui/`**: Implements the interactive terminal UI using `bubbletea`.
  - Defines the `Model` holding UI state (cursor, selections, current view, results, etc.).
  - Implements `Init`, `Update`, `View` methods.
  - Handles keyboard input, state transitions (selecting -> confirming -> deleting -> results).
  - Renders the different screens based on the model state.
  - Uses a `deleterFunc` (passed from `main`) to trigger deletions via a `tea.Cmd`.
- **`internal/types/`**: Defines shared data structures (`BranchInfo`, `AnalyzedBranch`, `BranchCategory`, `DeleteResult`) used across multiple internal packages to avoid import cycles and ensure consistency.

## 9. User Interface (TUI)

### 9.1 UI Flow Overview

The TUI follows the "Checkbox List" style (Option 1 discussed).

1.  **Selection Screen:** Displays branches categorized under "Merged" and "Unmerged". User navigates with arrow keys. Space toggles local deletion selection. Tab (or 'r') toggles remote deletion (only if local is selected and remote exists).
2.  **Confirmation Screen:** Shown after pressing Enter with selections. Lists exactly which local branches will be deleted (with `-d` vs `-D` indication) and which remote branches will be deleted. Requires 'y'/'n' confirmation (defaults 'n'). Clearly indicates if in dry-run mode.
3.  **Deleting Screen:** Shown briefly while deletion commands run (if not dry run). Displays a spinner/activity indicator.
4.  **Results Screen:** Displays the outcome (success ✅ / failure ❌ / info ℹ️ / dry-run notice) for each attempted local and remote deletion. User presses any key to exit.

### 9.2 Key Screens (Mockups)

**(Screen 1: Selection)**

```text
git-sweep: Select branches to delete (Dry Run Mode)

Use ↑/↓ to navigate. Space: Toggle local deletion. Tab/r: Toggle remote deletion.

Merged Branches (Safe to delete local):
> [ ] Local: feature/old-merged     Remote: [x] (origin/feature/old-merged)   (95 days ago)
  [ ] Local: hotfix/done            Remote: [x] (origin/hotfix/done)          (62 days ago)

Unmerged Branches (⚠️ Deleting local requires force -D):
  [ ] Local: dev/stale-experiment   Remote: [x] (origin/dev/stale-experiment) (110 days ago)
  [ ] Local: old-feature-no-remote  Remote: [ ] ((none))                      (125 days ago)


Enter: Review & Confirm | q/Ctrl+C: Quit
```

**(Screen 2: Confirmation - Assuming first two selected Local+Remote)**

```text
git-sweep: Confirm Actions (Dry Run Mode - No changes will be made)

You have chosen the following actions:

Local Deletions:
  - Delete 'feature/old-merged'   (Merged, safe -d)
  - Delete 'hotfix/done'          (Merged, safe -d)

Remote Deletions:
  - Delete remote 'origin/feature/old-merged'
  - Delete remote 'origin/hotfix/done'

This action cannot be easily undone for deleted branches.

Proceed? (y/N) N
```

**(Screen 3: Results - Assuming Dry Run confirmation)**

```text
git-sweep: Deletion Results (Dry Run Completed)

ℹ️ Would delete local branch feature/old-merged (using -d)
ℹ️ Would delete local branch hotfix/done (using -d)
ℹ️ Would delete remote branch origin/feature/old-merged
ℹ️ Would delete remote branch origin/hotfix/done


Press any key to exit.
```

### 9.3 Interaction Model

- **Navigation:** `Up`/`Down` arrows (or `k`/`j`).
- **Select Local:** `Spacebar` (toggles `[ ]`/`[x]` next to "Local:"). Deselecting local also deselects remote.
- **Select Remote:** `Tab` or `r` (toggles `[ ]`/`[x]` next to "Remote:"). Only enabled if local is selected and a remote exists.
- **Confirm Selections:** `Enter` (moves from Selection to Confirmation screen).
- **Confirm Deletion:** `y` (on Confirmation screen; triggers deletion or shows dry-run results).
- **Cancel Confirmation:** `n` or `Esc` (on Confirmation screen; returns to Selection screen).
- **Quit:** `q` or `Ctrl+C` (exits the application from most screens).

## 10. Command-Line Interface (CLI)

Defined using `cobra`.

**Usage:** `git-sweep [flags]`

**Flags:**

- `--dry-run`: (Bool, default `false`) Analyze and preview actions, but do not delete.
- `-c, --config`: (String, default `""`) Path to custom configuration file.
- `-r, --remote`: (String, default `"origin"`) Specify the remote repository to fetch from and consider for remote deletions.
- `--age`: (Int, default `0`) Override config: Max age (in days) for unmerged branches.
- `--primary-main`: (String, default `""`) Override config: The single main branch name to check merge status against.
- `--protected`: (StringSlice, default `[]`) Override config: Comma-separated list of protected branch names.

## 11. Error Handling Strategy

- **Git Command Failures:** `internal/gitcmd.RunGitCommand` captures `stderr` and exit codes. Errors are wrapped with context (`fmt.Errorf`) and propagated up.
- **Configuration Errors:** `internal/config.LoadConfig` returns specific `config.ErrConfigNotFound` for triggering setup. Other file I/O or parsing errors are wrapped and reported. `SaveConfig` errors are also wrapped.
- **Non-Git Repo:** Detected early; exits with a clear message.
- **Fetch Failure:** Treated as a warning; allows the tool to proceed using potentially stale info but informs the user.
- **Analysis Errors:** Failure to find the configured `primary_main_branch` or its hash is treated as fatal. Parsing errors during branch info gathering should log warnings but attempt to continue with other branches.
- **TUI Errors:** `bubbletea` program errors are caught in `main.go`. Internal TUI errors (e.g., during deletion callback) are stored in `model.fatalErr` and displayed before exit.
- **Deletion Results:** Handled individually per branch (local/remote). The `internal/gitcmd.DeleteBranches` returns a `[]DeleteResult` slice detailing success/failure for _each_ operation, which is then displayed clearly in the final TUI screen.
