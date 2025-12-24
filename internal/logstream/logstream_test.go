package logstream

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelWriter_CompleteLine(t *testing.T) {
	w, ch := NewChannelWriter(10)

	n, err := w.Write([]byte("hello world\n"))
	require.NoError(t, err)
	assert.Equal(t, 12, n)

	line := <-ch
	assert.Equal(t, "hello world", line)
}

func TestChannelWriter_MultipleLines(t *testing.T) {
	w, ch := NewChannelWriter(10)

	_, err := w.Write([]byte("line1\nline2\nline3\n"))
	require.NoError(t, err)

	assert.Equal(t, "line1", <-ch)
	assert.Equal(t, "line2", <-ch)
	assert.Equal(t, "line3", <-ch)
}

func TestChannelWriter_PartialLine(t *testing.T) {
	w, ch := NewChannelWriter(10)

	_, err := w.Write([]byte("partial"))
	require.NoError(t, err)

	select {
	case line := <-ch:
		t.Fatalf("expected no line, got: %s", line)
	default:
	}

	_, err = w.Write([]byte(" complete\n"))
	require.NoError(t, err)

	line := <-ch
	assert.Equal(t, "partial complete", line)
}

func TestChannelWriter_Close_FlushesRemaining(t *testing.T) {
	w, ch := NewChannelWriter(10)

	_, err := w.Write([]byte("no newline"))
	require.NoError(t, err)

	err = w.Close()
	require.NoError(t, err)

	line := <-ch
	assert.Equal(t, "no newline", line)

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed")
}

func TestChannelWriter_Close_Idempotent(t *testing.T) {
	w, _ := NewChannelWriter(10)

	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
}

func TestChannelWriter_WriteAfterClose(t *testing.T) {
	w, _ := NewChannelWriter(10)
	w.Close()

	_, err := w.Write([]byte("test\n"))
	assert.Error(t, err)
}

func TestContextWriter(t *testing.T) {
	ctx := context.Background()

	assert.Nil(t, Writer(ctx))

	var buf bytes.Buffer
	ctx = WithWriter(ctx, &buf)
	w := Writer(ctx)
	require.NotNil(t, w)

	w.Write([]byte("test"))
	assert.Equal(t, "test", buf.String())
}

func TestMultiWriter(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		w := MultiWriter(nil, nil)
		assert.Nil(t, w)
	})

	t.Run("stream nil", func(t *testing.T) {
		var buf bytes.Buffer
		w := MultiWriter(nil, &buf)
		w.Write([]byte("test"))
		assert.Equal(t, "test", buf.String())
	})

	t.Run("buffer nil", func(t *testing.T) {
		cw, ch := NewChannelWriter(10)
		w := MultiWriter(cw, nil)
		w.Write([]byte("test\n"))
		assert.Equal(t, "test", <-ch)
	})

	t.Run("both present", func(t *testing.T) {
		var buf bytes.Buffer
		cw, ch := NewChannelWriter(10)

		w := MultiWriter(cw, &buf)
		w.Write([]byte("test\n"))

		assert.Equal(t, "test\n", buf.String())
		assert.Equal(t, "test", <-ch)
	})
}

func TestChannelWriter_ConcurrentWrites(t *testing.T) {
	w, ch := NewChannelWriter(1000)
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := w.Write(fmt.Appendf(nil, "line %d\n", n))
			assert.NoError(t, err)
		}(i)
	}

	go func() {
		wg.Wait()
		w.Close()
	}()

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 100, count, "All lines should be received")
}

func TestLog(t *testing.T) {
	t.Run("writes to context writer", func(t *testing.T) {
		cw, ch := NewChannelWriter(10)
		ctx := WithWriter(context.Background(), cw)

		Log(ctx, "hello world")

		line := <-ch
		assert.Equal(t, "hello world", line)
	})

	t.Run("adds newline if missing", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := WithWriter(context.Background(), &buf)

		Log(ctx, "no newline")

		assert.Equal(t, "no newline\n", buf.String())
	})

	t.Run("does not double newline", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := WithWriter(context.Background(), &buf)

		Log(ctx, "has newline\n")

		assert.Equal(t, "has newline\n", buf.String())
	})

	t.Run("silent when no writer", func(t *testing.T) {
		ctx := context.Background()

		Log(ctx, "ignored")
	})

	t.Run("handles empty message", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := WithWriter(context.Background(), &buf)

		Log(ctx, "")

		assert.Equal(t, "\n", buf.String())
	})
}
