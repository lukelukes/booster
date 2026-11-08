package run

import (
	"booster/internal/cmdexec"
	"context"
	"sync"
	"time"
)

const SudoRefreshInterval = time.Minute

func KeepSudoAlive(ctx context.Context, runner cmdexec.Runner, ticks <-chan time.Time) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticks:
				_, _ = runner.Run(ctx, "sudo", "-n", "-v")
			}
		}
	})
	return func() {
		cancel()
		wg.Wait()
	}
}
