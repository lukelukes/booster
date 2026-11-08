package cmdexec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
)

const killGrace = 2 * time.Second

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)

	LookPath(name string) (string, error)
}

type lineSinkKey struct{}

func WithLineSink(ctx context.Context, sink func(line string)) context.Context {
	return context.WithValue(ctx, lineSinkKey{}, sink)
}

func lineSink(ctx context.Context) func(string) {
	sink, _ := ctx.Value(lineSinkKey{}).(func(string))
	return sink
}

type RealRunner struct {
	LogWriter io.Writer
}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		_ = signalGroup(cmd.Process.Pid, syscall.SIGTERM)
		return signalGroup(cmd.Process.Pid, syscall.SIGCONT)
	}
	cmd.WaitDelay = killGrace
	var out bytes.Buffer

	writers := []io.Writer{&out}
	if r.LogWriter != nil {
		writers = append(writers, r.LogWriter)
	}
	var lines *lineWriter
	if sink := lineSink(ctx); sink != nil {
		lines = &lineWriter{sink: sink}
		writers = append(writers, lines)
	}

	w := io.MultiWriter(writers...)
	cmd.Stdout = w
	cmd.Stderr = w
	err := cmd.Run()
	if cmd.Process != nil {
		_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)
	}
	if lines != nil {
		lines.flush()
	}
	return out.Bytes(), err
}

func signalGroup(pgid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func (r *RealRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func DefaultRunner() Runner {
	return &RealRunner{}
}

type lineWriter struct {
	sink func(string)
	buf  []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.sink(lastSegment(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
}

func (w *lineWriter) flush() {
	if len(w.buf) > 0 {
		w.sink(lastSegment(w.buf))
		w.buf = nil
	}
}

func lastSegment(line []byte) string {
	line = bytes.TrimRight(line, "\r")
	if i := bytes.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	return string(line)
}
