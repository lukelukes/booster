package run

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sudoRunner struct {
	calls chan []string
	hang  bool
}

func newSudoRunner() *sudoRunner {
	return &sudoRunner{calls: make(chan []string, 10)}
}

func (r *sudoRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls <- append([]string{name}, args...)
	if r.hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, nil
}

func (r *sudoRunner) LookPath(name string) (string, error) {
	return name, nil
}

func (r *sudoRunner) next(t *testing.T) []string {
	t.Helper()
	select {
	case call := <-r.calls:
		return call
	case <-time.After(time.Second):
		require.FailNow(t, "sudo was not refreshed")
		return nil
	}
}

func TestKeepSudoAlive_RefreshesOnEveryTick(t *testing.T) {
	runner := newSudoRunner()
	ticks := make(chan time.Time)
	stop := KeepSudoAlive(t.Context(), runner, ticks)
	defer stop()

	for range 3 {
		ticks <- time.Now()
		assert.Equal(t, []string{"sudo", "-n", "-v"}, runner.next(t))
	}
}

func TestKeepSudoAlive_StopsRefreshing(t *testing.T) {
	tests := []struct {
		name string
		end  func(stop func(), cancel context.CancelFunc)
	}{
		{name: "stop", end: func(stop func(), _ context.CancelFunc) { stop() }},
		{name: "parent cancelled", end: func(stop func(), cancel context.CancelFunc) { cancel(); stop() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newSudoRunner()
			ticks := make(chan time.Time, 1)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stop := KeepSudoAlive(ctx, runner, ticks)

			tt.end(stop, cancel)
			ticks <- time.Now()

			select {
			case call := <-runner.calls:
				assert.Fail(t, "refreshed after stopping", call)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}

func TestKeepSudoAlive_StopIsSafeToCallTwice(t *testing.T) {
	stop := KeepSudoAlive(t.Context(), newSudoRunner(), make(chan time.Time))

	stop()
	stop()
}

func TestKeepSudoAlive_StopCancelsARefreshInFlight(t *testing.T) {
	runner := newSudoRunner()
	runner.hang = true
	ticks := make(chan time.Time)
	stop := KeepSudoAlive(t.Context(), runner, ticks)
	ticks <- time.Now()
	runner.next(t)

	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		assert.Fail(t, "stop waited on a hung sudo")
	}
}
