//go:build tools
// +build tools

package tools

// This file is used to track development tool dependencies.
// It ensures they're tracked in go.mod but not included in the final build.
import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint" // Linting tool
	_ "golang.org/x/tools/cmd/goimports"                    // Import organization
)
