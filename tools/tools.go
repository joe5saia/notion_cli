//go:build tools

package tools

// This file tracks CLI tool dependencies via go modules.
import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "mvdan.cc/gofumpt"
)
