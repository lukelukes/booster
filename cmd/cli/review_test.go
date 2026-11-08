package main

import (
	"booster/internal/model"
	"booster/internal/run"
	"booster/internal/ui"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReview struct {
	job      ui.Job
	screen   strings.Builder
	decision ui.Decision
	modes    []ui.Mode
	steps    []string
	started  bool
}

func (r *fakeReview) review(ctx context.Context, p model.Plan, mode ui.Mode, start ui.Start) (ui.Decision, error) {
	r.modes = append(r.modes, mode)
	for _, s := range p.Steps {
		r.steps = append(r.steps, s.Name)
	}
	if !r.decision.Apply {
		return r.decision, nil
	}

	job, err := start(r.decision)
	if err != nil {
		return r.decision, err
	}
	r.job, r.started = job, true
	if job.Sudo != nil {
		if err := job.Sudo(); err != nil {
			return r.decision, err
		}
	}
	job.Run(ctx, ui.Progress(&r.screen, job.Plan))
	return r.decision, nil
}

func (ft *fakeTerminal) withReview(t *testing.T, r *fakeReview) terminal {
	t.Helper()
	term := ft.terminal(t, "", true)
	term.review = r.review
	return term
}

func TestApplyCmd_Review(t *testing.T) {
	tests := []struct {
		name       string
		decision   ui.Decision
		wantDir    bool
		wantLink   bool
		wantOut    string
		notWantOut string
	}{
		{name: "enter applies every step", decision: ui.Decision{Apply: true}, wantDir: true, wantLink: true, wantOut: "2 changed · 0 up to date"},
		{name: "skipped steps are not applied", decision: ui.Decision{Apply: true, Skip: []int{0}}, wantLink: true, wantOut: "1 changed · 0 up to date", notWantOut: "dirs"},
		{name: "quitting applies nothing", decision: ui.Decision{}, wantOut: "cancelled, nothing changed", notWantOut: "applying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newApplyFixture(t)
			var ft fakeTerminal
			r := &fakeReview{decision: tt.decision}

			err := (&ApplyCmd{}).run(t.Context(), f.cli, ft.withReview(t, r))

			require.NoError(t, err)
			assert.Equal(t, []ui.Mode{ui.ModeApply}, r.modes)
			assert.Equal(t, []string{"dirs", "links"}, r.steps)
			assert.Contains(t, ft.out.String()+r.screen.String(), tt.wantOut)
			if tt.notWantOut != "" {
				assert.NotContains(t, r.screen.String(), tt.notWantOut)
			}
			assert.Equal(t, tt.wantDir, dirExists(f.newDir))
			_, linkErr := os.Lstat(f.link)
			assert.Equal(t, tt.wantLink, linkErr == nil)
			assert.NotContains(t, ft.out.String(), "apply these changes")
		})
	}
}

func TestApplyCmd_ReviewIsSkippedWithYesOrNothingToDo(t *testing.T) {
	f := newApplyFixture(t)
	r := &fakeReview{}

	require.NoError(t, (&ApplyCmd{Yes: true}).run(t.Context(), f.cli, new(fakeTerminal).withReview(t, r)))
	require.NoError(t, (&ApplyCmd{}).run(t.Context(), f.cli, new(fakeTerminal).withReview(t, r)))

	assert.Empty(t, r.modes)
}

func TestPlanCmd_Review(t *testing.T) {
	t.Run("quitting leaves the drift and exits 2", func(t *testing.T) {
		f := newApplyFixture(t)
		r := &fakeReview{}

		err := (&PlanCmd{}).run(t.Context(), f.cli, new(fakeTerminal).withReview(t, r))

		require.ErrorIs(t, err, errChangesPending)
		assert.Equal(t, []ui.Mode{ui.ModeDrift}, r.modes)
		assert.NoDirExists(t, f.newDir)
	})

	t.Run("enter applies the fixes", func(t *testing.T) {
		f := newApplyFixture(t)
		var ft fakeTerminal
		r := &fakeReview{decision: ui.Decision{Apply: true, Skip: []int{1}}}

		err := (&PlanCmd{}).run(t.Context(), f.cli, ft.withReview(t, r))

		require.NoError(t, err)
		assert.DirExists(t, f.newDir)
		assert.NoFileExists(t, f.link)
		assert.Contains(t, r.screen.String(), "1 changed · 0 up to date")
	})
}

