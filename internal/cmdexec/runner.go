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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.WaitDelay = killGrace + time.Second
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
	if err := cmd.Start(); err != nil {
		return out.Bytes(), err
	}
	pgid := cmd.Process.Pid
	waited := make(chan struct{})
	cancelled := make(chan bool, 1)
	go func() { cancelled <- terminateOnCancel(ctx, pgid, waited) }()
	err := cmd.Wait()
	close(waited)
	wasCancelled := <-cancelled
	if wasCancelled || errors.Is(err, exec.ErrWaitDelay) {
		_ = signalGroup(pgid, syscall.SIGKILL)
	}
	if wasCancelled && err == nil {
		err = ctx.Err()
	}
	if lines != nil {
		lines.flush()
	}
	return out.Bytes(), err
}

func terminateOnCancel(ctx context.Context, pgid int, waited <-chan struct{}) bool {
	select {
	case <-waited:
		return false
	case <-ctx.Done():
	}
	_ = signalGroup(pgid, syscall.SIGTERM)
	select {
	case <-waited:
	case <-time.After(killGrace):
		_ = signalGroup(pgid, syscall.SIGKILL)
	}
	return true
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
