package main

import (
	"booster/internal/model"
	"booster/internal/ui"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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

type screenFixture struct {
	cli     *CLI
	dir     string
	runs    string
	started string
	pid     string
	newDir  string
}

func newScreenFixture(t *testing.T, middle string) screenFixture {
	t.Helper()
	dir := t.TempDir()
	f := screenFixture{
		dir:     dir,
		runs:    filepath.Join(dir, "runs"),
		started: filepath.Join(dir, "started"),
		pid:     filepath.Join(dir, "pid"),
		newDir:  filepath.Join(dir, "home", ".config"),
	}
	middle = strings.NewReplacer("STARTED", strconv.Quote(f.started), "PID", strconv.Quote(f.pid)).Replace(middle)
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `version: "1"
tasks:
  - name: count
    action: cmd
    args: { check: [test, -f, %q], run: [sh, -c, 'echo run >> "$1"', sh, %q] }
%s
  - name: dirs
    action: dir.create
    args: [%q]
`, filepath.Join(dir, "never"), f.runs, middle, f.newDir), 0o644))
	f.cli = &CLI{Config: configPath}
	t.Cleanup(f.killSlowStep)
	return f
}

func (f screenFixture) slowStepPID(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile(f.pid)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	require.NoError(t, err)
	return pid
}

func (f screenFixture) killSlowStep() {
	raw, err := os.ReadFile(f.pid)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func (f screenFixture) applyCount(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile(f.runs)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(raw), "run\n")
}

type keyWhenExists struct {
	path string
	key  string
	err  error
	sent bool
}

func (r *keyWhenExists) Read(p []byte) (int, error) {
	if r.sent || r.path == "" {
		return 0, io.EOF
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(r.path); err == nil {
			r.sent = true
			if r.err != nil {
				return 0, r.err
			}
			return copy(p, r.key), nil
		}
	}
	return 0, io.EOF
}

const slowStep = `  - name: slow
    action: cmd
    args: { check: [false], run: [sh, -c, 'echo $$ > "$1"; touch "$2"; exec sleep 30', sh, PID, STARTED] }`

var errTTYGone = errors.New("tty gone")

const (
	showCursor = "\x1b[?25h"
	hideCursor = "\x1b[?25l"
)

func TestApplyCmd_ThroughTheScreens(t *testing.T) {
	tests := []struct {
		name       string
		middle     string
		reviewKeys string
		applyInput func(screenFixture) io.Reader
		wantErr    string
		wantRuns   int
		wantDir    bool
		wantLogs   int
		wantScreen []string
		wantLog    string
		wantOut    string
	}{
		{
			name:       "enter applies once and logs once",
			reviewKeys: "\r",
			wantRuns:   1,
			wantDir:    true,
			wantLogs:   1,
			wantScreen: []string{"applied 2 changes", "2 changed · 0 up to date"},
			wantLog:    "2 changed · 0 up to date",
		},
		{
			name:       "a skipped step is left alone",
			reviewKeys: " \r",
			wantDir:    true,
			wantLogs:   1,
			wantScreen: []string{"applied 1 change", "1 changed · 0 up to date"},
			wantLog:    "1 changed · 0 up to date",
		},
		{
			name:       "a failed step stops the run and fails the command",
			middle:     "  - name: broken\n    action: cmd\n    args: { check: [false], run: [false] }",
			reviewKeys: "\r",
			wantErr:    "apply: 1 of 3 steps failed",
			wantRuns:   1,
			wantLogs:   1,
			wantScreen: []string{"stopped at broken", "1 changed · 0 up to date · 1 failed · 1 not run"},
			wantLog:    "1 changed · 0 up to date · 1 failed · 1 not run",
		},
		{
			name:       "ctrl+c stops the running step and nothing after it runs",
			middle:     slowStep,
			reviewKeys: "\r",
			applyInput: func(f screenFixture) io.Reader { return &keyWhenExists{path: f.started, key: "\x03"} },
			wantErr:    "apply interrupted: context canceled",
			wantRuns:   1,
			wantLogs:   1,
			wantScreen: []string{"stopped at slow", "1 changed · 0 up to date · 1 failed · 1 not run"},
			wantLog:    "1 changed · 0 up to date · 1 failed · 1 not run",
		},
		{
			name:       "a broken terminal stops the run before the log is closed",
			middle:     slowStep,
			reviewKeys: "\r",
			applyInput: func(f screenFixture) io.Reader { return &keyWhenExists{path: f.started, err: errTTYGone} },
			wantErr:    "program was killed: error reading input: tty gone\napply interrupted: context canceled",
			wantRuns:   1,
			wantLogs:   1,
			wantLog:    "1 changed · 0 up to date · 1 failed · 1 not run",
		},
		{
			name:       "quitting the review applies nothing and leaves no log",
			reviewKeys: "q",
			wantOut:    "cancelled, nothing changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newScreenFixture(t, tt.middle)
			var ft fakeTerminal
			var screen bytes.Buffer
			applyIn := io.Reader(&keyWhenExists{})
			if tt.applyInput != nil {
				applyIn = tt.applyInput(f)
			}
			term := withTUI(ft.terminal(t, "", true), strings.NewReader(tt.reviewKeys), applyIn, &screen, ui.Header{})

			done := make(chan error, 1)
			go func() { done <- (&ApplyCmd{}).run(t.Context(), f.cli, term) }()
			var err error
			select {
			case err = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("apply did not return")
			}

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantRuns, f.applyCount(t))
			assert.Equal(t, tt.wantDir, dirExists(f.newDir))
			for _, want := range tt.wantScreen {
				assert.Contains(t, screen.String(), want)
			}
			assert.Greater(t, strings.LastIndex(screen.String(), showCursor), strings.LastIndex(screen.String(), hideCursor), "the terminal is restored")
			assert.Contains(t, ft.out.String(), tt.wantOut)

			logs, err := filepath.Glob(filepath.Join(ft.logDir, "*.log"))
			require.NoError(t, err)
			require.Len(t, logs, tt.wantLogs)
			if tt.wantLogs > 0 {
				content, err := os.ReadFile(logs[0])
				require.NoError(t, err)
				assert.Contains(t, string(content), "to change")
				assert.True(t, strings.HasSuffix(string(content), tt.wantLog+"\n"), "the log is closed after the run's last event")
			}
			if _, err := os.Stat(f.pid); err == nil {
				assert.ErrorIs(t, syscall.Kill(f.slowStepPID(t), 0), syscall.ESRCH, "the slow step's process is gone")
			}
		})
	}
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
