package ui

import (
	"booster/internal/model"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	toggleOutput = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}
	ctrlC        = tea.KeyMsg{Type: tea.KeyCtrlC}
	t0           = time.Date(2026, 11, 8, 14, 2, 0, 0, time.UTC)
)

func applyPlan() model.Plan {
	return model.Plan{Steps: []model.PlannedStep{
		{Name: "homebrew", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 1, Items: items("homebrew")}},
		{Name: "packages", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 5, Items: items("ripgrep", "fd", "starship")}},
		{Name: "git-config", Change: model.Change{Verb: "set", Total: 2}},
		{Name: "links", Change: model.Change{Kind: model.ChangeUpdate, Verb: "link", Total: 3, Items: items("~/.zshrc")}},
		{Name: "login-shell", Change: model.Change{Kind: model.ChangeUnknown, Verb: "run"}},
	}}
}

type at struct {
	msg tea.Msg
	sec int
}

func ev(sec int, e model.Event) at { return at{msg: eventMsg{e}, sec: sec} }
func key(sec int, k tea.KeyMsg) at { return at{msg: k, sec: sec} }

func done(i int, status model.Status, d time.Duration) model.TaskDone {
	return model.TaskDone{Index: i, Result: model.Result{Status: status, Duration: d}}
}

func output(sec, i int, lines ...string) []at {
	out := make([]at, 0, len(lines))
	for _, l := range lines {
		out = append(out, ev(sec, model.TaskOutput{Index: i, Line: l}))
	}
	return out
}

func seq(parts ...[]at) []at {
	var out []at
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var (
	started = []at{
		ev(0, model.RunStart{Total: 5}),
		ev(0, model.TaskStart{Index: 0, Name: "homebrew"}),
		ev(108, done(0, model.StatusChanged, 108*time.Second)),
		ev(108, model.TaskStart{Index: 1, Name: "packages"}),
	}
	packagesOutput = output(120, 1,
		"==> Fetching ripgrep",
		"==> Pouring ripgrep--14.1.1.arm64_sequoia.bottle.tar.gz",
		"==> Fetching starship",
		"\x1b[34m==>\x1b[0m Downloading https://ghcr.io/v2/homebrew/core/starship/manifests/1.21.1/with/a/path/long/enough/to/be/cut/off",
		"==> Pouring\tstarship--1.21.1.arm64_sequoia.bottle.tar.gz",
	)
	afterPackages = []at{
		ev(200, done(1, model.StatusChanged, 92*time.Second)),
		ev(200, model.TaskStart{Index: 2, Name: "git-config"}),
		ev(200, done(2, model.StatusOK, 0)),
		ev(200, model.TaskStart{Index: 3, Name: "links"}),
	}
	stopped = []at{
		key(205, ctrlC),
		ev(206, model.TaskDone{Index: 3, Result: model.Result{Status: model.StatusFailed, Duration: 6 * time.Second, Err: context.Canceled}}),
		ev(206, model.TaskDone{Index: 4, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}}),
		ev(206, model.RunDone{Summary: model.Summary{Total: 5, Changed: 2, OK: 1, Failed: 1, Skipped: 1, Duration: 206 * time.Second}}),
	}
)

func applyingScreen(p model.Plan, width, height int) (*applyScreen, *time.Time) {
	clock := t0
	a := newApplyScreen(lipgloss.NewRenderer(io.Discard), p, "~/.local/state/booster/logs/apply-20261108-140200-1.log", nil, func() {}, func() time.Time { return clock })
	a.resize(width, height)
	return a, &clock
}

func feed(a *applyScreen, clock *time.Time, steps []at) *applyScreen {
	for _, s := range steps {
		*clock = t0.Add(time.Duration(s.sec) * time.Second)
		a.Update(s.msg)
	}
	return a
}

func TestApply_Frames(t *testing.T) {
	tests := []struct {
		name   string
		steps  []at
		width  int
		height int
	}{
		{name: "before the first step", steps: started[:1]},
		{name: "running step shows its output tail", steps: seq(started, packagesOutput, []at{{msg: tickMsg{}, sec: 124}})},
		{name: "o hides the output tail", steps: seq(started, packagesOutput, []at{key(124, toggleOutput)})},
		{name: "o shows the output tail again", steps: seq(started, packagesOutput, []at{key(124, toggleOutput), key(125, toggleOutput)})},
		{name: "up to date steps stay dim", steps: seq(started, packagesOutput, afterPackages)},
		{name: "ctrl+c waits for the running step", steps: seq(started, packagesOutput, afterPackages, stopped[:1])},
		{name: "stopped run leaves no running step", steps: seq(started, packagesOutput, afterPackages, stopped)},
		{name: "finished", steps: seq(started, afterPackages, []at{
			ev(201, done(3, model.StatusChanged, time.Second)),
			ev(201, model.TaskStart{Index: 4, Name: "login-shell"}),
			ev(3725, done(4, model.StatusChanged, 3524*time.Second)),
			ev(3725, model.RunDone{Summary: model.Summary{Total: 5, Changed: 4, OK: 1, Duration: 3725 * time.Second}}),
		})},
		{name: "stopped between steps counts only the applied changes", steps: seq(started, []at{
			key(110, ctrlC),
			ev(150, done(1, model.StatusChanged, 42*time.Second)),
			ev(150, model.TaskDone{Index: 2, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}}),
			ev(150, model.TaskDone{Index: 3, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}}),
			ev(150, model.TaskDone{Index: 4, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}}),
			ev(150, model.RunDone{Summary: model.Summary{Total: 5, Changed: 2, Skipped: 3, Duration: 150 * time.Second}}),
		})},
		{name: "scrolls to keep the running step visible", width: 80, height: 9, steps: seq(started, packagesOutput, afterPackages, output(203, 3, "linking ~/.zshrc"))},
		{name: "scrolls to keep the failed step visible", width: 80, height: 8, steps: seq(started, packagesOutput, afterPackages, stopped)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, clock := applyingScreen(applyPlan(), tt.width, tt.height)

			a = feed(a, clock, tt.steps)

			golden.RequireEqual(t, []byte(a.View()))
		})
	}
}

