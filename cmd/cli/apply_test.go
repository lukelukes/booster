package main

import (
	"booster/internal/action"
	"booster/internal/cmdexec"
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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
	out        strings.Builder
	logDir     string
	events     []string
	sudoErr    error
	log        *brokenLog
	sudoRunner cmdexec.Runner
	sudoTicks  chan time.Time
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
	if ft.sudoRunner == nil {
		ft.sudoRunner = &cmdexec.MockRunner{}
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
		sudoRunner: ft.sudoRunner,
		sudoTicks:  ft.sudoTicks,
	}
}

func (ft *fakeTerminal) planned(name string, change model.Change) run.Planned {
	return run.Planned{
		PlannedStep: model.PlannedStep{Name: name, Change: change},
		Action:      &fakeAction{name: name, events: &ft.events},
	}
}

type fakeAction struct {
	name    string
	events  *[]string
	onApply func() error
}

func (a *fakeAction) Name() string { return a.name }

func (a *fakeAction) Check(context.Context) (model.Change, error) {
	return model.Change{}, nil
}

func (a *fakeAction) Apply(context.Context) error {
	*a.events = append(*a.events, "apply "+a.name)
	if a.onApply != nil {
		return a.onApply()
	}
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

			err := (&ApplyCmd{Yes: tt.yes}).run(t.Context(), f.cli, ft.terminal(t, tt.input, tt.interactive))

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

	err := (&ApplyCmd{Yes: true}).run(t.Context(), f.cli, ft.terminal(t, "", false))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2 steps failed")
	assert.Contains(t, ft.out.String(), "✗ dirs  failed: ")
	assert.Contains(t, ft.out.String(), "permission denied")
	assert.Contains(t, ft.out.String(), "- links  not run")
	assert.NoFileExists(t, f.link)
}

func TestApplyCmd_RefreshesSudoOnlyWhileApplying(t *testing.T) {
	tests := []struct {
		name       string
		finish     func(cancel context.CancelFunc) error
		wantEvents []string
		wantErr    string
		wantOut    string
	}{
		{
			name:       "completed",
			wantEvents: []string{"sudo", "apply a", "apply b"},
			wantOut:    "2 changed · 0 up to date",
		},
		{
			name:       "failed",
			finish:     func(context.CancelFunc) error { return errors.New("exit status 1") },
			wantEvents: []string{"sudo", "apply a"},
			wantErr:    "1 of 2 steps failed",
			wantOut:    "- b  not run",
		},
		{
			name:       "interrupted",
			finish:     func(cancel context.CancelFunc) error { cancel(); return nil },
			wantEvents: []string{"sudo", "apply a"},
			wantErr:    "apply interrupted: context canceled",
			wantOut:    "- b  not run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refreshes := make(chan []string, 10)
			ft := fakeTerminal{
				sudoTicks: make(chan time.Time, 1),
				sudoRunner: &cmdexec.MockRunner{RunFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
					refreshes <- append([]string{name}, args...)
					return nil, nil
				}},
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			needsSudo := model.Change{Kind: model.ChangeCreate, Verb: "install", NeedsSudo: true}
			plan := run.PlanResult{Steps: []run.Planned{ft.planned("a", needsSudo), ft.planned("b", needsSudo)}}
			plan.Steps[0].Action.(*fakeAction).onApply = func() error {
				ft.sudoTicks <- time.Time{}
				select {
				case call := <-refreshes:
					assert.Equal(t, []string{"sudo", "-n", "-v"}, call)
				case <-time.After(5 * time.Second):
					t.Error("sudo was not refreshed while applying")
				}
				if tt.finish != nil {
					return tt.finish(cancel)
				}
				return nil
			}

			err := (&ApplyCmd{Yes: true}).applyPlan(ctx, plan, ft.terminal(t, "", false))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantEvents, ft.events)
			assert.Contains(t, ft.out.String(), tt.wantOut)
			select {
			case ft.sudoTicks <- time.Time{}:
			default:
			}
			assert.Never(t, func() bool { return len(refreshes) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "sudo was refreshed after apply finished")
			assert.Len(t, ft.sudoTicks, 1)
		})
	}
}

