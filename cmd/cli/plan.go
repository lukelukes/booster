package main

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/run"
	"booster/internal/ui"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var errChangesPending = errors.New("changes pending")

type PlanCmd struct {
	Profile string `help:"Profile to use (required when profiles defined in config)"`
}

func (c *PlanCmd) Run(cli *CLI) error {
	return c.run(cli, os.Stdout)
}

func (c *PlanCmd) run(cli *CLI, out io.Writer) error {
	plan, err := buildPlan(context.Background(), cli, c.Profile)
	if err != nil {
		return err
	}

	view := plan.View()
	if err := ui.RenderPlan(out, view); err != nil {
		return err
	}

	if view.HasChanges() {
		return errChangesPending
	}
	return nil
}

func buildPlan(ctx context.Context, cli *CLI, profileFlag string) (run.PlanResult, error) {
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	plan, err := planSteps(ctx, cli, profileFlag)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return run.PlanResult{}, fmt.Errorf("plan interrupted: %w", ctxErr)
	}
	return plan, err
}

func planSteps(ctx context.Context, cli *CLI, profileFlag string) (run.PlanResult, error) {
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("load config: %w", err)
	}

	profile, err := validateProfile(cfg.Profiles, profileFlag)
	if err != nil {
		return run.PlanResult{}, err
	}

	vars, err := resolveVariables(cfg.Variables)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("resolve variables: %w", err)
	}

	varsAny := make(map[string]any, len(vars))
	for k, v := range vars {
		varsAny[k] = v
	}
	exprCtx := expr.NewContext().WithProfile(profile).WithVars(varsAny).WithConfigDir(filepath.Dir(cli.Config))

	compiled, err := run.Compile(cfg.Tasks, exprCtx, action.Registry())
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("compile: %w", err)
	}

	plan, err := run.Plan(ctx, compiled)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("plan: %w", err)
	}

	return plan, nil
}
