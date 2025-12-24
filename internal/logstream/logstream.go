package logstream

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type ctxKey struct{}

func Writer(ctx context.Context) io.Writer {
	w, _ := ctx.Value(ctxKey{}).(io.Writer)
	return w
}

func WithWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, ctxKey{}, w)
}

type ChannelWriter struct {
	ch     chan string
	buf    []byte
	mu     sync.Mutex
	closed bool
}

func NewChannelWriter(bufSize int) (*ChannelWriter, <-chan string) {
	if bufSize <= 0 {
		bufSize = 100
	}
	ch := make(chan string, bufSize)
	return &ChannelWriter{ch: ch}, ch
}

func (w *ChannelWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, io.ErrClosedPipe
	}

	w.buf = append(w.buf, p...)

	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]

		select {
		case w.ch <- line:
		default:
		}
	}

	return len(p), nil
}

func (w *ChannelWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

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

func MultiWriter(stream, buffer io.Writer) io.Writer {
	if stream == nil {
		return buffer
	}
	if buffer == nil {
		return stream
	}
	return io.MultiWriter(stream, buffer)
}

func Log(ctx context.Context, msg string) {
	if w := Writer(ctx); w != nil {

		if len(msg) == 0 || msg[len(msg)-1] != '\n' {
			msg += "\n"
		}
		_, _ = w.Write([]byte(msg))
	}
}
