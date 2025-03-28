# Git Sweep Go - Improvement Tracking

This document tracks suggested improvements based on the code review conducted on March 28, 2025.

## 1. Enhanced Testing

**Goal:** Increase test coverage and confidence in the application's correctness across various scenarios and edge cases.

**Status:** Completed (Initial Pass)
**Priority:** High

**Summary:** Added unit tests using mocking for `internal/gitcmd` functions (`query`, `delete`, `fetch`). Expanded unit tests for `internal/analyze`. Added basic unit tests for TUI model updates (`internal/tui`) covering navigation, selection, and state transitions. Expanded integration tests (`cmd/git-sweep`) to cover `--quick-status` and flag overrides.

**Details & Action Items:**

- **Git Command Mocking (`internal/gitcmd`):**
  - [x] Add tests for `GetMainBranchHash`, `IsInGitRepo`, `GetCurrentBranchName` in `query_test.go`. (Existing tests for `GetAllLocalBranchInfo`, `GetMergedBranches` were already present).
  - [x] Create comprehensive tests for `delete.go`.
  - [x] Create comprehensive tests for `fetch.go`.
  - [x] Utilize the `GitRunner` mock interface (`gitcmd.Runner = mockRunner`) extensively.
  - [x] Simulate various `git` command outputs:
    - Successful execution with expected output.
    - Empty output (e.g., no branches, no merged branches).
    - Git command errors (e.g., non-zero exit code, specific stderr messages like "branch not found", "permission denied", "not a git repository").
    - Edge cases in parsing (e.g., unusual branch names, different date formats if applicable).
  - [x] Verify that the functions return the correct data structures or errors based on the mocked Git output.
- **Analysis Logic (`internal/analyze`):**
  - [x] Expand `analyze_test.go` with more test cases for `AnalyzeBranches`.
  - [x] Cover combinations:
    - Merged vs. Unmerged branches.
    - Branches older/newer than `AgeDays`.
    - Branches listed in `ProtectedBranches`.
    - The `PrimaryMainBranch` itself.
    - The `currentBranchName`.
    - Branches with/without remotes.
  - [x] Assert that the returned `AnalyzedBranch` slice has the correct categories (`Protected`, `MergedOld`, `UnmergedOld`, `Active`) and flags (`IsMerged`, `IsOldByAge`, `IsProtected`, `IsCurrent`).
- **TUI Interaction (`internal/tui`):**
  - [x] Investigate using `bubbletea/testenv` or manual simulation to test the `Model.Update` function. (Manual simulation used).
  - [x] Send sequences of `tea.KeyMsg` (up, down, space, tab, enter, y, n, q, ctrl+c) and verify state transitions (`viewState`).
  - [x] Check that `selectedLocal` and `selectedRemote` maps are updated correctly based on selections and `isSelectable` logic.
  - [x] Verify that the correct `tea.Cmd` (e.g., `performDeletionCmd`, `tea.Quit`) is returned under different conditions. (Basic check implemented).
  - [x] Test navigation bounds and behavior with empty lists.
- **Integration Tests (`cmd/git-sweep/main_integration_test.go`):**
  - Add tests that execute the compiled binary (`git-sweep`) against temporary Git repositories.
  - Set up repositories with specific branch structures (merged, old, protected, current).
  - Test various command-line flag combinations:
    - `--dry-run` (verify output, ensure no changes).
    - `--config` with a custom config file.
    - `--age`, `--primary-main`, `--protected` overrides.
    - `--quick-status`.
  - Test first-run setup simulation (requires mocking stdin/stdout or filesystem interaction).
  - Test error conditions: running outside a Git repo, invalid primary branch config, fetch errors.

## 2. Refined Error Handling & Logging

**Goal:** Provide clearer, more user-friendly error messages and implement structured logging for better debugging and operational insight.

**Status:** Not Started
**Priority:** Medium

**Details & Action Items:**

- **Implement Structured Logging:**
  - Choose a logging library (e.g., standard `log/slog`).
  - Replace all `fmt.Printf` / `fmt.Fprintln(os.Stderr, ...)` calls used for debugging (`logDebugf`, `logDebugln`) or warnings/errors with logger calls (e.g., `logger.Debug`, `logger.Warn`, `logger.Error`).
  - Configure the logger to output only DEBUG level messages when the `--debug` flag is set.
  - Ensure errors passed up the call stack are logged appropriately at the point where they are handled (e.g., in `main.go`).
- **Improve Git Error Parsing:**
  - In `internal/gitcmd/runner.go`, when `cmd.Run()` returns an error, analyze the `stderrBuf` more deeply.
  - Identify common Git error patterns (e.g., `fatal: Not a git repository`, `error: branch '...' not found.`, `error: permission denied`, `error: failed to push some refs`).
  - Wrap the original error with a more specific, user-friendly error type or message where possible.
  - Update `internal/gitcmd/delete.go` (lines 69-76) to use this improved error information instead of basic string splitting.
