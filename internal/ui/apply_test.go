package ui

import (
	"booster/internal/model"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	toggleOutput = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}
	enter        = tea.KeyMsg{Type: tea.KeyEnter}
	quit         = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
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

func applyingSession(p model.Plan, width, height int) (session, *time.Time) {
	clock := t0
	st := newStyles(lipgloss.NewRenderer(io.Discard))
	m := session{now: func() time.Time { return clock }, cancel: func() {}, apply: newApplyScreen(st, p, "~/.local/state/booster/logs/apply-20261108-140200-1.log")}
	m.apply.resize(width, height)
	return m, &clock
}

func feed(m session, clock *time.Time, steps []at) session {
	for _, s := range steps {
		*clock = t0.Add(time.Duration(s.sec) * time.Second)
		next, _ := m.Update(s.msg)
		m = next.(session)
	}
	return m
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
		{name: "scrolls to keep the running step visible", width: 80, height: 9, steps: seq(started, packagesOutput, afterPackages, output(203, 3, "linking ~/.zshrc"))},
		{name: "scrolls to keep the failed step visible", width: 80, height: 8, steps: seq(started, packagesOutput, afterPackages, stopped)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, clock := applyingSession(applyPlan(), tt.width, tt.height)

			m = feed(m, clock, tt.steps)

			golden.RequireEqual(t, []byte(m.View()))
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
		m, clock := applyingSession(p, width, height)
		m = feed(m, clock, steps)

		rows := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
		assert.LessOrEqual(t, len(rows), height)
		for _, r := range rows {
			assert.LessOrEqual(t, ansi.StringWidth(r), width, "row %q", r)
		}
		assert.Contains(t, m.View(), "…")
	}
}

type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func scriptedRun(rec *recorder) func(context.Context, func(model.Event)) {
	return func(_ context.Context, emit func(model.Event)) {
		rec.add("run")
		emit(model.RunStart{Total: 2})
		emit(model.TaskStart{Index: 0, Name: "dirs"})
		emit(model.TaskOutput{Index: 0, Line: "mkdir ~/.config"})
		emit(done(0, model.StatusChanged, time.Second))
		emit(model.TaskStart{Index: 1, Name: "links"})
		emit(done(1, model.StatusChanged, time.Second))
		emit(model.RunDone{Summary: model.Summary{Total: 2, Changed: 2, Duration: 2 * time.Second}})
	}
}

func cancellableRun(rec *recorder) func(context.Context, func(model.Event)) {
	return func(ctx context.Context, emit func(model.Event)) {
		emit(model.RunStart{Total: 2})
		emit(model.TaskStart{Index: 0, Name: "dirs"})
		<-ctx.Done()
		rec.add("cancelled")
		emit(model.TaskDone{Index: 0, Result: model.Result{Status: model.StatusFailed, Err: ctx.Err()}})
		emit(model.TaskDone{Index: 1, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}})
		emit(model.RunDone{Summary: model.Summary{Total: 2, Failed: 1, Skipped: 1}})
	}
}

func twoSteps() model.Plan {
	return model.Plan{Steps: []model.PlannedStep{
		{Name: "dirs", Change: model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1, Items: items("~/.config")}},
		{Name: "links", Change: model.Change{Kind: model.ChangeCreate, Verb: "link", Total: 1, Items: items("~/.zshrc")}},
	}}
}

type sessionRun struct {
	err      error
	rec      *recorder
	decision Decision
	final    session
}

func runSession(t *testing.T, job func(*recorder) Job, keys ...tea.KeyMsg) sessionRun {
	t.Helper()

	rec := &recorder{}
	start := func(d Decision) (Job, error) {
		rec.add("start")
		return job(rec), nil
	}
	m := newSession(t.Context(), lipgloss.NewRenderer(io.Discard), header, twoSteps(), ModeApply, start, time.Now)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))
	for _, k := range keys {
		if k.Type == tea.KeyCtrlC {
			teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("ctrl+c")) })
		}
		tm.Send(k)
	}
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(session)
	return sessionRun{rec: rec, decision: final.review.decision, err: final.err, final: final}
}

func TestSession(t *testing.T) {
	t.Run("enter switches to the apply screen and runs the job", func(t *testing.T) {
		r := runSession(t, func(rec *recorder) Job {
			return Job{Plan: twoSteps(), Log: "~/apply.log", Run: scriptedRun(rec)}
		}, enter)

		require.NoError(t, r.err)
		assert.Equal(t, Decision{Apply: true}, r.decision)
		assert.Equal(t, []string{"start", "run"}, r.rec.list())
		view := r.final.View()
		assert.Contains(t, view, "applied 2 changes")
		assert.Contains(t, view, "2 changed · 0 up to date")
		assert.Contains(t, view, "~/apply.log")
	})

	t.Run("sudo runs before the job", func(t *testing.T) {
		r := runSession(t, func(rec *recorder) Job {
			return Job{Plan: twoSteps(), Run: scriptedRun(rec), Sudo: func() error {
				rec.add("sudo")
				return nil
			}}
		}, enter)

		require.NoError(t, r.err)
		assert.Equal(t, []string{"start", "sudo", "run"}, r.rec.list())
	})

	t.Run("refused sudo runs nothing", func(t *testing.T) {
		refused := errors.New("wrong password")
		r := runSession(t, func(rec *recorder) Job {
			return Job{Plan: twoSteps(), Run: scriptedRun(rec), Sudo: func() error { return refused }}
		}, enter)

		require.ErrorIs(t, r.err, refused)
		assert.Equal(t, []string{"start"}, r.rec.list())
	})

	t.Run("ctrl+c cancels the running step and waits for the run to end", func(t *testing.T) {
		r := runSession(t, func(rec *recorder) Job {
			return Job{Plan: twoSteps(), Run: cancellableRun(rec)}
		}, enter, ctrlC)

		require.NoError(t, r.err)
		assert.Equal(t, []string{"start", "cancelled"}, r.rec.list())
		view := r.final.View()
		assert.Contains(t, view, "stopped at dirs")
		assert.Contains(t, view, "1 not run")
		assert.NotContains(t, view, "●")
	})

	t.Run("quitting the review starts nothing", func(t *testing.T) {
		r := runSession(t, func(rec *recorder) Job {
			return Job{Plan: twoSteps(), Run: scriptedRun(rec)}
		}, quit)

		assert.Equal(t, Decision{}, r.decision)
		assert.Empty(t, r.rec.list())
		assert.NotContains(t, r.final.View(), "applying")
	})
}
