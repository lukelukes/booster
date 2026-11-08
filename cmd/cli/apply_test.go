package main

import (
	"booster/internal/cmdexec"
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
	out     strings.Builder
	logDir  string
	events  []string
	sudoErr error
	log     *brokenLog
}

type brokenLog struct {
	ft       *fakeTerminal
	writeErr error
	closeErr error
	written  strings.Builder
}

func (l *brokenLog) Name() string { return "broken.log" }

func (l *brokenLog) Write(p []byte) (int, error) {
	if l.writeErr != nil && len(l.ft.events) > 0 {
		return 0, l.writeErr
	}
	return l.written.Write(p)
}

func (l *brokenLog) Close() error { return l.closeErr }

func (ft *fakeTerminal) terminal(t *testing.T, input string, interactive bool) terminal {
	t.Helper()
	if ft.logDir == "" {
		ft.logDir = t.TempDir()
	}
	return terminal{
		in:  strings.NewReader(input),
		out: &ft.out,
		openLog: func() (logFile, error) {
			if ft.log != nil {
				ft.log.ft = ft
				return ft.log, nil
			}
			return openLog(ft.logDir)
		},
		interactive: interactive,
		sudo: func() error {
			ft.events = append(ft.events, "sudo")
			return ft.sudoErr
		},
	}
}

func (ft *fakeTerminal) planned(name string, change model.Change) run.Planned {
	return run.Planned{
		PlannedStep: model.PlannedStep{Name: name, Change: change},
		Action:      &fakeAction{name: name, events: &ft.events},
	}
}

type fakeAction struct {
	name   string
	events *[]string
}

func (a *fakeAction) Name() string { return a.name }

func (a *fakeAction) Check(context.Context) (model.Change, error) {
	return model.Change{}, nil
}

func (a *fakeAction) Apply(context.Context) error {
	*a.events = append(*a.events, "apply "+a.name)
	return nil
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

			err := (&ApplyCmd{Yes: tt.yes}).run(f.cli, ft.terminal(t, tt.input, tt.interactive))

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
			assert.Empty(t, ft.events)
		})
	}
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

	err := (&ApplyCmd{Yes: true}).run(f.cli, ft.terminal(t, "", false))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2 steps failed")
	assert.Contains(t, ft.out.String(), "✗ dirs  failed: ")
	assert.Contains(t, ft.out.String(), "permission denied")
	assert.Contains(t, ft.out.String(), "- links  not run")
	assert.NoFileExists(t, f.link)
}

func TestApplyCmd_AsksForSudoOnceBeforeApplying(t *testing.T) {
	var ft fakeTerminal
	needsSudo := model.Change{Kind: model.ChangeCreate, Verb: "install", NeedsSudo: true}
	plan := run.PlanResult{Steps: []run.Planned{ft.planned("a", needsSudo), ft.planned("b", needsSudo)}}

	err := (&ApplyCmd{Yes: true}).applyPlan(plan, ft.terminal(t, "", false))

	require.NoError(t, err)
	assert.Equal(t, []string{"sudo", "apply a", "apply b"}, ft.events)
}

func TestApplyCmd_SudoRefusedAppliesNothing(t *testing.T) {
	ft := fakeTerminal{sudoErr: errors.New("wrong password")}
	plan := run.PlanResult{Steps: []run.Planned{
		ft.planned("a", model.Change{Kind: model.ChangeCreate, NeedsSudo: true}),
	}}

	err := (&ApplyCmd{Yes: true}).applyPlan(plan, ft.terminal(t, "", false))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong password")
	assert.Equal(t, []string{"sudo"}, ft.events)
}

func TestApplyCmd_LogFailureIsReportedAfterTheRunFinishes(t *testing.T) {
	diskFull := errors.New("no space left on device")
	tests := []struct {
		name string
		log  brokenLog
		want error
	}{
		{name: "write fails mid-run", log: brokenLog{writeErr: diskFull}, want: diskFull},
		{name: "close fails", log: brokenLog{closeErr: diskFull}, want: diskFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := fakeTerminal{log: &tt.log}
			change := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1}
			plan := run.PlanResult{Steps: []run.Planned{ft.planned("a", change), ft.planned("b", change)}}

			err := (&ApplyCmd{Yes: true}).applyPlan(plan, ft.terminal(t, "", false))

			require.ErrorIs(t, err, tt.want)
			assert.Contains(t, err.Error(), "broken.log")
			assert.Equal(t, []string{"apply a", "apply b"}, ft.events)
			assert.Contains(t, ft.out.String(), "2 changed · 0 up to date")
			assert.Contains(t, ft.out.String(), "log: broken.log")
			assert.Contains(t, tt.log.written.String(), "2 to change")
		})
	}
}

type outputAction struct{}

func (outputAction) Name() string    { return "fake" }
func (outputAction) NeedsSudo() bool { return false }

func (outputAction) Check(context.Context) (model.Change, error) {
	return model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 1}, nil
}

func (outputAction) Apply(ctx context.Context) error {
	_, err := cmdexec.DefaultRunner().Run(ctx, "sh", "-c", "echo downloading jq; echo installed jq >&2")
	return err
}

func TestApplyCmd_StreamsOutputAndWritesALog(t *testing.T) {
	ft := fakeTerminal{logDir: filepath.Join(t.TempDir(), "state", "booster", "logs")}
	plan, err := run.Plan(context.Background(), run.Compiled{Steps: []run.Step{{Name: "tools", Action: outputAction{}}}})
	require.NoError(t, err)

	require.NoError(t, (&ApplyCmd{Yes: true}).applyPlan(plan, ft.terminal(t, "", false)))

	out := ft.out.String()
	assert.Contains(t, out, "→ tools\n  │ downloading jq\n  │ installed jq\n+ tools  changed")
	logs, err := filepath.Glob(filepath.Join(ft.logDir, "*.log"))
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Contains(t, out, "log: "+logs[0])
	content, err := os.ReadFile(logs[0])
	require.NoError(t, err)
	assert.Contains(t, string(content), "1 to change")
	assert.Contains(t, string(content), "  │ downloading jq\n  │ installed jq\n+ tools  changed")
}

func TestLogDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg/state")
	assert.Equal(t, filepath.Join("/xdg/state", "booster", "logs"), logDir())

	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/someone")
	assert.Equal(t, filepath.Join("/home/someone", ".local", "state", "booster", "logs"), logDir())
}
