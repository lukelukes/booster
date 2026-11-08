package main

import (
	"booster/internal/model"
	"booster/internal/ui"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReview struct {
	decision ui.Decision
	modes    []ui.Mode
	steps    []string
}

func (r *fakeReview) review(p model.Plan, mode ui.Mode) (ui.Decision, error) {
	r.modes = append(r.modes, mode)
	for _, s := range p.Steps {
		r.steps = append(r.steps, s.Name)
	}
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
			assert.Contains(t, ft.out.String(), tt.wantOut)
			if tt.notWantOut != "" {
				assert.NotContains(t, ft.out.String(), tt.notWantOut)
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
		assert.Contains(t, ft.out.String(), "1 changed · 0 up to date")
	})
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
