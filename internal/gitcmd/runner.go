package gitcmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitRunner defines the function signature for executing git commands.
// This allows mocking the actual git execution during tests.
type GitRunner func(ctx context.Context, args ...string) (stdout string, err error)

// Runner is the package-level variable holding the function used to run git commands.
// It defaults to the real implementation but can be swapped out in tests.
var Runner GitRunner = runGitCommandReal

// runGitCommandReal is the actual implementation that executes git commands.
func runGitCommandReal(ctx context.Context, args ...string) (string, error) {
	// Add a default timeout if the context doesn't have one
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second) // Default 30-second timeout
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())

	if err != nil {
		// Include stderr in the error message for better debugging
		return stdout, fmt.Errorf("git command failed: %w\nargs: %v\nstderr: %s", err, args, stderr)
	}

	return stdout, nil
}

// RunGitCommand is a convenience wrapper that uses the package-level Runner.
// All internal gitcmd functions should use this instead of calling runGitCommandReal directly.
func RunGitCommand(ctx context.Context, args ...string) (string, error) {
	if Runner == nil {
		// Safety check, should not happen if initialized correctly.
		return "", fmt.Errorf("GitRunner is not initialized")
	}
	return Runner(ctx, args...)
}