func TestApplyCmd_ReviewTeesEventsToTheLog(t *testing.T) {
	f := newApplyFixture(t)
	var ft fakeTerminal
	r := &fakeReview{decision: ui.Decision{Apply: true}}

	require.NoError(t, (&ApplyCmd{}).run(t.Context(), f.cli, ft.withReview(t, r)))

	logs, err := filepath.Glob(filepath.Join(ft.logDir, "*.log"))
	require.NoError(t, err)
	require.Len(t, logs, 1)
	content, err := os.ReadFile(logs[0])
	require.NoError(t, err)
	assert.Contains(t, string(content), "2 to change")
	assert.True(t, strings.HasSuffix(string(content), r.screen.String()), "log ends with the events the screen got")
	assert.Equal(t, logs[0], r.job.Log)
	assert.NotContains(t, ft.out.String(), "log:")
}

func TestApplyCmd_ReviewAsksForSudoThroughTheScreen(t *testing.T) {
	tests := []struct {
		name      string
		sudoErr   error
		wantErr   string
		wantLogs  int
		wantOrder []string
	}{
		{name: "granted", wantLogs: 1, wantOrder: []string{"sudo", "apply a"}},
		{name: "refused", sudoErr: errors.New("wrong password"), wantErr: "sudo required: wrong password", wantOrder: []string{"sudo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft fakeTerminal
			plan := run.PlanResult{Steps: []run.Planned{
				ft.planned("a", model.Change{Kind: model.ChangeCreate, NeedsSudo: true}),
			}}
			r := &fakeReview{decision: ui.Decision{Apply: true}}
			term := ft.withReview(t, r)
			term.sudo = func() error {
				ft.events = append(ft.events, "sudo")
				return tt.sudoErr
			}

			err := (&ApplyCmd{}).applyPlan(t.Context(), plan, term)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantOrder, ft.events)
			logs, err := filepath.Glob(filepath.Join(ft.logDir, "*.log"))
			require.NoError(t, err)
			assert.Len(t, logs, tt.wantLogs)
		})
	}
}

func TestApplyCmd_ReviewReportsLogFailureAfterTheRunFinishes(t *testing.T) {
	diskFull := errors.New("no space left on device")
	tests := []struct {
		name string
		log  brokenLog
	}{
		{name: "write fails mid-run", log: brokenLog{writeErr: diskFull}},
		{name: "close fails", log: brokenLog{closeErr: diskFull}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := fakeTerminal{log: &tt.log}
			change := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1}
			plan := run.PlanResult{Steps: []run.Planned{ft.planned("a", change), ft.planned("b", change)}}
			r := &fakeReview{decision: ui.Decision{Apply: true}}

			err := (&ApplyCmd{}).applyPlan(t.Context(), plan, ft.withReview(t, r))

			require.ErrorIs(t, err, diskFull)
			assert.Contains(t, err.Error(), "broken.log")
			assert.Equal(t, []string{"apply a", "apply b"}, ft.events)
			assert.Contains(t, r.screen.String(), "2 changed · 0 up to date")
			assert.Contains(t, tt.log.written.String(), "2 to change")
			assert.NotContains(t, ft.out.String(), "cancelled")
		})
	}
}

func TestApplyCmd_ReviewFailedStepExits1(t *testing.T) {
	var ft fakeTerminal
	plan := run.PlanResult{Steps: []run.Planned{
		{PlannedStep: model.PlannedStep{Name: "a", Change: model.Change{Kind: model.ChangeCreate}}, Action: failingAction{}},
	}}
	r := &fakeReview{decision: ui.Decision{Apply: true}}

	err := (&ApplyCmd{}).applyPlan(t.Context(), plan, ft.withReview(t, r))

	require.EqualError(t, err, "apply: 1 of 1 steps failed")
	assert.NotContains(t, ft.out.String(), "cancelled")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestUseTUI(t *testing.T) {
	tests := []struct {
		name      string
		stdinTTY  bool
		stdoutTTY bool
		noColor   string
		want      bool
	}{
		{name: "both terminals", stdinTTY: true, stdoutTTY: true, want: true},
		{name: "NO_COLOR set", stdinTTY: true, stdoutTTY: true, noColor: "1"},
		{name: "stdin piped", stdoutTTY: true},
		{name: "stdout piped", stdinTTY: true},
		{name: "neither terminal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, useTUI(tt.stdinTTY, tt.stdoutTTY, tt.noColor))
		})
	}
}

type failingAction struct{}

func (failingAction) Name() string { return "failing" }

func (failingAction) Check(context.Context) (model.Change, error) {
	return model.Change{}, nil
}

func (failingAction) Apply(context.Context) error { return errors.New("boom") }
