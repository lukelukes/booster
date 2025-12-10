// Package cmdexec provides an abstraction for executing shell commands.
// This enables testability by allowing mock implementations.
package cmdexec

import (
	"bytes"
	"context"
	"os/exec"
)

// Runner executes shell commands.
type Runner interface {
	// Run executes a command and returns its combined output.
	// Returns error if command fails (non-zero exit code).
	Run(ctx context.Context, name string, args ...string) ([]byte, error)

	// LookPath searches for an executable in PATH.
	// Returns the full path if found, error otherwise.
	LookPath(name string) (string, error)
}

// RealRunner executes commands on the actual system.
type RealRunner struct{}

// Run executes the command and returns combined stdout/stderr.
func (r *RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

// LookPath searches for the executable in PATH.
func (r *RealRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// DefaultRunner returns a runner that executes real system commands.
func DefaultRunner() Runner {
	return &RealRunner{}
}
