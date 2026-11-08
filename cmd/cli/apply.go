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

type reviewFunc func(model.Plan, ui.Mode) (ui.Decision, error)

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
		term.review = func(p model.Plan, mode ui.Mode) (ui.Decision, error) {
			return ui.Review(os.Stdin, os.Stdout, header, p, mode)
		}
	}
	return term
}

func useTUI(stdinTTY, stdoutTTY bool, noColor string) bool {
	return stdinTTY && stdoutTTY && noColor == ""
}

func plainReview(in io.Reader, out io.Writer, interactive bool) reviewFunc {
	return func(p model.Plan, mode ui.Mode) (ui.Decision, error) {
		if err := ui.RenderPlan(out, p); err != nil {
			return ui.Decision{}, err
		}
		if mode == ui.ModeDrift {
			return ui.Decision{}, nil
		}
		ok, err := confirm(in, out, interactive)
		return ui.Decision{Apply: ok}, err
	}
}

func showPlan(out io.Writer, apply bool) reviewFunc {
	return func(p model.Plan, _ ui.Mode) (ui.Decision, error) {
		return ui.Decision{Apply: apply}, ui.RenderPlan(out, p)
	}
}

func reviewHeader(config, profile string) ui.Header {
	ctx := expr.NewContext()
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, config); err == nil && !strings.HasPrefix(rel, "..") {
			config = filepath.Join("~", rel)
		}
	}
	return ui.Header{OS: ctx.OS, Arch: ctx.Arch, Profile: profile, Config: config}
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
	view := plan.View()
	review := term.review
	if c.Yes || !view.HasChanges() {
		review = showPlan(term.out, true)
	}
	decision, err := review(view, ui.ModeApply)
	if err != nil {
		return err
	}
	if !decision.Apply {
		fmt.Fprintln(term.out, "cancelled, nothing changed")
		return nil
	}
	return execute(ctx, plan.Without(decision.Skip), term)
}

func execute(ctx context.Context, plan run.PlanResult, term terminal) error {
	view := plan.View()
	if view.NeedsSudo() {
		if err := term.sudo(); err != nil {
			return fmt.Errorf("sudo required: %w", err)
		}
	}

	file, err := term.openLog()
	if err != nil {
		return err
	}
	log := &runLog{file: file}
	var applyErr error
	if err := ui.RenderPlan(log, view); err == nil && view.HasChanges() {
		applyErr = applySteps(ctx, plan, view, term, log)
	}
	fmt.Fprintf(term.out, "log: %s\n", file.Name())
	return errors.Join(applyErr, log.Close())
}

func applySteps(ctx context.Context, plan run.PlanResult, view model.Plan, term terminal, log io.Writer) error {
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if view.NeedsSudo() {
		stop := run.KeepSudoAlive(ctx, term.sudoRunner, term.sudoTicks)
		defer stop()
	}
	screen, logged := ui.Progress(term.out, view), ui.Progress(log, view)
	summary := run.Apply(ctx, plan, func(e model.Event) {
		screen(e)
		logged(e)
	})
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("apply interrupted: %w", err)
	}
	if summary.Failed > 0 {
		return fmt.Errorf("apply: %d of %d steps failed", summary.Failed, summary.Total)
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
