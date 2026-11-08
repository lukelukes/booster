package main

import (
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type applyFixture struct {
	cli    *CLI
	newDir string
	link   string
	source string
}

func newApplyFixture(t *testing.T) applyFixture {
	t.Helper()

	dir := t.TempDir()
	f := applyFixture{
		source: filepath.Join(dir, "dotfiles", "zshrc"),
		newDir: filepath.Join(dir, "home", ".config"),
		link:   filepath.Join(dir, "home", ".zshrc"),
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(f.source), 0o755))
	require.NoError(t, os.WriteFile(f.source, []byte("export X=1"), 0o644))

	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `version: "1"
tasks:
  - name: dirs
    action: dir.create
    args: [%q]
  - name: links
    action: symlink.create
    args:
      - {source: %q, target: %q}
`, f.newDir, f.source, f.link), 0o644))
	f.cli = &CLI{Config: configPath}

	return f
}

type fakeTerminal struct {
	out       strings.Builder
	sudoCalls int
}

func (ft *fakeTerminal) terminal(input string, interactive bool) terminal {
	return terminal{
		in:          strings.NewReader(input),
		out:         &ft.out,
		interactive: interactive,
		sudo: func() error {
			ft.sudoCalls++
			return nil
		},
	}
}

func TestApplyCmd(t *testing.T) {
	tests := []struct {
		name        string
		yes         bool
		interactive bool
		input       string
		wantApplied bool
		wantErr     error
		wantOut     []string
	}{
		{
			name:        "--yes applies without asking",
			yes:         true,
			wantApplied: true,
			wantOut:     []string{"+ dirs  changed", "+ links  changed", "2 changed · 0 up to date"},
		},
		{
			name:        "confirming at the prompt applies",
			interactive: true,
			input:       "y\n",
			wantApplied: true,
			wantOut:     []string{"apply these changes? [y/N]", "2 changed · 0 up to date"},
		},
		{
			name:        "anything but yes at the prompt cancels",
			interactive: true,
			input:       "\n",
			wantOut:     []string{"apply these changes? [y/N]", "cancelled, nothing changed"},
		},
		{
			name:    "no terminal and no --yes refuses",
			wantErr: errNotConfirmed,
			wantOut: []string{"+ dirs", "2 to change"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newApplyFixture(t)
			var ft fakeTerminal

			err := (&ApplyCmd{Yes: tt.yes}).run(f.cli, ft.terminal(tt.input, tt.interactive))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			for _, want := range tt.wantOut {
				assert.Contains(t, ft.out.String(), want)
			}
			if tt.wantApplied {
				assert.DirExists(t, f.newDir)
				target, err := os.Readlink(f.link)
				require.NoError(t, err)
				assert.Equal(t, f.source, target)
			} else {
				assert.NoDirExists(t, f.newDir)
				assert.NoFileExists(t, f.link)
			}
			assert.Zero(t, ft.sudoCalls)
		})
	}
}

func TestApplyCmd_UpToDateDoesNotAsk(t *testing.T) {
	f := newApplyFixture(t)
	require.NoError(t, (&ApplyCmd{Yes: true}).run(f.cli, new(fakeTerminal).terminal("", false)))
	var ft fakeTerminal

	err := (&ApplyCmd{}).run(f.cli, ft.terminal("", false))

	require.NoError(t, err)
	assert.Contains(t, ft.out.String(), "0 to change · 2 up to date")
	assert.NotContains(t, ft.out.String(), "apply these changes")
	assert.NotContains(t, ft.out.String(), "applying")
}

func TestApplyCmd_FailedStepStopsTheRun(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f := newApplyFixture(t)
	home := filepath.Dir(f.newDir)
	require.NoError(t, os.MkdirAll(home, 0o755))
	require.NoError(t, os.Chmod(home, 0o500))
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })
	var ft fakeTerminal

	err := (&ApplyCmd{Yes: true}).run(f.cli, ft.terminal("", false))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2 steps failed")
	assert.Contains(t, ft.out.String(), "✗ dirs  failed: ")
	assert.Contains(t, ft.out.String(), "permission denied")
	assert.Contains(t, ft.out.String(), "- links  not run")
	assert.NoFileExists(t, f.link)
}

type sudoAction struct {
	sudoCalls *int
	sudoSeen  int
	applied   bool
}

func (a *sudoAction) Name() string    { return "fake" }
func (a *sudoAction) NeedsSudo() bool { return true }

func (a *sudoAction) Check(context.Context) (model.Change, error) {
	return model.Change{}, nil
}

func (a *sudoAction) Apply(context.Context) error {
	a.applied = true
	a.sudoSeen = *a.sudoCalls
	return nil
}

func TestApplyCmd_AsksForSudoOnceBeforeApplying(t *testing.T) {
	var ft fakeTerminal
	needsSudo := model.Change{Kind: model.ChangeCreate, Verb: "install", NeedsSudo: true}
	first, second := &sudoAction{sudoCalls: &ft.sudoCalls}, &sudoAction{sudoCalls: &ft.sudoCalls}
	plan := run.PlanResult{Steps: []run.Planned{
		{PlannedStep: model.PlannedStep{Name: "a", Change: needsSudo}, Action: first},
		{PlannedStep: model.PlannedStep{Name: "b", Change: needsSudo}, Action: second},
	}}

	err := (&ApplyCmd{Yes: true}).applyPlan(plan, ft.terminal("", false))

	require.NoError(t, err)
	assert.Equal(t, 1, ft.sudoCalls)
	assert.Equal(t, 1, first.sudoSeen)
	assert.Equal(t, 1, second.sudoSeen)
}

func TestApplyCmd_SudoRefusedAppliesNothing(t *testing.T) {
	a := &sudoAction{sudoCalls: new(int)}
	plan := run.PlanResult{Steps: []run.Planned{
		{PlannedStep: model.PlannedStep{Name: "a", Change: model.Change{Kind: model.ChangeCreate, NeedsSudo: true}}, Action: a},
	}}
	var out strings.Builder
	term := terminal{out: &out, sudo: func() error { return errors.New("wrong password") }}

	err := (&ApplyCmd{Yes: true}).applyPlan(plan, term)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong password")
	assert.False(t, a.applied)
}
