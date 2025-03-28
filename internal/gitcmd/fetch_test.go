package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Note: The setup function is defined in query_test.go but accessible within the package.

func TestFetchAndPrune(t *testing.T) {
	ctx := context.Background()
	remoteName := "origin"

	// --- Test Case 1: Successful Fetch ---
	t.Run("Successful Fetch", func(t *testing.T) {
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
			expectedArgs := []string{"fetch", remoteName, "--prune"}
			if !reflectDeepEqual(args, expectedArgs) { // Using reflect.DeepEqual helper
				return "", fmt.Errorf("unexpected command args: got %v, want %v", args, expectedArgs)
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
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
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
		teardown := setup(t, func(ctx context.Context, args ...string) (string, error) {
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

// reflectDeepEqual is a simple helper for comparing slices in the mock.
// Needed because direct comparison of slices doesn't work.
func reflectDeepEqual(a, b interface{}) bool {
	// Basic type check first
	if fmt.Sprintf("%T", a) != fmt.Sprintf("%T", b) {
		return false
	}

	// Use reflect.DeepEqual for actual comparison
	// Note: This requires importing "reflect"
	// If reflect is not desired, implement manual slice comparison.
	// For this test, let's assume reflect is acceptable.
	// We need to import "reflect" for this.
	// Let's try without reflect first for simplicity in this context.

	aSlice, okA := a.([]string)
	bSlice, okB := b.([]string)

	if !okA || !okB {
		// Fallback or error if not []string, adjust as needed
		return false
	}

	if len(aSlice) != len(bSlice) {
		return false
	}
	for i := range aSlice {
		if aSlice[i] != bSlice[i] {
			return false
		}
	}
	return true
}
