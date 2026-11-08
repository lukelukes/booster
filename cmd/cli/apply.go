package main

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/run"
	"booster/internal/ui"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
)

var errNotConfirmed = errors.New("refusing to apply without confirmation: stdin is not a terminal, pass --yes to apply anyway")

type ApplyCmd struct {
	Profile string `help:"Profile to use (required when profiles defined in config)"`
	Yes     bool   `short:"y" help:"Apply without asking for confirmation"`
}

type terminal struct {
	out        io.Writer
	openLog    func() (logFile, error)
	sudo       func() error
	sudoRunner cmdexec.Runner
	sudoTicks  <-chan time.Time
	review     reviewFunc
}

type reviewFunc func(context.Context, model.Plan, ui.Mode, ui.Start) (ui.Decision, error)

type logFile interface {
	io.WriteCloser
	Name() string
}

func newTerminal(cli *CLI, profile string, sudoTicks <-chan time.Time) terminal {
	stdinTTY := isTerminal(os.Stdin)
	term := terminal{
		out:        os.Stdout,
		openLog:    func() (logFile, error) { return openLog(logDir()) },
		sudo:       ensureSudo,
		sudoRunner: cmdexec.DefaultRunner(),
		sudoTicks:  sudoTicks,
		review:     plainReview(os.Stdin, os.Stdout, stdinTTY),
	}
	if useTUI(stdinTTY, isTerminal(os.Stdout), os.Getenv("NO_COLOR")) {
		header := reviewHeader(cli.Config, profile)
		term.review = func(ctx context.Context, p model.Plan, mode ui.Mode, start ui.Start) (ui.Decision, error) {
			return ui.Review(ctx, os.Stdin, os.Stdout, header, p, mode, func(d ui.Decision) (ui.Job, error) {
				job, err := start(d)
				job.Log = tilde(job.Log)
				return job, err
			})
		}
	}
	return term
}

func useTUI(stdinTTY, stdoutTTY bool, noColor string) bool {
	return stdinTTY && stdoutTTY && noColor == ""
}

func plainReview(in io.Reader, out io.Writer, interactive bool) reviewFunc {
	return func(ctx context.Context, p model.Plan, mode ui.Mode, start ui.Start) (ui.Decision, error) {
		if err := ui.RenderPlan(out, p); err != nil {
			return ui.Decision{}, err
		}
		if mode == ui.ModeDrift {
			return ui.Decision{}, nil
		}
		ok, err := confirm(in, out, interactive)
		if err != nil {
			return ui.Decision{}, err
		}
		d := ui.Decision{Apply: ok}
		return d, runJob(ctx, out, d, start)
	}
}

func showPlan(out io.Writer, apply bool) reviewFunc {
	return func(ctx context.Context, p model.Plan, _ ui.Mode, start ui.Start) (ui.Decision, error) {
		if err := ui.RenderPlan(out, p); err != nil {
			return ui.Decision{}, err
		}
		d := ui.Decision{Apply: apply}
		return d, runJob(ctx, out, d, start)
	}
}

func runJob(ctx context.Context, out io.Writer, d ui.Decision, start ui.Start) error {
	if !d.Apply {
		return nil
	}
	job, err := start(d)
	if err != nil {
		return err
	}
	if job.Sudo != nil {
		if err := job.Sudo(); err != nil {
			return err
		}
	}
	job.Run(ctx, ui.Progress(out, job.Plan))
	fmt.Fprintf(out, "log: %s\n", job.Log)
	return nil
}

func reviewHeader(config, profile string) ui.Header {
	ctx := expr.NewContext()
	return ui.Header{OS: ctx.OS, Arch: ctx.Arch, Profile: profile, Config: tilde(config)}
}

func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}

func (c *ApplyCmd) Run(cli *CLI) error {
	ticker := time.NewTicker(run.SudoRefreshInterval)
	defer ticker.Stop()
	return c.run(context.Background(), cli, newTerminal(cli, c.Profile, ticker.C))
}

func (c *ApplyCmd) run(ctx context.Context, cli *CLI, term terminal) error {
	plan, err := buildPlan(ctx, cli, c.Profile)
	if err != nil {
		return err
	}
	return c.applyPlan(ctx, plan, term)
}

func (c *ApplyCmd) applyPlan(ctx context.Context, plan run.PlanResult, term terminal) error {
	review := term.review
	if c.Yes || !plan.View().HasChanges() {
		review = showPlan(term.out, true)
	}
	applied, err := reviewAndApply(ctx, plan, term, review, ui.ModeApply)
	if err != nil {
		return err
	}
	if !applied {
		fmt.Fprintln(term.out, "cancelled, nothing changed")
	}
	return nil
}

func reviewAndApply(ctx context.Context, plan run.PlanResult, term terminal, review reviewFunc, mode ui.Mode) (bool, error) {
	var (
		log    *runLog
		ran    bool
		runErr error
	)
	start := func(d ui.Decision) (ui.Job, error) {
		steps := plan.Without(d.Skip)
		view := steps.View()
		file, err := term.openLog()
		if err != nil {
			return ui.Job{}, err
		}
		log = &runLog{file: file}
		if err := ui.RenderPlan(log, view); err != nil {
			return ui.Job{}, err
		}
		job := ui.Job{Plan: view, Log: file.Name(), Run: func(ctx context.Context, emit func(model.Event)) {
			if view.HasChanges() {
				runErr = applySteps(ctx, steps, view, term, log, emit)
			}
			ran = true
		}}
		if view.NeedsSudo() {
			job.Sudo = func() error { return sudo(term) }
		}
		return job, nil
	}

	_, err := review(ctx, plan.View(), mode, start)
	if log == nil {
		return false, err
	}
	closeErr := log.Close()
	if !ran {
		os.Remove(log.file.Name())
		return false, err
	}
	return true, errors.Join(err, runErr, closeErr)
}

func applySteps(ctx context.Context, plan run.PlanResult, view model.Plan, term terminal, log io.Writer, emit func(model.Event)) error {
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if view.NeedsSudo() {
		stop := run.KeepSudoAlive(ctx, term.sudoRunner, term.sudoTicks)
		defer stop()
	}
	logged := ui.Progress(log, view)
	summary := run.Apply(ctx, plan, func(e model.Event) {
		logged(e)
		emit(e)
	})
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("apply interrupted: %w", err)
	}
	return failure(summary)
}

func sudo(term terminal) error {
	if err := term.sudo(); err != nil {
		return fmt.Errorf("sudo required: %w", err)
	}
	return nil
}

func failure(s model.Summary) error {
	if s.Failed > 0 {
		return fmt.Errorf("apply: %d of %d steps failed", s.Failed, s.Total)
	}
	return nil
}

type runLog struct {
	file logFile
	err  error
}

func (l *runLog) Write(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	n, err := l.file.Write(p)
	if err != nil {
		l.err = fmt.Errorf("write log %s: %w", l.file.Name(), err)
	}
	return n, l.err
}

func (l *runLog) Close() error {
	if err := l.file.Close(); err != nil && l.err == nil {
		l.err = fmt.Errorf("close log %s: %w", l.file.Name(), err)
	}
	return l.err
}

func confirm(in io.Reader, out io.Writer, interactive bool) (bool, error) {
	if !interactive {
		return false, errNotConfirmed
	}

	fmt.Fprint(out, "\napply these changes? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func logDir() string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "booster", "logs")
}

func openLog(dir string) (logFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.CreateTemp(dir, "apply-"+time.Now().Format("20060102-150405")+"-*.log")
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	return f, nil
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd())
}
