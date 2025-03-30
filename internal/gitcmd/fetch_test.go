package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	// Removed reflect import as reflectDeepEqual is removed
)

// Note: The setupMockRunner function is defined in test_helpers_test.go

func TestFetchAndPrune(t *testing.T) {
	ctx := context.Background()
	remoteName := "origin"

	// --- Test Case 1: Successful Fetch ---
	t.Run("Successful Fetch", func(t *testing.T) {
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
			expectedArgs := []string{"fetch", remoteName, "--prune"}
			// Simple comparison is sufficient here as the mock logic is specific to this test case
			if len(args) != len(expectedArgs) {
				return "", fmt.Errorf("unexpected command args length: got %d, want %d", len(args), len(expectedArgs))
			}
			for i := range args {
				if args[i] != expectedArgs[i] {
					return "", fmt.Errorf("unexpected command arg at index %d: got %q, want %q", i, args[i], expectedArgs[i])
				}
			}
			return "Fetch output", nil // Output doesn't matter much, just success
		})
		defer teardown()

		err := FetchAndPrune(ctx, remoteName)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	// --- Test Case 2: Git Command Error ---
	t.Run("Git Command Error", func(t *testing.T) {
		expectedErr := errors.New("simulated fetch error")
		teardown := setupMockRunner(t, func(_ context.Context, _ ...string) (string, error) { // Use setupMockRunner
			// Simulate the runner returning an error
			return "", expectedErr
		})
		defer teardown()

		err := FetchAndPrune(ctx, remoteName)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		// Check if the error message contains the remote name and wraps the original
		if !strings.Contains(err.Error(), remoteName) {
			t.Errorf("Expected error message to contain remote name %q, got: %v", remoteName, err)
		}
		// Note: errors.Is might not work directly if the error is wrapped with fmt.Errorf
		// Check for substring instead for simplicity here.
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Errorf("Expected error message to contain original error '%v', got: %v", expectedErr, err)
		}
	})

	// --- Test Case 3: Empty Remote Name ---
	t.Run("Empty Remote Name", func(t *testing.T) {
		// Runner should not be called
		teardown := setupMockRunner(t, func(_ context.Context, args ...string) (string, error) { // Use setupMockRunner
			t.Errorf("Runner should not be called with empty remote name, called with: %v", args)
			return "", errors.New("runner called unexpectedly")
		})
		defer teardown()

		err := FetchAndPrune(ctx, "") // Call with empty remote
		if err == nil {
			t.Fatal("Expected an error for empty remote name, got nil")
		}
		if !strings.Contains(err.Error(), "remote name cannot be empty") {
			t.Errorf("Expected error message about empty remote name, got: %v", err)
		}
	})
}

// Removed reflectDeepEqual helper function as it's no longer needed
