package main

import (
	"booster/internal/run"
	"booster/internal/ui"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	interactive bool
	sudo        func() error
}

func (c *ApplyCmd) Run(cli *CLI) error {
	return c.run(cli, terminal{in: os.Stdin, out: os.Stdout, interactive: isTerminal(os.Stdin), sudo: ensureSudo})
}

func (c *ApplyCmd) run(cli *CLI, term terminal) error {
	plan, err := buildPlan(cli, c.Profile)
	if err != nil {
		return err
	}
	return c.applyPlan(plan, term)
}

func (c *ApplyCmd) applyPlan(plan run.PlanResult, term terminal) error {
	view := plan.View()
	if err := ui.RenderPlan(term.out, view); err != nil {
		return err
	}
	if !view.HasChanges() {
		return nil
	}

	if !c.Yes {
		ok, err := confirm(term)
		if err != nil || !ok {
			return err
		}
	}

	if view.NeedsSudo() {
		if err := term.sudo(); err != nil {
			return fmt.Errorf("sudo required: %w", err)
		}
	}

	summary := run.Apply(context.Background(), plan, ui.Progress(term.out, view))
	if summary.Failed > 0 {
		return fmt.Errorf("apply: %d of %d steps failed", summary.Failed, summary.Total)
	}
	return nil
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

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd())
}
