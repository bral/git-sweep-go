// Package gitcmd contains helpers for testing git command interactions.
// The _test suffix ensures this file is only included during tests.
package gitcmd

import (
	"context"
	"errors"
	"testing"
)

// mockRunner is a helper for tests to mock git command execution.
type mockRunner struct {
	mock func(ctx context.Context, args ...string) (string, error)
}

func (m *mockRunner) run(ctx context.Context, args ...string) (string, error) {
	if m.mock != nil {
		return m.mock(ctx, args...)
	}
	return "", errors.New("mockRunner not implemented")
}

// setupMockRunner sets the package Runner to the mock and returns a teardown function.
// This is a simplified setup for tests that only need a single mock function.
func setupMockRunner(_ *testing.T, mockFunc func(_ context.Context, args ...string) (string, error)) func() {
	originalRunner := Runner
	mock := &mockRunner{mock: mockFunc}
	Runner = mock.run
	// Return a teardown function to restore the original runner
	return func() {
		Runner = originalRunner
	}
}
