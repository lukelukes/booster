package cmdexec

import (
	"booster/internal/logstream"
	"bytes"
	"context"
	"io"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)

	LookPath(name string) (string, error)
}

type RealRunner struct{}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer

	var w io.Writer = &out
	if stream := logstream.Writer(ctx); stream != nil {
		w = io.MultiWriter(&out, stream)
	}

	cmd.Stdout = w
	cmd.Stderr = w
	err := cmd.Run()
	return out.Bytes(), err
}

func (r *RealRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func DefaultRunner() Runner {
	return &RealRunner{}
}
