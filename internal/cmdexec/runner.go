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

type RealRunner struct {
	LogWriter io.Writer
}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer

	var writers []io.Writer
	writers = append(writers, &out)
	if r.LogWriter != nil {
		writers = append(writers, r.LogWriter)
	}
	if stream := logstream.Writer(ctx); stream != nil {
		writers = append(writers, stream)
	}
	w := io.MultiWriter(writers...)

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
