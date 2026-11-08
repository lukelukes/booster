package cmdexec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealRunner_StreamsLinesToTheSinkOnTheContext(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{name: "stdout lines", script: `echo one; echo two`, want: []string{"one", "two"}},
		{name: "stderr interleaved with stdout", script: `echo out; echo err >&2; echo out2`, want: []string{"out", "err", "out2"}},
		{name: "unterminated last line", script: `echo first; printf last`, want: []string{"first", "last"}},
		{name: "line written in pieces", script: `printf 'hel'; sleep 0.05; printf 'lo\r\n'`, want: []string{"hello"}},
		{name: "blank lines kept", script: `echo a; echo; echo b`, want: []string{"a", "", "b"}},
		{name: "progress redraws keep the last frame", script: `printf 'jq 10%%\rjq 60%%\rjq 100%%\n'; echo next`, want: []string{"jq 100%", "next"}},
		{name: "unterminated progress keeps the last frame", script: `printf 'jq 10%%\rjq 60%%'`, want: []string{"jq 60%"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			ctx := WithLineSink(context.Background(), func(line string) { got = append(got, line) })

			_, err := DefaultRunner().Run(ctx, "sh", "-c", tt.script)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRealRunner_DeliversLinesWhileTheProcessIsStillRunning(t *testing.T) {
	release := filepath.Join(t.TempDir(), "release")
	lines := make(chan string, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = WithLineSink(ctx, func(line string) { lines <- line })
	done := make(chan error, 1)

	go func() {
		_, err := DefaultRunner().Run(ctx, "sh", "-c", `echo ready; while [ ! -e "$1" ]; do sleep 0.01; done; echo released`, "sh", release)
		done <- err
	}()

	select {
	case line := <-lines:
		assert.Equal(t, "ready", line)
	case err := <-done:
		t.Fatalf("process exited before any line arrived: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("no line arrived while the process was running")
	}
	require.NoError(t, os.WriteFile(release, nil, 0o644))
	require.NoError(t, <-done)
	assert.Equal(t, "released", <-lines)
}

func TestRealRunner_StillReturnsTheFullOutput(t *testing.T) {
	var got []string
	ctx := WithLineSink(context.Background(), func(line string) { got = append(got, line) })

	out, err := DefaultRunner().Run(ctx, "sh", "-c", "echo a; echo b >&2; exit 3")

	require.Error(t, err)
	assert.Equal(t, "a\nb\n", string(out))
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestRealRunner_WithoutASinkOnlyReturnsOutput(t *testing.T) {
	out, err := DefaultRunner().Run(context.Background(), "sh", "-c", "echo quiet")

	require.NoError(t, err)
	assert.Equal(t, "quiet\n", string(out))
}

func TestRealRunner_CancelStopsTheWholeProcessGroup(t *testing.T) {
	tests := []struct {
		name   string
		script string
		within time.Duration
	}{
		{name: "children that exit on SIGTERM", script: `sleep 1000 & echo $!; wait`, within: time.Second},
		{name: "children that ignore SIGTERM", script: `trap '' TERM; sleep 1000 & echo $!; wait`, within: killGrace + time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pids := make(chan string, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = WithLineSink(ctx, func(line string) { pids <- line })
			done := make(chan error, 1)
			go func() {
				_, err := DefaultRunner().Run(ctx, "sh", "-c", tt.script)
				done <- err
			}()
			child := readPID(t, pids)
			t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })

			cancel()

			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(tt.within):
				t.Fatal("Run kept waiting after the context was cancelled")
			}
			assert.Eventually(t, func() bool {
				return errors.Is(syscall.Kill(child, 0), syscall.ESRCH)
			}, 2*time.Second, 10*time.Millisecond, "the child outlived the cancelled command")
		})
	}
}

func TestRealRunner_StopsChildrenLeftBehindByAnExitedParent(t *testing.T) {
	tests := []struct {
		name   string
		cancel bool
		within time.Duration
	}{
		{name: "cancelled after the parent exited", cancel: true, within: time.Second},
		{name: "never cancelled", within: killGrace + 2*time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pids := make(chan string, 2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = WithLineSink(ctx, func(line string) { pids <- line })
			done := make(chan error, 1)
			go func() {
				_, err := DefaultRunner().Run(ctx, "sh", "-c", `echo $$; sleep 1000 & echo $!; exit 0`)
				done <- err
			}()
			parent, child := readPID(t, pids), readPID(t, pids)
			t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })
			require.Eventually(t, func() bool {
				return errors.Is(syscall.Kill(parent, 0), syscall.ESRCH)
			}, 5*time.Second, 10*time.Millisecond, "the parent never exited")

			if tt.cancel {
				cancel()
			}

			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(tt.within):
				t.Fatal("Run kept waiting on the child holding its output")
			}
			assert.Eventually(t, func() bool {
				return errors.Is(syscall.Kill(child, 0), syscall.ESRCH)
			}, 2*time.Second, 10*time.Millisecond, "the child outlived its command")
		})
	}
}

func TestRealRunner_CommandsHaveNoControllingTerminal(t *testing.T) {
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		_, err := DefaultRunner().Run(ctx, "sh", "-c", `read -r line </dev/tty`)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("a command reading the terminal blocked instead of failing")
	}
}

func readPID(t *testing.T, lines <-chan string) int {
	t.Helper()
	select {
	case line := <-lines:
		pid, err := strconv.Atoi(line)
		require.NoError(t, err)
		return pid
	case <-time.After(5 * time.Second):
		t.Fatal("the command never printed its pid")
		return 0
	}
}
