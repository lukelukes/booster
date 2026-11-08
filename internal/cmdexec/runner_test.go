package cmdexec

import (
	"context"
	"os"
	"path/filepath"
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