func TestApplyCmd_SudoRunsInBoostersSessionInItsOwnGroup(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	bin := t.TempDir()
	for _, name := range []string{"sudo", "pacman"} {
		require.NoError(t, os.Symlink(exe, filepath.Join(bin, name)))
	}
	records := filepath.Join(t.TempDir(), "sudo.log")
	t.Setenv("PATH", bin)
	t.Setenv(fakeSudoLogEnv, records)
	t.Setenv("GORACE", "atexit_sleep_ms=0")

	var ft fakeTerminal
	ticks := make(chan time.Time, 1)
	term := ft.terminal(t, "", false)
	term.sudo = ensureSudo
	term.sudoRunner = cmdexec.DefaultRunner()
	term.sudoTicks = ticks
	plan := run.PlanResult{Steps: []run.Planned{
		ft.planned("pkgs", model.Change{Kind: model.ChangeCreate, Verb: "install", NeedsSudo: true}),
	}}
	plan.Steps[0].Action.(*fakeAction).onApply = func() error {
		ticks <- time.Time{}
		assert.Eventually(t, func() bool {
			return len(readSudoRecords(t, records)) == 2
		}, 5*time.Second, 10*time.Millisecond, "sudo was not refreshed while applying")
		_, err := action.NewPacmanManager(cmdexec.DefaultRunner()).Install(t.Context(), []string{"fd"})
		return err
	}

	require.NoError(t, (&ApplyCmd{Yes: true}).applyPlan(t.Context(), plan, term))

	got := readSudoRecords(t, records)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"-n true", "-n -v", "-n pacman -S --noconfirm --needed fd"},
		[]string{got[0].args, got[1].args, got[2].args})
	session, err := unix.Getsid(0)
	require.NoError(t, err)
	for _, r := range got {
		assert.Equal(t, session, r.sid, "sudo %s ran outside booster's session", r.args)
	}
	for _, r := range got[1:] {
		assert.Equal(t, r.pid, r.pgid, "sudo %s does not lead its own process group", r.args)
		assert.NotEqual(t, unix.Getpgrp(), r.pgid, "sudo %s shares booster's process group", r.args)
	}
}

const fakeSudoLogEnv = "BOOSTER_TEST_FAKE_SUDO_LOG"

func TestMain(m *testing.M) {
	if records := os.Getenv(fakeSudoLogEnv); records != "" {
		os.Exit(fakeSudo(records))
	}
	os.Exit(m.Run())
}

func fakeSudo(records string) int {
	if filepath.Base(os.Args[0]) != "sudo" {
		return 0
	}
	sid, err := unix.Getsid(0)
	if err != nil {
		return 1
	}
	f, err := os.OpenFile(records, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 1
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%d %d %d %s\n", sid, unix.Getpgrp(), os.Getpid(), strings.Join(os.Args[1:], " ")); err != nil {
		return 1
	}
	return 0
}

type sudoRecord struct {
	sid, pgid, pid int
	args           string
}

func readSudoRecords(t *testing.T, path string) []sudoRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	var records []sudoRecord
	for line := range strings.Lines(string(data)) {
		var r sudoRecord
		fields := strings.SplitN(strings.TrimSpace(line), " ", 4)
		require.Len(t, fields, 4)
		for i, n := range []*int{&r.sid, &r.pgid, &r.pid} {
			*n, err = strconv.Atoi(fields[i])
			require.NoError(t, err)
		}
		r.args = fields[3]
		records = append(records, r)
	}
	return records
}

func TestApplyCmd_SudoRefusedAppliesNothing(t *testing.T) {
	ft := fakeTerminal{sudoErr: errors.New("wrong password")}
	plan := run.PlanResult{Steps: []run.Planned{
		ft.planned("a", model.Change{Kind: model.ChangeCreate, NeedsSudo: true}),
	}}

	err := (&ApplyCmd{Yes: true}).applyPlan(t.Context(), plan, ft.terminal(t, "", false))

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

			err := (&ApplyCmd{Yes: true}).applyPlan(t.Context(), plan, ft.terminal(t, "", false))

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

	require.NoError(t, (&ApplyCmd{Yes: true}).applyPlan(t.Context(), plan, ft.terminal(t, "", false)))

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
