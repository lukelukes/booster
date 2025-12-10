// Package logstream provides streaming output support for command execution.
// It allows commands to emit output line-by-line through channels for real-time display.
package logstream

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type ctxKey struct{}

// Writer returns an io.Writer from context for streaming output.
// Returns nil if no writer is set (caller should handle gracefully).
func Writer(ctx context.Context) io.Writer {
	w, _ := ctx.Value(ctxKey{}).(io.Writer)
	return w
}

// WithWriter returns a context with the given writer attached.
func WithWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, ctxKey{}, w)
}

// ChannelWriter sends written data to a channel line by line.
// It buffers partial lines until a newline is received.
// Thread-safe for concurrent writes.
type ChannelWriter struct {
	ch     chan string
	buf    []byte
	mu     sync.Mutex
	closed bool
}

// NewChannelWriter creates a ChannelWriter and returns it along with the read channel.
// The channel is buffered to prevent blocking writers when readers are slow.
func NewChannelWriter(bufSize int) (*ChannelWriter, <-chan string) {
	if bufSize <= 0 {
		bufSize = 100
	}
	ch := make(chan string, bufSize)
	return &ChannelWriter{ch: ch}, ch
}

// Write implements io.Writer. Buffers data and sends complete lines to the channel.
func (w *ChannelWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, io.ErrClosedPipe
	}

	w.buf = append(w.buf, p...)

	// Emit complete lines
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]

		// Non-blocking send - drop if channel is full
		select {
		case w.ch <- line:
		default:
		}
	}

	return len(p), nil
}

// Close flushes any remaining buffered data and closes the channel.
func (w *ChannelWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Flush remaining buffer as final line
	if len(w.buf) > 0 {
		select {
		case w.ch <- string(w.buf):
		default:
		}
		w.buf = nil
	}

	close(w.ch)
	return nil
}

// MultiWriter creates a writer that duplicates writes to both the streaming
// channel and a buffer for final result capture.
func MultiWriter(stream, buffer io.Writer) io.Writer {
	if stream == nil {
		return buffer
	}
	if buffer == nil {
		return stream
	}
	return io.MultiWriter(stream, buffer)
}

// Log writes a message to the context's stream writer if present.
// This is a convenience function for tasks that want to emit log messages.
// If no writer is present in the context, the message is silently dropped.
func Log(ctx context.Context, msg string) {
	if w := Writer(ctx); w != nil {
		// Ensure message ends with newline for proper line-by-line streaming
		if len(msg) == 0 || msg[len(msg)-1] != '\n' {
			msg += "\n"
		}
		_, _ = w.Write([]byte(msg))
	}
}