- **Distinguish Warnings vs. Errors:**
  - Clearly differentiate between situations that should halt execution (e.g., invalid config, cannot find primary branch) and those that can proceed with a warning (e.g., `git fetch` failure). Use appropriate log levels (WARN vs. ERROR) and user messages.
- **Address TODOs:**
  - Implement the improved check in `internal/gitcmd/query.go` (line 36) to reliably distinguish "no branches found" from other `git for-each-ref` errors. Check the specific exit code or stderr message from the `git` command.

## 3. Configuration Validation

**Goal:** Prevent runtime errors and provide earlier feedback to the user by validating configuration values during setup and loading.

**Status:** Not Started
**Priority:** Medium

**Details & Action Items:**

- **First-Run Setup Validation (`internal/config/setup.go`):**
  - After prompting for `primary_main_branch` (line 38), add a step to validate the input.
  - Use `gitcmd.RunGitCommand(ctx, "rev-parse", "--verify", input)` (or similar).
  - If the command fails (indicating the branch doesn't exist locally), inform the user and re-prompt for the branch name until a valid one is entered or they choose to keep the default. This requires passing the `context.Context` to `FirstRunSetup`.
- **Load Time Validation (`internal/config/config.go` & `cmd/git-sweep/main.go`):**
  - After loading the config in `main.go`'s `PersistentPreRunE` (around line 198), add a check _before_ fetching or analyzing.
  - Call `gitcmd.GetMainBranchHash(ctx, appConfig.PrimaryMainBranch)` early.
  - If it returns an error, report a fatal error to the user indicating the configured primary branch is invalid in the current repository and exit.
- **Age Validation:**
  - In `FirstRunSetup` (line 24), ensure the parsed `age` is strictly greater than 0. If not, use the default and inform the user.
  - In `LoadConfig` (line 92), ensure `cfg.AgeDays` is strictly greater than 0 after loading from the file. If not, reset to `defaultAgeDays`.
  - In `main.go`'s `PersistentPreRunE` (line 175), ensure the `--age` flag override is strictly greater than 0 if provided.

## 4. TUI Usability Enhancements

**Goal:** Improve the clarity, readability, and robustness of the Terminal User Interface.

**Status:** Completed (Initial Pass)
**Priority:** Low-Medium

**Summary:** Added visual separators between branch groups, added a selection count summary to the footer, and implemented basic handling for terminal resize events in the TUI model.

**Details & Action Items:**

- **Visual Grouping (`internal/tui/model.go` View):**
  - [x] Add horizontal lines or distinct spacing using `lipgloss` between the "Key/Protected", "Suggested", and "Active" sections in the `stateSelecting` view (around lines 299, 354).
- **Selection Summary (`internal/tui/model.go` View):**
  - [x] In the footer of the `stateSelecting` view (near line 386), add text dynamically showing the current selection counts (e.g., `fmt.Sprintf("Selected: %d local, %d remote", len(m.selectedLocal), len(m.selectedRemote))`).
- **Window Size Awareness (`internal/tui/model.go`):**
  - [x] Add `width`, `height` fields to the `Model` struct.
  - [x] Handle the `tea.WindowSizeMsg` in the `Update` function to store the terminal width/height.
  - [ ] Use the stored `width` in the `View` function to potentially truncate long branch names or adjust layout, preventing wrapping issues. (Basic handling added, truncation not implemented).
  - Add `width`, `height` fields to the `Model` struct (line 71).
  - Handle `tea.WindowSizeMsg` in the `Update` function: update `m.width` and `m.height`.
  - In the `View` function, use `m.width` to:
    - Potentially truncate long branch names (`branch.Name`, `remoteInfo`) if they exceed available space, adding ellipsis (...).
    - Ensure help text and other elements wrap correctly or are positioned appropriately.

## 5. Git Interaction Robustness

**Goal:** Harden the parsing of Git command output and potentially provide more nuanced feedback.

**Status:** Not Started
**Priority:** Low

**Details & Action Items:**

- **Date Parsing (`internal/gitcmd/query.go`):**
  - Investigate the exact format guarantees of `%(committerdate:iso8601)`.
  - If variations are possible across Git versions/locales, consider replacing `time.Parse("2006-01-02 15:04:05 -0700", dateStr)` (line 74) with `time.Parse(time.RFC3339, ...)` or a more flexible ISO 8601 library function that handles different timezone offset formats or precision. Add test cases for different valid date strings.
- **Remote Deletion Feedback (`internal/gitcmd/delete.go`):**
  - Analyze the `stdout` and `stderr` from `git push <remote> --delete <branch>` more closely (currently only `err` is checked).
  - Check if Git provides distinct output when the remote branch was successfully deleted versus when it didn't exist to begin with (e.g., messages like `To <url>`, `- [deleted] <branch>`, or `error: unable to delete '<branch>': remote ref does not exist`).
  - If reliable patterns exist, update the `result.Message` in `DeleteBranches` (lines 76, 79) to provide more specific feedback (e.g., "Successfully deleted remote branch", "Remote branch already gone", "Failed: remote branch not found"). This might be low-yield if Git's output isn't consistent enough.