func TestApply_RowsFitANarrowTerminal(t *testing.T) {
	const width, height = 40, 10
	p := applyPlan()
	p.Steps[1].Name = "packages-from-a-very-long-brewfile-name"
	stages := [][]at{
		started,
		seq(started, packagesOutput),
		seq(started, packagesOutput, afterPackages),
		seq(started, packagesOutput, afterPackages, stopped),
	}

	for _, steps := range stages {
		a, clock := applyingScreen(p, width, height)
		a = feed(a, clock, steps)

		rows := strings.Split(strings.TrimSuffix(a.View(), "\n"), "\n")
		assert.LessOrEqual(t, len(rows), height)
		for _, r := range rows {
			assert.LessOrEqual(t, ansi.StringWidth(r), width, "row %q", r)
		}
		assert.Contains(t, a.View(), "…")
	}
}

func twoSteps() model.Plan {
	return model.Plan{Steps: []model.PlannedStep{
		{Name: "dirs", Change: model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1, Items: items("~/.config")}},
		{Name: "links", Change: model.Change{Kind: model.ChangeCreate, Verb: "link", Total: 1, Items: items("~/.zshrc")}},
	}}
}

type producer struct {
	ctx    context.Context
	stop   context.CancelFunc
	events chan model.Event
	done   chan struct{}
}

func produce(t *testing.T, run func(ctx context.Context, emit func(model.Event))) producer {
	t.Helper()
	ctx, stop := context.WithCancel(t.Context())
	p := producer{ctx: ctx, stop: stop, events: make(chan model.Event), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		defer close(p.events)
		run(ctx, func(e model.Event) { p.events <- e })
	}()
	t.Cleanup(func() {
		stop()
		for range p.events {
		}
		<-p.done
	})
	return p
}

func finishedRun(_ context.Context, emit func(model.Event)) {
	emit(model.RunStart{Total: 2})
	emit(model.TaskStart{Index: 0, Name: "dirs"})
	emit(model.TaskOutput{Index: 0, Line: "mkdir ~/.config"})
	emit(done(0, model.StatusChanged, time.Second))
	emit(model.TaskStart{Index: 1, Name: "links"})
	emit(done(1, model.StatusChanged, time.Second))
	emit(model.RunDone{Summary: model.Summary{Total: 2, Changed: 2, Duration: 2 * time.Second}})
}

func cancellableRun(ctx context.Context, emit func(model.Event)) {
	emit(model.RunStart{Total: 2})
	emit(model.TaskStart{Index: 0, Name: "dirs"})
	<-ctx.Done()
	emit(model.TaskDone{Index: 0, Result: model.Result{Status: model.StatusFailed, Err: ctx.Err()}})
	emit(model.TaskDone{Index: 1, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}})
	emit(model.RunDone{Summary: model.Summary{Total: 2, Failed: 1, Skipped: 1}})
}

type failingInput struct{ err error }

func (r failingInput) Read([]byte) (int, error) { return 0, r.err }

func TestApply(t *testing.T) {
	t.Run("renders events until the producer closes the channel", func(t *testing.T) {
		p := produce(t, finishedRun)
		var out bytes.Buffer

		err := Apply(strings.NewReader(""), &out, twoSteps(), "~/apply.log", p.events, p.stop)

		require.NoError(t, err)
		assert.Contains(t, out.String(), "applied 2 changes")
		assert.Contains(t, out.String(), "2 changed · 0 up to date")
		assert.Contains(t, out.String(), "~/apply.log")
		assert.Error(t, p.ctx.Err(), "the run's context is released on exit")
	})

	t.Run("ctrl+c stops the run and waits for its last event", func(t *testing.T) {
		p := produce(t, cancellableRun)
		var out bytes.Buffer

		err := Apply(strings.NewReader("\x03"), &out, twoSteps(), "~/apply.log", p.events, p.stop)

		require.NoError(t, err)
		<-p.done
		assert.Contains(t, out.String(), "stopped at dirs")
		assert.Contains(t, out.String(), "1 not run")
	})

	t.Run("an input error ends the program and stops the run", func(t *testing.T) {
		broken := errors.New("tty gone")
		p := produce(t, cancellableRun)

		err := Apply(failingInput{broken}, io.Discard, twoSteps(), "~/apply.log", p.events, p.stop)

		require.ErrorIs(t, err, broken)
		assert.Error(t, p.ctx.Err())
	})
}
