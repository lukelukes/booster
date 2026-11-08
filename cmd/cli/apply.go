package main

import (
	"booster/internal/cmdexec"
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
	in          io.Reader
	out         io.Writer
	openLog     func() (logFile, error)
	interactive bool
	sudo        func() error
	keepSudo    func(context.Context) (stop func())
}

type logFile interface {
	io.WriteCloser
	Name() string
}

func (c *ApplyCmd) Run(cli *CLI) error {
	return c.run(context.Background(), cli, terminal{
		in:          os.Stdin,
		out:         os.Stdout,
		openLog:     func() (logFile, error) { return openLog(logDir()) },
		interactive: isTerminal(os.Stdin),
		sudo:        ensureSudo,
		keepSudo:    keepSudoAlive,
	})
}

func (c *ApplyCmd) run(ctx context.Context, cli *CLI, term terminal) error {
	plan, err := buildPlan(cli, c.Profile)
	if err != nil {
		return err
	}
	return c.applyPlan(ctx, plan, term)
}

func (c *ApplyCmd) applyPlan(ctx context.Context, plan run.PlanResult, term terminal) error {
	view := plan.View()
	if err := ui.RenderPlan(term.out, view); err != nil {
		return err
	}
	if view.HasChanges() {
		if ok, err := c.approve(view, term); err != nil || !ok {
			return err
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

func (c *ApplyCmd) approve(view model.Plan, term terminal) (bool, error) {
	if !c.Yes {
		ok, err := confirm(term)
		if err != nil || !ok {
			return false, err
		}
	}
	if view.NeedsSudo() {
		if err := term.sudo(); err != nil {
			return false, fmt.Errorf("sudo required: %w", err)
		}
	}
	return true, nil
}

func applySteps(ctx context.Context, plan run.PlanResult, view model.Plan, term terminal, log io.Writer) error {
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if view.NeedsSudo() {
		stop := term.keepSudo(ctx)
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

func keepSudoAlive(ctx context.Context) (stop func()) {
	ticker := time.NewTicker(run.SudoRefreshInterval)
	stopRefresh := run.KeepSudoAlive(ctx, cmdexec.DefaultRunner(), ticker.C)
	return func() {
		stopRefresh()
		ticker.Stop()
	}
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

func confirm(term terminal) (bool, error) {
	if !term.interactive {
		return false, errNotConfirmed
	}

	fmt.Fprint(term.out, "\napply these changes? [y/N] ")
	line, err := bufio.NewReader(term.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		fmt.Fprintln(term.out, "cancelled, nothing changed")
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
