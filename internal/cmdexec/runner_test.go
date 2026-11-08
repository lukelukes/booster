package cmdexec

import (
	"context"
	"testing"

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
