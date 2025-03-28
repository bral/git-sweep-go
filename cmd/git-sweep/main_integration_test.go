//go:build integration
// +build integration

// Integration tests require the 'integration' build tag to run:
// go test -tags=integration ./cmd/git-sweep/...

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time" // Added for commit timestamps
)

var (
	// Path to the compiled binary used for testing.
	binaryPath string
)

// runCmd is a helper to execute shell commands, typically git.
func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	outBytes, err := cmd.CombinedOutput() // Capture both stdout and stderr
	output := string(outBytes)
	if err != nil {
		t.Fatalf("Command failed: %s %v\nOutput:\n%s\nError: %v", name, args, output, err)
	}
	// t.Logf("Ran: %s %v\nOutput:\n%s", name, args, output) // Optional logging
	return output
}

// setupTestRepo creates a temporary directory, initializes a git repo,
// and returns the path and a cleanup function.
func setupTestRepo(t *testing.T) (repoPath string, cleanup func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "git-sweep-test-repo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	repoPath = tempDir

	// Initialize the repository first
	runCmd(t, repoPath, "git", "init", "-b", "main")

	// Basic git config needed for commits (run after init)
	runCmd(t, repoPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoPath, "git", "config", "user.name", "Test User")

	// Create the initial commit (run after init and config)
	runCmd(t, repoPath, "git", "commit", "--allow-empty", "-m", "Initial commit")

	cleanup = func() {
		err := os.RemoveAll(repoPath)
		if err != nil {
			t.Logf("Warning: Failed to remove test repo %q: %v", repoPath, err)
		}
	}
	return repoPath, cleanup
}

// createBranchAndCommit creates a branch and adds an empty commit.
func createBranchAndCommit(t *testing.T, repoPath, branchName, message string, commitDate time.Time) {
	t.Helper()
	runCmd(t, repoPath, "git", "checkout", "-b", branchName)
	// Set committer date for the commit
	dateStr := commitDate.Format(time.RFC3339) // Use a format git understands
	env := append(os.Environ(), fmt.Sprintf("GIT_COMMITTER_DATE=%s", dateStr))
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", message, "--date", dateStr)
	cmd.Dir = repoPath
	cmd.Env = env
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to commit on branch %s: %v\nOutput:\n%s", branchName, err, string(outBytes))
	}
	runCmd(t, repoPath, "git", "checkout", "main") // Go back to main
}

// TestMain runs setup before all tests in the package.
func TestMain(m *testing.M) {
	fmt.Println("Building binary for integration tests...")

	binaryName := "git-sweep-test"
	if runtime.GOOS == "windows" { binaryName += ".exe" }

	buildPath, err := filepath.Abs(binaryName)
	if err != nil {
		fmt.Printf("Error getting absolute path for binary: %v\n", err)
		os.Exit(1)
	}
	binaryPath = buildPath

	// Ensure GOBIN is set or default GOPATH/bin exists for dependencies
	// (This might be needed if tests depend on tools installed via go install)

	// Build in current directory
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Binary built at: %s\n", binaryPath)

	exitCode := m.Run()

	fmt.Printf("Removing test binary: %s\n", binaryPath)
	err = os.Remove(binaryPath)
	if err != nil {
		fmt.Printf("Warning: Failed to remove test binary: %v\n", err)
	}

	os.Exit(exitCode)
}

// TestIntegrationDryRun is a basic integration test scenario.
func TestIntegrationDryRun(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -100) // 100 days ago
	recentDate := now.AddDate(0, 0, -10) // 10 days ago

	// Create branches
	createBranchAndCommit(t, repoPath, "merged-recent", "feat: merged recent", recentDate)
	createBranchAndCommit(t, repoPath, "merged-old", "feat: merged old", oldDate)
	createBranchAndCommit(t, repoPath, "unmerged-recent", "feat: unmerged recent", recentDate)
	createBranchAndCommit(t, repoPath, "unmerged-old", "feat: unmerged old", oldDate)
	createBranchAndCommit(t, repoPath, "protected-config", "feat: protected", oldDate) // Will be protected by config

	// Merge some branches
	runCmd(t, repoPath, "git", "merge", "--no-ff", "merged-recent", "-m", "Merge merged-recent")
	runCmd(t, repoPath, "git", "merge", "--no-ff", "merged-old", "-m", "Merge merged-old")

	// Create a test config file
	configContent := `
age_days = 90
primary_main_branch = "main"
protected_branches = ["protected-config"]
`
	configPath := filepath.Join(repoPath, ".git-sweep-test.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Run git-sweep with --dry-run and the test config
	cmd := exec.Command(binaryPath, "--dry-run", "--config", configPath)
	cmd.Dir = repoPath // Run command inside the test repo
	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)

	// Basic assertions (more detailed parsing could be added)
	if err != nil {
		// Dry run shouldn't fail execution itself unless there's a setup issue
		t.Fatalf("git-sweep --dry-run failed unexpectedly:\nOutput:\n%s\nError: %v", output, err)
	}

	// Check stdout for expected candidate branches
	// Merged branches should always be candidates in dry run (unless protected)
	if !strings.Contains(output, "merged-recent") {
		t.Errorf("Expected 'merged-recent' to be listed as candidate, output:\n%s", output)
	}
	if !strings.Contains(output, "merged-old") {
		t.Errorf("Expected 'merged-old' to be listed as candidate, output:\n%s", output)
	}
	// Only old unmerged branches should be candidates
	if strings.Contains(output, "unmerged-recent") {
		t.Errorf("Did not expect 'unmerged-recent' to be listed as candidate, output:\n%s", output)
	}
	if !strings.Contains(output, "unmerged-old") {
		t.Errorf("Expected 'unmerged-old' to be listed as candidate, output:\n%s", output)
	}
	// Protected branches should NOT be listed
	if strings.Contains(output, "protected-config") {
		t.Errorf("Did not expect 'protected-config' to be listed as candidate, output:\n%s", output)
	}
	if strings.Contains(output, "main") { // Main branch should also be protected
		t.Errorf("Did not expect 'main' to be listed as candidate, output:\n%s", output)
	}

	// Check for the dry run indicator in the TUI output (might be fragile)
	if !strings.Contains(output, "[Dry Run]") {
		t.Errorf("Expected '[Dry Run]' indicator in output, output:\n%s", output)
	}

	// TODO: Add more scenarios: different flags, remote branches, current branch protection etc.
}
