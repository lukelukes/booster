package run

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sudoRunner struct {
	calls     chan []string
	cancelled chan struct{}
	hang      bool
}

func newSudoRunner() *sudoRunner {
	return &sudoRunner{calls: make(chan []string, 10), cancelled: make(chan struct{}, 10)}
}

func (r *sudoRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls <- append([]string{name}, args...)
	if r.hang {
		<-ctx.Done()
		r.cancelled <- struct{}{}
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
	runner := newSudoRunner()
	ticks := make(chan time.Time, 1)
	stop := KeepSudoAlive(t.Context(), runner, ticks)

	stop()
	ticks <- time.Now()

	select {
	case call := <-runner.calls:
		assert.Fail(t, "refreshed after stopping", call)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestKeepSudoAlive_StopIsSafeToCallTwice(t *testing.T) {
	stop := KeepSudoAlive(t.Context(), newSudoRunner(), make(chan time.Time))

	stop()
	stop()
}

func TestKeepSudoAlive_CancelsARefreshInFlight(t *testing.T) {
	tests := []struct {
		name string
		end  func(stop func(), cancel context.CancelFunc)
	}{
		{name: "stop", end: func(stop func(), _ context.CancelFunc) { go stop() }},
		{name: "parent cancelled", end: func(_ func(), cancel context.CancelFunc) { cancel() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newSudoRunner()
			runner.hang = true
			ticks := make(chan time.Time)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stop := KeepSudoAlive(ctx, runner, ticks)
			t.Cleanup(stop)
			ticks <- time.Now()
			runner.next(t)

			tt.end(stop, cancel)

			select {
			case <-runner.cancelled:
			case <-time.After(time.Second):
				assert.Fail(t, "the hung sudo was not cancelled")
			}
		})
	}
}
